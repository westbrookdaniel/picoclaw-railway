package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var secretFields = map[string]struct{}{
	"api_key":              {},
	"token":                {},
	"app_secret":           {},
	"encrypt_key":          {},
	"verification_token":   {},
	"bot_token":            {},
	"app_token":            {},
	"channel_secret":       {},
	"channel_access_token": {},
	"client_secret":        {},
}

type gatewayStatus struct {
	State        string `json:"state"`
	PID          int    `json:"pid,omitempty"`
	Uptime       int64  `json:"uptime,omitempty"`
	RestartCount int    `json:"restart_count"`
}

type logBuffer struct {
	mu    sync.Mutex
	max   int
	lines []string
}

func newLogBuffer(max int) *logBuffer {
	return &logBuffer{max: max, lines: make([]string, 0, max)}
}

func (b *logBuffer) append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.lines) == b.max {
		copy(b.lines, b.lines[1:])
		b.lines = b.lines[:b.max-1]
	}
	b.lines = append(b.lines, line)
}

func (b *logBuffer) snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

type gatewayManager struct {
	mu           sync.Mutex
	cmd          *exec.Cmd
	state        string
	startTime    time.Time
	restartCount int
	logs         *logBuffer
}

func newGatewayManager() *gatewayManager {
	return &gatewayManager{
		state: "stopped",
		logs:  newLogBuffer(500),
	}
}

func (g *gatewayManager) start() {
	g.mu.Lock()
	if g.cmd != nil && g.cmd.Process != nil {
		g.mu.Unlock()
		return
	}
	g.state = "starting"
	cmd := exec.Command("picoclaw", "gateway")
	cmd.Env = os.Environ()
	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter
	if err := cmd.Start(); err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		g.state = "error"
		g.logs.append("Failed to start gateway: " + err.Error())
		g.mu.Unlock()
		return
	}
	g.cmd = cmd
	g.state = "running"
	g.startTime = time.Now()
	g.mu.Unlock()

	go g.readOutput(pipeReader)
	go g.waitForExit(cmd, pipeWriter)
}

func (g *gatewayManager) stop() {
	g.mu.Lock()
	cmd := g.cmd
	if cmd == nil || cmd.Process == nil {
		g.state = "stopped"
		g.mu.Unlock()
		return
	}
	g.state = "stopping"
	g.mu.Unlock()

	_ = cmd.Process.Signal(syscall.SIGTERM)
	for range 100 {
		time.Sleep(100 * time.Millisecond)
		g.mu.Lock()
		stillRunning := g.cmd == cmd
		g.mu.Unlock()
		if !stillRunning {
			return
		}
	}
	_ = cmd.Process.Kill()
}

func (g *gatewayManager) restart() {
	g.mu.Lock()
	g.restartCount++
	g.mu.Unlock()
	g.stop()
	g.start()
}

func (g *gatewayManager) readOutput(r io.ReadCloser) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		g.logs.append(ansiEscape.ReplaceAllString(line, ""))
	}
	if err := scanner.Err(); err != nil {
		g.logs.append("Gateway log stream error: " + err.Error())
	}
}

func (g *gatewayManager) waitForExit(cmd *exec.Cmd, pipeWriter *io.PipeWriter) {
	err := cmd.Wait()
	_ = pipeWriter.Close()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cmd != cmd {
		return
	}
	g.cmd = nil
	g.startTime = time.Time{}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			g.state = "error"
			g.logs.append(fmt.Sprintf("Gateway exited with code %d", exitErr.ExitCode()))
			return
		}
		g.state = "error"
		g.logs.append("Gateway exited: " + err.Error())
		return
	}
	if g.state != "stopping" {
		g.state = "stopped"
		return
	}
	g.state = "stopped"
}

func (g *gatewayManager) status() gatewayStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	status := gatewayStatus{State: g.state, RestartCount: g.restartCount}
	if g.cmd != nil && g.cmd.Process != nil {
		status.PID = g.cmd.Process.Pid
	}
	if !g.startTime.IsZero() && g.state == "running" {
		status.Uptime = int64(time.Since(g.startTime).Seconds())
	}
	return status
}

type app struct {
	adminUsername string
	adminPassword string
	configDir     string
	configPath    string
	templateHTML  []byte
	gateway       *gatewayManager
	configMu      sync.Mutex
}

func main() {
	configDir := getenv("PICOCLAW_HOME", filepath.Join(getenv("HOME", "/data"), ".picoclaw"))
	configPath := filepath.Join(configDir, "config.json")
	adminUsername := getenv("ADMIN_USERNAME", "admin")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = randomPassword()
		log.Printf("Generated admin password: %s", adminPassword)
	}
	templateHTML, err := os.ReadFile("/app/templates/index.html")
	if err != nil {
		log.Fatalf("read template: %v", err)
	}

	a := &app{
		adminUsername: adminUsername,
		adminPassword: adminPassword,
		configDir:     configDir,
		configPath:    configPath,
		templateHTML:  templateHTML,
		gateway:       newGatewayManager(),
	}
	a.autoStartGateway()

	mux := http.NewServeMux()
	mux.HandleFunc("/", a.homepage)
	mux.HandleFunc("/health", a.health)
	mux.HandleFunc("/api/config", a.configHandler)
	mux.HandleFunc("/api/status", a.requireAuth(a.status))
	mux.HandleFunc("/api/logs", a.requireAuth(a.logs))
	mux.HandleFunc("/api/gateway/start", a.requireAuth(a.gatewayStart))
	mux.HandleFunc("/api/gateway/stop", a.requireAuth(a.gatewayStop))
	mux.HandleFunc("/api/gateway/restart", a.requireAuth(a.gatewayRestart))

	server := &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		a.gateway.stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("Listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (a *app) homepage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !a.checkAuth(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(a.templateHTML)
}

func (a *app) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"gateway": a.gateway.status().State,
	})
}

func (a *app) configHandler(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		cfg := a.loadConfig()
		writeJSON(w, http.StatusOK, maskSecrets(cfg))
	case http.MethodPut:
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		restart, _ := body["_restartGateway"].(bool)
		delete(body, "_restartGateway")
		a.configMu.Lock()
		existing := a.loadConfig()
		merged, ok := mergeSecrets(body, existing).(map[string]any)
		if !ok {
			a.configMu.Unlock()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid config payload"})
			return
		}
		if err := a.saveConfig(merged); err != nil {
			a.configMu.Unlock()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		a.configMu.Unlock()
		if restart {
			go a.gateway.restart()
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restarting": restart})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func (a *app) status(w http.ResponseWriter, _ *http.Request) {
	config := a.loadConfig()
	cronJobs := a.readCronJobs()
	providers := map[string]any{}
	if rawProviders, ok := config["providers"].(map[string]any); ok {
		for name, raw := range rawProviders {
			configured := false
			if provider, ok := raw.(map[string]any); ok {
				configured = asString(provider["api_key"]) != ""
			}
			providers[name] = map[string]bool{"configured": configured}
		}
	}
	channels := map[string]any{}
	if rawChannels, ok := config["channels"].(map[string]any); ok {
		for name, raw := range rawChannels {
			enabled := false
			if channel, ok := raw.(map[string]any); ok {
				enabled = asBool(channel["enabled"])
			}
			channels[name] = map[string]bool{"enabled": enabled}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"gateway":   a.gateway.status(),
		"providers": providers,
		"channels":  channels,
		"cron":      map[string]any{"count": len(cronJobs), "jobs": cronJobs},
	})
}

func (a *app) logs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"lines": a.gateway.logs.snapshot()})
}

func (a *app) gatewayStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	go a.gateway.start()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) gatewayStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	go a.gateway.stop()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) gatewayRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	go a.gateway.restart()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) autoStartGateway() {
	config := a.loadConfig()
	if rawProviders, ok := config["providers"].(map[string]any); ok {
		for _, raw := range rawProviders {
			if provider, ok := raw.(map[string]any); ok && asString(provider["api_key"]) != "" {
				go a.gateway.start()
				return
			}
		}
	}
}

func (a *app) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.checkAuth(w, r) {
			return
		}
		next(w, r)
	}
}

func (a *app) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if !ok || subtle.ConstantTimeCompare([]byte(username), []byte(a.adminUsername)) != 1 || subtle.ConstantTimeCompare([]byte(password), []byte(a.adminPassword)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="picoclaw"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (a *app) loadConfig() map[string]any {
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return defaultConfig()
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return defaultConfig()
	}
	return config
}

func (a *app) saveConfig(config map[string]any) error {
	if err := os.MkdirAll(a.configDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, append(data, '\n'), 0o644)
}

func (a *app) readCronJobs() []map[string]any {
	cronDir := filepath.Join(a.configDir, "cron")
	entries, err := os.ReadDir(cronDir)
	if err != nil {
		return nil
	}
	jobs := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cronDir, entry.Name()))
		if err != nil {
			continue
		}
		var job map[string]any
		if err := json.Unmarshal(data, &job); err == nil {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func defaultConfig() map[string]any {
	var config map[string]any
	if err := json.Unmarshal([]byte(defaultConfigJSON), &config); err != nil {
		panic(err)
	}
	return config
}

func maskSecrets(value any) any {
	switch v := value.(type) {
	case map[string]any:
		masked := make(map[string]any, len(v))
		for key, item := range v {
			if _, ok := secretFields[key]; ok {
				secret := asString(item)
				switch {
				case secret == "":
					masked[key] = ""
				case len(secret) > 8:
					masked[key] = secret[:8] + "***"
				default:
					masked[key] = "***"
				}
				continue
			}
			masked[key] = maskSecrets(item)
		}
		return masked
	case []any:
		masked := make([]any, len(v))
		for i := range v {
			masked[i] = maskSecrets(v[i])
		}
		return masked
	default:
		return value
	}
}

func mergeSecrets(newValue, existingValue any) any {
	newMap, okNew := newValue.(map[string]any)
	existingMap, okExisting := existingValue.(map[string]any)
	if okNew && okExisting {
		merged := make(map[string]any, len(newMap))
		for key, item := range newMap {
			if _, ok := secretFields[key]; ok {
				secret := asString(item)
				if secret == "" || strings.HasSuffix(secret, "***") {
					merged[key] = existingMap[key]
					continue
				}
			}
			merged[key] = mergeSecrets(item, existingMap[key])
		}
		return merged
	}
	newList, okNew := newValue.([]any)
	existingList, okExisting := existingValue.([]any)
	if okNew && okExisting {
		merged := make([]any, len(newList))
		for i := range newList {
			var existing any
			if i < len(existingList) {
				existing = existingList[i]
			}
			merged[i] = mergeSecrets(newList[i], existing)
		}
		return merged
	}
	return newValue
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func randomPassword() string {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d%x", time.Now().Unix(), os.Getpid())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asBool(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

const defaultConfigJSON = `{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "restrict_to_workspace": true,
      "provider": "",
      "model": "glm-4.7",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "channels": {
    "telegram": {"enabled": false, "token": "", "proxy": "", "allow_from": []},
    "discord": {"enabled": false, "token": "", "allow_from": []},
    "slack": {"enabled": false, "bot_token": "", "app_token": "", "allow_from": []},
    "whatsapp": {"enabled": false, "bridge_url": "ws://localhost:3001", "allow_from": []},
    "feishu": {"enabled": false, "app_id": "", "app_secret": "", "encrypt_key": "", "verification_token": "", "allow_from": []},
    "dingtalk": {"enabled": false, "client_id": "", "client_secret": "", "allow_from": []},
    "qq": {"enabled": false, "app_id": "", "app_secret": "", "allow_from": []},
    "line": {"enabled": false, "channel_secret": "", "channel_access_token": "", "webhook_host": "0.0.0.0", "webhook_port": 18791, "webhook_path": "/webhook/line", "allow_from": []},
    "maixcam": {"enabled": false, "host": "0.0.0.0", "port": 18790, "allow_from": []}
  },
  "providers": {
    "anthropic": {"api_key": ""},
    "openai": {"api_key": "", "api_base": ""},
    "openrouter": {"api_key": ""},
    "deepseek": {"api_key": ""},
    "groq": {"api_key": ""},
    "gemini": {"api_key": ""},
    "zhipu": {"api_key": "", "api_base": ""},
    "vllm": {"api_key": "", "api_base": ""},
    "nvidia": {"api_key": "", "api_base": ""},
    "moonshot": {"api_key": ""}
  },
  "gateway": {"host": "0.0.0.0", "port": 18790},
  "tools": {
    "web": {
      "brave": {"enabled": false, "api_key": "", "max_results": 5},
      "duckduckgo": {"enabled": true, "max_results": 5}
    }
  },
  "heartbeat": {"enabled": true, "interval": 30},
  "devices": {"enabled": false, "monitor_usb": false}
}`

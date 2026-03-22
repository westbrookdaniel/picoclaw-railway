FROM golang:1.25.8-alpine AS picoclaw-builder

RUN apk add --no-cache git make

WORKDIR /src

ARG PICOCLAW_VERSION=main

RUN git clone --depth 1 --branch ${PICOCLAW_VERSION} https://github.com/sipeed/picoclaw.git .
RUN go mod download
RUN make build

FROM golang:1.25.8-alpine AS app-builder

WORKDIR /app

COPY go.mod /app/go.mod
COPY main.go /app/main.go
RUN CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/server /app/main.go

FROM alpine:3.22

RUN apk add --no-cache bash ca-certificates curl git

COPY --from=picoclaw-builder /src/build/picoclaw /usr/local/bin/picoclaw
COPY --from=app-builder /out/server /app/server

RUN mkdir -p /data/.picoclaw

COPY templates/ /app/templates/
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

ENV HOME=/data
ENV PICOCLAW_AGENTS_DEFAULTS_WORKSPACE=/data/.picoclaw/workspace

CMD ["/app/start.sh"]

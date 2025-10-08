# LiveKit MCU Agent

A minimal Go agent that joins a LiveKit room, subscribes to selected tracks, mixes audio/video, and publishes a composite track back to the room.

## Features

- Connects to LiveKit using API key/secret.
- Selects and mixes tracks via `internal/pipeline` and `internal/media`.
- Exposes health and metrics endpoints.

## Quick Start

```bash
cp .env.example .env
make build
./bin/mcu-agent --room ${LIVEKIT_ROOM} --identity mcu-bot
```

Endpoints: `GET /healthz` → 200, `GET /metrics` → stub metrics.

## Configuration

Environment variables (see `.env.example`):

```
LIVEKIT_URL=ws://localhost:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=secret
LIVEKIT_ROOM=demo
MCU_REGION=us-east
MCU_LOG_LEVEL=info
```

Flags:

- `--room` Room name to join (overrides `LIVEKIT_ROOM`).
- `--identity` Participant identity for the agent.
- `--include-kind` Comma-separated kinds to include (empty = all). Options: `audio`, `video`, `screen`.
- `--exclude-kind` Comma-separated kinds to exclude. Options: `audio`, `video`, `screen`.

## Development

- Build: `make build` → outputs `bin/mcu-agent`.
- Run: `make run` or use the `run` script.
- Test: `make test` (uses Go `testing`).
- Format: `make fmt` (gofmt), Lint: `make lint` (golangci-lint).

Code layout:

- Entry point: `cmd/mcu-agent/main.go`.
- Core packages: `internal/pipeline`, `internal/media`, `internal/logging`, `internal/config`, `internal/metrics`.
- Tests colocated as `*_test.go`.

## Docker

Build and run using Docker (example):

```bash
docker build -t lkmcu .
docker run --env-file .env lkmcu \
  --room ${LIVEKIT_ROOM} --identity mcu-bot
```

Compose: see `docker-compose.yml` for local orchestration.

## Contributing

See `AGENTS.md` for repository guidelines (structure, style, testing, and PR process).

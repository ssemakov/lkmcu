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

## Layout Examples

These examples illustrate how to define stage layouts for the MCU.

### Example 1

One participant on stage with audio and video tracks published. No slot specified — video takes the entire stage.

JSON:

`'{"slots": [{"identity": "abc", "trackKind": "video"}, {"identity": "abc", "trackKind": "audio"}]}'`

### Example 2

Two participants on stage. Alice is the main video filling most of the stage; Bob is a picture-in-picture tile on top. Both have audio included.

JSON:

```
  {"slots": [
    {"identity": "alice", "trackKind": "video", "rect": {"x":0, "y":0, "w":0.7, "h":1.0, "z":0}},
    {"identity": "bob", "trackKind": "video", "rect": {"x":0.7, "y":0, "w":0.3, "h":0.3, "z":1}},
    {"identity": "alice", "trackKind": "audio"},
    {"identity": "bob", "trackKind": "audio"}
    ]}
```

## Layout API (curl)

Assuming the layout HTTP handler is mounted at `http://localhost:8080/layout`.

GET current layout

```bash
curl -sS -X GET http://localhost:8080/layout \
  -H "Accept: application/json"
```

POST new layout

- Sends a Stage JSON with non-empty `slots`; success returns `204 No Content`.
- Invalid JSON or empty `slots` returns `400` with body `invalid layout`.

Example 1: single participant full-screen video plus audio

```bash
curl -sS -X POST http://localhost:8080/layout \
  -H "Content-Type: application/json" \
  -d '{
    "slots": [
      {"identity": "abc", "trackKind": "video"},
      {"identity": "abc", "trackKind": "audio"}
    ]
  }' -i
```

Example 2: two videos with explicit rectangles and z-order

```bash
curl -sS -X POST http://localhost:8080/layout \
  -H "Content-Type: application/json" \
  -d '{
    "slots": [
      {
        "identity": "alice",
        "trackKind": "video",
        "rect": {"x":0, "y":0, "w":0.7, "h":1.0, "z":0}
      },
      {
        "identity": "bob",
        "trackKind": "video",
        "rect": {"x":0.7, "y":0, "w":0.3, "h":0.3, "z":1}
      },
      {
        "identity": "alice",
        "trackKind": "audio"
      },
      {
        "identity": "bob",
        "trackKind": "audio"
      }
    ]
  }' -i
```

Notes

- `slots[].identity`: participant identity
- `slots[].trackKind`: `audio`, `video`, or `screen`
- `slots[].trackId`: optional explicit track SID (alternative to identity/trackKind)
- `slots[].rect`: optional `{x,y,w,h,z}` for video/screen; ignored for audio

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

# LiveKit MCU (Reference Skeleton)

Minimal Go repo skeleton for an MCU agent that joins a LiveKit room, subscribes to selected tracks, mixes them, and publishes a composite back.

**Boot:**  
```bash
cp .env.example .env
make build
./bin/mcu-agent --room ${LIVEKIT_ROOM} --identity mcu-bot
```
Health: `GET /healthz` → 200. Metrics: `GET /metrics` (stub).

## Env
```
LIVEKIT_URL=ws://localhost:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=secret
LIVEKIT_ROOM=demo
MCU_REGION=us-east
MCU_LOG_LEVEL=info
```
# lkmcu

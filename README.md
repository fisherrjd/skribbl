# Transcribe Tonic → Microsoft Teams Webhook

A single Go binary that ingests meeting data from **Transcribe Tonic** and posts rich Adaptive Card notifications to a **Microsoft Teams** channel.

```
Transcribe Tonic (cloud)
        │  POST /webhook/transcribe-tonic
        │  Header: X-Tonic-Signature: sha256=<hmac-sha256>
        ▼
┌──────────────────────────────────────┐
│  main.go  (net/http + graceful stop) │   localhost:5050
│  • HMAC-SHA256 verification          │
│  • JSON parsing & structured logging │
│  • Adaptive Card builder             │
└──────────────┬───────────────────────┘
               │  POST Adaptive Card
               ▼
       Microsoft Teams Channel
```

---

## Quick start

### 1 — Configure secrets

```bash
cp .env.example .env
# Edit .env:
#   TRANSCRIBE_TONIC_WEBHOOK_SECRET  — Transcribe Tonic → Settings → Webhooks
#   TEAMS_WEBHOOK_URL                — Teams channel connector URL
```

### 2 — Install as a local launchd service

```bash
make install
```

This will:
- Build the binary
- Render and install `~/Library/LaunchAgents/com.sinch.meetings.plist`
- Load it into launchd — **starts now and auto-starts on every login**
- Restart automatically if it ever crashes

### Service management

```bash
make stop      # stop the service
make start     # start it again
make restart   # stop + start
make status    # check if it's running (PID)
make logs      # tail -f the log file
make uninstall # stop + remove the plist entirely
```

Logs live at `~/Library/Logs/sinch-meetings/meetings.log`.

### One-off / dev run (no service)

```bash
go run .
```

---

## Expose locally with ngrok

Transcribe Tonic needs a public HTTPS URL to deliver webhooks.

```bash
ngrok http 5050
# Copy the https://xxxxx.ngrok-free.app URL
```

---

## Register the webhook in Transcribe Tonic

1. **Settings → Webhooks → Add Webhook**
2. **URL**: `https://<ngrok-url>/webhook/transcribe-tonic`
3. **Events**: `meeting.completed`, `transcript.ready`, etc.
4. **Secret**: same value as `TRANSCRIBE_TONIC_WEBHOOK_SECRET` in `.env`

---

## Register Incoming Webhook in Teams

1. Open the target Teams channel
2. **Manage channel → Connectors → Incoming Webhook → Configure**
3. Copy the generated URL → paste into `TEAMS_WEBHOOK_URL` in `.env`

---

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET`  | `/health` | None | Liveness probe |
| `POST` | `/webhook/transcribe-tonic` | HMAC-SHA256 | Primary receiver |
| `POST` | `/webhook/test` | None | Card preview *(DEBUG=true only)* |

---

## Testing locally

### Health check
```bash
curl http://localhost:5050/health
```

### Simulate a signed webhook
```bash
SECRET="your_secret_here"
BODY=$(cat sample_payload.json)
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')

curl -s -X POST http://localhost:5050/webhook/transcribe-tonic \
     -H "Content-Type: application/json" \
     -H "X-Tonic-Signature: sha256=$SIG" \
     -d "$BODY" | jq
```

### Test card preview (no auth — debug mode only)
```bash
# Set DEBUG=true in .env first
curl -s -X POST http://localhost:5050/webhook/test \
     -H "Content-Type: application/json" \
     -d @sample_payload.json | jq
```

---

## Project structure

```
Meetings/
├── default.nix                        ← nix dev environment (go, gopls, go-tools)
├── .envrc                             ← direnv hook
├── .env.example                       ← secrets template
├── .gitignore
├── go.mod
├── go.sum
├── main.go                            ← entry point, server, graceful shutdown
├── internal/
│   ├── config/config.go               ← env var loading & validation
│   ├── webhook/handler.go             ← HTTP handlers + HMAC verification
│   └── teams/notifier.go              ← Adaptive Card builder + Teams POST
├── Makefile                           ← build / install / start / stop / logs
├── com.sinch.meetings.plist.tmpl      ← launchd service template
├── sample_payload.json
└── README.md
```

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TRANSCRIBE_TONIC_WEBHOOK_SECRET` | ✅ | — | HMAC signing secret |
| `TEAMS_WEBHOOK_URL` | ✅ | — | Teams Incoming Webhook URL |
| `FLASK_HOST` | No | `0.0.0.0` | Bind address |
| `FLASK_PORT` | No | `5050` | Port |
| `DEBUG` | No | `false` | Enable debug logging + `/webhook/test` |

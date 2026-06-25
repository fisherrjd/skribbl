# Transcribe Tonic → Meetings KB

A Go service that ingests meeting transcripts from **Transcribe Tonic**, runs them through a local **LM Studio** model, and writes structured meeting summaries and people profiles into an **Obsidian vault**.

```
Transcribe Tonic (cloud)
        │  POST /webhook/transcribe-tonic
        │  Header: X-Tonic-Signature: sha256=<hmac-sha256>
        ▼
┌──────────────────────────────────────────┐
│  main.go  (net/http + graceful shutdown) │   localhost:5050
│  • HMAC-SHA256 verification              │
│  • JSON parsing & structured logging     │
│  • Async processor dispatch              │
└──────────────┬───────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────┐
│  processor                               │
│  • Loads Dictionary.md corrections       │
│  • Saves raw transcript                  │
│  • Generates meeting summary (LM Studio) │
│  • Updates per-person profiles           │
└──────────┬───────────────────────────────┘
           │  HTTP  │  Writes markdown
           ▼        ▼
     LM Studio   Obsidian Vault
     (local)     ~/vaults/meetings/
```

---

## How it works

When a meeting ends, Transcribe Tonic POSTs the transcript to the webhook. The processor then:

1. **Saves the raw transcript** immediately — source material is never lost
2. **Loads `Dictionary.md`** from the vault root — any known transcription corrections are injected into the LLM system prompt
3. **Generates a summary** via LM Studio — written as an Obsidian-native note with YAML frontmatter, a callout overview, and a structured action items table
4. **Updates person profiles** for every participant — each person gets a running profile in `people/` that is enriched after every meeting they appear in

### Vault structure

```
~/vaults/meetings/
├── Dictionary.md                          ← transcription correction reference
├── meetings/
│   └── YYYY-MM-DD-<slug>/
│       ├── transcript.md                  ← raw transcript
│       └── summary.md                     ← AI-generated summary
└── people/
    └── <name-slug>.md                     ← per-person running profile
```

### Summary format

Summaries are written as Obsidian-native markdown:

```markdown
---
date: 2026-06-24
duration: 43m
software: Microsoft Teams
participants:
  - "[[Person One]]"
  - "[[Person Two]]"
tags:
  - meeting
---

# Meeting Title

> [!abstract] Overview
> 2–3 sentence overview of the meeting.

## Key Discussion Points
- **Topic:** detail

## Decisions Made
- Decision made

## Action Items

| Owner | Action | Status |
|---|---|---|
| [[Person One]] | Do the thing | ⬜ |

## Next Steps
- Follow-up item

---
[[Person One]] · [[Person Two]]
```

---

## Transcription Dictionary

`Dictionary.md` at the vault root is a manually maintained reference of known transcription errors — words or names that Transcribe Tonic consistently mishears. It is loaded fresh on every webhook, so edits take effect immediately without a service restart.

Add entries whenever you spot a mistake in a summary or transcript:

| Transcribed As | Correct Term | Notes |
|---|---|---|
| KOP | COP | Sinch's internal deployment & observability platform |

---

## Service management

The service runs as a **nix-darwin launchd agent** on `gjallar`. It starts automatically on login and restarts on crash.

```bash
# check status
launchctl list | grep sinch

# tail logs
tail -f ~/Library/Logs/sinch-meetings/meetings.log

# stop / start
launchctl stop org.nixos.com.sinch.meetings
launchctl start org.nixos.com.sinch.meetings
```

Logs live at `~/Library/Logs/sinch-meetings/meetings.log`.

### Deploying changes

The service is managed through the `cfg` nix-darwin flake. To deploy any code changes:

```bash
# 1 — commit changes in this repo
cd ~/Sinch/Meetings
git add -A && git commit -m "your message"

# 2 — update the lock in cfg to pick up the new commit
cd ~/cfg
nix flake lock --update-input sinch-meetings

# 3 — rebuild and restart
darwin-rebuild switch --flake .#gjallar
```

nix-darwin handles rebuilding the binary, updating the plist, and bouncing the service automatically.

### One-off / dev run

```bash
cd ~/Sinch/Meetings
go run .
```

The service loads its config from `.env` in the working directory. The live secrets live at `~/.config/sinch/meetings/.env`.

---

## Manual ingest CLI

Use the ingest tool to manually push a Transcribe Tonic `.txt` file through the pipeline — useful for reprocessing old meetings or ones that didn't come through automatically.

```bash
# the service must be running first
./ingest -file ~/path/to/transcript.txt

# against a different target (e.g. dev instance)
./ingest -file ~/path/to/transcript.txt -url http://localhost:5050/webhook/transcribe-tonic
```

The tool parses the meeting title and timestamp directly from the Transcribe Tonic filename format:
```
Teams transcript-<Title> at MM-DD-YYYY, HH-MM AM on.txt
```

Do not rename the file before ingesting.

To rebuild the binary:
```bash
cd ~/Sinch/Meetings
go build -o ingest ./cmd/ingest
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
3. **Secret**: same value as `TRANSCRIBE_TONIC_WEBHOOK_SECRET` in `.env`

---

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | None | Liveness probe |
| `POST` | `/webhook/transcribe-tonic` | HMAC-SHA256 | Primary receiver |
| `POST` | `/webhook/test` | None | Payload preview *(DEBUG=true only)* |

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

---

## Project structure

```
Meetings/
├── default.nix                        ← nix dev environment
├── flake.nix                          ← nix package + nix-darwin module
├── module.nix                         ← launchd service + vault git autocommit
├── .envrc                             ← direnv hook
├── .env.example                       ← secrets template
├── main.go                            ← entry point, HTTP server, graceful shutdown
├── cmd/
│   └── ingest/main.go                 ← CLI: manually ingest a .txt transcript
├── internal/
│   ├── config/config.go               ← env var loading & validation
│   ├── webhook/
│   │   ├── handler.go                 ← HTTP handlers + HMAC verification
│   │   └── payload.go                 ← Transcribe Tonic payload types
│   ├── lmstudio/client.go             ← LM Studio HTTP client
│   ├── processor/
│   │   ├── processor.go               ← orchestrates transcript → vault pipeline
│   │   └── prompts.go                 ← LLM system prompts + user prompt builders
│   └── kb/kb.go                       ← vault file I/O helpers
├── sample_payload.json
└── README.md
```

---

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `VAULT_PATH` | Yes | — | Absolute path to the Obsidian vault |
| `TRANSCRIBE_TONIC_WEBHOOK_SECRET` | No | — | HMAC signing secret — skips verification if empty |
| `LM_STUDIO_URL` | No | `http://localhost:1234` | LM Studio server URL |
| `LM_MODEL` | No | `qwen3-27b-instruct` | Model identifier passed to LM Studio |
| `HOST` | No | `0.0.0.0` | Bind address |
| `PORT` | No | `5050` | Port |
| `DEBUG` | No | `false` | Verbose logging + enables `/webhook/test` |

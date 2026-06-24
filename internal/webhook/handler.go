package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	secret    string
	debug     bool
	processor Processor
}

// Processor is anything that can handle a payload asynchronously.
type Processor interface {
	Process(p Payload)
}

func NewHandler(secret string, debug bool, proc Processor) *Handler {
	return &Handler{secret: secret, debug: debug, processor: proc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /webhook/transcribe-tonic", h.transcribeTonic)
	if h.debug {
		mux.HandleFunc("POST /webhook/test", h.test)
		slog.Info("debug mode: POST /webhook/test enabled")
	}
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"ts":     time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) transcribeTonic(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20)) // 4 MB
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("could not read body"))
		return
	}

	if !verifySignature(h.secret, raw, r.Header.Get("X-Tonic-Signature")) {
		slog.Warn("signature verification failed", "remote", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, errBody("invalid signature"))
		return
	}

	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid JSON"))
		return
	}

	slog.Info("received webhook", "event", p.Event, "meeting_id", p.MeetingID, "title", p.Title)

	// ACK immediately — processing is async
	writeJSON(w, http.StatusOK, map[string]any{"received": true})
	go h.processor.Process(p)
}

// test accepts any JSON without signature verification — debug only.
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("could not read body"))
		return
	}
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid JSON"))
		return
	}
	slog.Info("[test] dispatching payload to processor")
	writeJSON(w, http.StatusOK, map[string]any{"received": true})
	go h.processor.Process(p)
}

func verifySignature(secret string, body []byte, headerSig string) bool {
	if headerSig == "" {
		slog.Warn("missing X-Tonic-Signature header")
		return false
	}
	sigHex := strings.TrimPrefix(headerSig, "sha256=")
	provided, err := hex.DecodeString(sigHex)
	if err != nil {
		slog.Warn("signature is not valid hex")
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), provided)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writing JSON response", "err", err)
	}
}

func errBody(msg string) map[string]string {
	return map[string]string{"error": fmt.Sprintf(msg)}
}

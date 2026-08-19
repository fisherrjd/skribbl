package lmstudio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Manager loads a model into LM Studio before a meeting is processed and
// unloads it afterwards, so a local model only occupies VRAM while skribbl is
// actually working.
//
// It shells out to the `lms` CLI: LM Studio's OpenAI-compatible endpoint has no
// load/unload route. Disabled by default — hosted backends (MiniMax) have no
// local model to manage.
type Manager struct {
	enabled bool
	bin     string
	model   string
	ctxLen  int // 0 = LM Studio's per-model default
	ttl     int // seconds of idleness before LM Studio unloads it itself; 0 = none

	// loadedByUs guards against unloading a model somebody else loaded: if the
	// model was already resident when Load ran, another consumer may be using
	// it and Unload must leave it alone.
	loadedByUs bool
}

// loadTimeout is generous: a 20 GB model can take a while to page into VRAM.
const (
	loadTimeout  = 10 * time.Minute
	shortTimeout = 30 * time.Second
)

func NewManager(enabled bool, bin, model string, ctxLen, ttl int) *Manager {
	if bin == "" {
		bin = "lms"
	}
	return &Manager{enabled: enabled, bin: bin, model: model, ctxLen: ctxLen, ttl: ttl}
}

// psEntry is the subset of `lms ps --json` we care about.
type psEntry struct {
	Identifier    string `json:"identifier"`
	ModelKey      string `json:"modelKey"`
	ContextLength int    `json:"contextLength"`
}

// Load makes the model resident. It is idempotent: if the model is already
// loaded (by a previous run, or by the user), Load adopts that instance rather
// than loading a second copy, and a matching Unload will leave it alone.
func (m *Manager) Load() error {
	if !m.enabled {
		return nil
	}

	if entry, err := m.loaded(); err != nil {
		slog.Warn("could not list loaded models, loading anyway", "err", err)
	} else if entry != nil {
		// Adopting somebody else's instance means adopting its context window,
		// which may be too small for a full transcript — say so rather than
		// silently truncating.
		if m.ctxLen > 0 && entry.ContextLength < m.ctxLen {
			slog.Warn("model already loaded with a smaller context window; leaving it as is",
				"model", m.model, "loaded_context", entry.ContextLength, "wanted", m.ctxLen)
		} else {
			slog.Info("model already loaded", "model", m.model, "context", entry.ContextLength)
		}
		m.loadedByUs = false
		return nil
	}

	args := []string{"load", m.model, "--yes"}
	if m.ctxLen > 0 {
		args = append(args, "--context-length", strconv.Itoa(m.ctxLen))
	}
	if m.ttl > 0 {
		args = append(args, "--ttl", strconv.Itoa(m.ttl))
	}

	slog.Info("loading model", "model", m.model, "context", m.ctxLen, "ttl", m.ttl)
	start := time.Now()
	if out, err := m.run(loadTimeout, args...); err != nil {
		return fmt.Errorf("lms load: %w: %s", err, lastLine(out))
	}
	m.loadedByUs = true
	slog.Info("model loaded", "model", m.model, "took", time.Since(start).Round(time.Second))

	// LM Studio's saved per-model config outranks --context-length (verified
	// 2026-08-13: -c 4096 still loaded at 183296), so read back what we
	// actually got rather than trusting the flag. Too small silently truncates
	// the transcript, which is invisible in the output.
	if m.ctxLen > 0 {
		if entry, err := m.loaded(); err == nil && entry != nil && entry.ContextLength < m.ctxLen {
			slog.Warn("LM Studio ignored the requested context window; long transcripts may be truncated",
				"model", m.model, "got", entry.ContextLength, "wanted", m.ctxLen,
				"fix", "set the context length in LM Studio's per-model settings")
		}
	}
	return nil
}

// Unload frees the model, but only if this Manager is the one that loaded it.
func (m *Manager) Unload() {
	if !m.enabled || !m.loadedByUs {
		return
	}
	m.loadedByUs = false

	slog.Info("unloading model", "model", m.model)
	if out, err := m.run(shortTimeout, "unload", m.model); err != nil {
		// Not fatal: the --ttl set at load time is the backstop.
		slog.Error("unloading model", "model", m.model, "err", err, "out", out)
		return
	}
	slog.Info("model unloaded", "model", m.model)
}

// loaded returns the resident instance matching our model, or nil.
func (m *Manager) loaded() (*psEntry, error) {
	out, err := m.run(shortTimeout, "ps", "--json")
	if err != nil {
		return nil, fmt.Errorf("lms ps: %w: %s", err, out)
	}
	var entries []psEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return nil, fmt.Errorf("parsing lms ps output: %w", err)
	}
	for i, e := range entries {
		if e.Identifier == m.model || e.ModelKey == m.model {
			return &entries[i], nil
		}
	}
	return nil, nil
}

func (m *Manager) run(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, m.bin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// lastLine trims `lms load`'s progress spinner (hundreds of \r-separated frames
// with ANSI escapes) down to the final line, so a failure logs one readable
// message instead of a screenful.
func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, "\r\n"); i != -1 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

package processor

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"skribbl/internal/kb"
	"skribbl/internal/lmstudio"
	"skribbl/internal/webhook"
)

type Processor struct {
	lm        *lmstudio.Client
	vaultPath string
	mu        sync.Mutex
}

func New(lm *lmstudio.Client, vaultPath string) *Processor {
	return &Processor{lm: lm, vaultPath: vaultPath}
}

func (p *Processor) Process(payload webhook.Payload) {
	title := coalesce(payload.MeetingTitle, "Untitled Meeting")
	date := dateStr(payload.MeetingStartTimestamp)

	if !p.mu.TryLock() {
		slog.Info("processor busy; queued meeting", "title", title, "date", date)
		p.mu.Lock()
	}
	defer p.mu.Unlock()

	slog.Info("processing meeting", "title", title, "date", date, "software", payload.MeetingSoftware)

	if err := kb.EnsureDirs(p.vaultPath); err != nil {
		slog.Error("ensuring vault dirs", "err", err)
		return
	}

	meetingDir, err := kb.MeetingDir(p.vaultPath, payload.MeetingStartTimestamp, title)
	if err != nil {
		slog.Error("creating meeting dir", "err", err)
		return
	}

	transcript := payload.TranscriptString()

	// load dictionary fresh each run — edits take effect without a restart
	dict, err := kb.ReadDictionary(p.vaultPath)
	if err != nil {
		slog.Warn("reading dictionary", "err", err)
	} else if dict != "" {
		slog.Info("dictionary loaded", "bytes", len(dict))
	} else {
		slog.Info("no dictionary found, proceeding without corrections")
	}

	// 1. save raw transcript immediately — never lose source material
	if transcript != "" {
		content := fmt.Sprintf("# Transcript: %s\n\n**Date:** %s\n**Software:** %s\n\n---\n\n%s\n",
			title, date, payload.MeetingSoftware, transcript)
		path := filepath.Join(meetingDir, "transcript.md")
		if err := kb.WriteFile(path, content); err != nil {
			slog.Error("writing transcript", "err", err)
		} else {
			slog.Info("transcript saved", "path", path)
		}
	}

	// 2. generate meeting summary
	p.generateSummary(payload, title, date, meetingDir, transcript, dict)

	// 3. update person profiles
	participants := payload.Participants()
	for _, name := range participants {
		p.updatePersonProfile(payload, name, title, date, transcript, dict)
	}

	slog.Info("processing complete", "title", title, "participants", len(participants))
}

func (p *Processor) generateSummary(payload webhook.Payload, title, date, meetingDir, transcript, dict string) {
	if transcript == "" {
		slog.Warn("empty transcript, skipping summary", "title", title)
		return
	}

	duration := calcDuration(payload.MeetingStartTimestamp, payload.MeetingEndTimestamp)
	participants := payload.Participants()

	slog.Info("generating meeting summary", "title", title)

	summary, err := p.lm.Complete(buildSummarySystem(dict), summaryPrompt(title, date, duration, payload.MeetingSoftware, participants, transcript))
	if err != nil {
		slog.Error("generating summary", "err", err, "title", title)
		return
	}

	path := filepath.Join(meetingDir, "summary.md")
	summary = normalizeGeneratedSummary(strings.TrimSpace(summary))
	if err := kb.WriteFile(path, summary+"\n"); err != nil {
		slog.Error("writing summary", "err", err)
		return
	}
	slog.Info("summary saved", "path", path)
}

func (p *Processor) updatePersonProfile(payload webhook.Payload, name, meetingTitle, date, transcript, dict string) {
	turns := payload.SpeakerTurns(name)
	if turns == "" {
		slog.Info("no speaker turns found, skipping", "person", name)
		return
	}

	existing, err := kb.ReadPersonProfile(p.vaultPath, name)
	if err != nil {
		slog.Warn("reading existing profile", "person", name, "err", err)
	}

	allParticipants := strings.Join(payload.Participants(), ", ")

	slog.Info("updating person profile", "person", name)

	updated, err := p.lm.Complete(buildProfileSystem(dict), profilePrompt(name, date, meetingTitle, existing, turns, allParticipants))
	if err != nil {
		slog.Error("generating profile", "person", name, "err", err)
		return
	}

	updated = strings.TrimSpace(updated)
	if err := kb.WritePersonProfile(p.vaultPath, name, updated+"\n\n"+meetingHistoryBlock(name)+"\n"); err != nil {
		slog.Error("writing profile", "person", name, "err", err)
		return
	}
	slog.Info("profile updated", "person", name)
}

func calcDuration(start, end string) string {
	s, err1 := time.Parse(time.RFC3339, start)
	e, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		return "unknown"
	}
	dur := e.Sub(s)
	h := int(dur.Hours())
	m := int(dur.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func dateStr(ts string) string {
	formats := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, ts); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return time.Now().UTC().Format("2006-01-02")
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

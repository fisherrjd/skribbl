package processor

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"sinch/meetings/internal/kb"
	"sinch/meetings/internal/lmstudio"
	"sinch/meetings/internal/webhook"
)

type Processor struct {
	lm        *lmstudio.Client
	vaultPath string
}

func New(lm *lmstudio.Client, vaultPath string) *Processor {
	return &Processor{lm: lm, vaultPath: vaultPath}
}

// Process is the main entry point — called in a background goroutine.
func (p *Processor) Process(payload webhook.Payload) {
	title := coalesce(payload.Title, payload.MeetingTitle, "Untitled Meeting")
	startedAt := coalesce(payload.StartedAt, payload.StartTime)
	date := dateStr(startedAt)

	slog.Info("processing meeting", "title", title, "date", date)

	// ensure vault dirs exist
	if err := kb.EnsureDirs(p.vaultPath); err != nil {
		slog.Error("ensuring vault dirs", "err", err)
		return
	}

	// get or create meeting directory
	meetingDir, err := kb.MeetingDir(p.vaultPath, startedAt, title)
	if err != nil {
		slog.Error("creating meeting dir", "err", err)
		return
	}

	// 1. save raw transcript immediately — never lose the source material
	if payload.Transcript != "" {
		transcriptPath := filepath.Join(meetingDir, "transcript.md")
		content := fmt.Sprintf("# Transcript: %s\n\n**Date:** %s\n\n---\n\n%s\n", title, date, payload.Transcript)
		if err := kb.WriteFile(transcriptPath, content); err != nil {
			slog.Error("writing transcript", "err", err)
		} else {
			slog.Info("transcript saved", "path", transcriptPath)
		}
	}

	// 2. generate and save meeting summary
	p.generateSummary(payload, title, date, meetingDir)

	// 3. update person profiles
	participants := extractParticipants(payload)
	for _, name := range participants {
		p.updatePersonProfile(payload, name, title, date)
	}

	slog.Info("processing complete", "title", title, "participants", len(participants))
}

// ── summary ───────────────────────────────────────────────────────────────────

func (p *Processor) generateSummary(payload webhook.Payload, title, date, meetingDir string) {
	if payload.Transcript == "" {
		slog.Warn("no transcript in payload, skipping summary", "title", title)
		return
	}

	duration := formatDuration(payload.DurationSecs)
	participants := participantNames(payload.Participants)

	slog.Info("generating meeting summary", "title", title)

	summary, err := p.lm.Complete(
		summarySystem,
		summaryPrompt(title, date, duration, participants, payload.Transcript),
	)
	if err != nil {
		slog.Error("generating summary", "err", err, "title", title)
		return
	}

	summaryPath := filepath.Join(meetingDir, "summary.md")
	if err := kb.WriteFile(summaryPath, summary+"\n"); err != nil {
		slog.Error("writing summary", "err", err)
		return
	}
	slog.Info("summary saved", "path", summaryPath)
}

// ── person profiles ───────────────────────────────────────────────────────────

func (p *Processor) updatePersonProfile(payload webhook.Payload, name, meetingTitle, date string) {
	if payload.Transcript == "" {
		return
	}

	// extract only this person's speaker turns
	turns := extractSpeakerTurns(payload.Transcript, name)
	if turns == "" {
		slog.Info("no speaker turns found, skipping profile update", "person", name)
		return
	}

	// read existing profile
	existing, err := kb.ReadPersonProfile(p.vaultPath, name)
	if err != nil {
		slog.Warn("reading existing profile", "person", name, "err", err)
	}

	allParticipants := participantNames(payload.Participants)

	slog.Info("updating person profile", "person", name)

	updated, err := p.lm.Complete(
		profileSystem,
		profilePrompt(name, date, meetingTitle, existing, turns, allParticipants),
	)
	if err != nil {
		slog.Error("generating profile", "person", name, "err", err)
		return
	}

	if err := kb.WritePersonProfile(p.vaultPath, name, updated+"\n"); err != nil {
		slog.Error("writing profile", "person", name, "err", err)
		return
	}
	slog.Info("profile updated", "person", name)
}

// ── transcript parsing ────────────────────────────────────────────────────────

// speakerLineRe matches lines like:  "Alice Lee: text"  or  "[Alice Lee]: text"
var speakerLineRe = regexp.MustCompile(`(?m)^[\[\s]*([A-Za-z][A-Za-z .'-]+?)[\]\s]*:\s+(.+)$`)

// extractSpeakerTurns returns all lines spoken by name (case-insensitive match).
func extractSpeakerTurns(transcript, name string) string {
	nameLower := strings.ToLower(name)
	var turns []string
	for _, match := range speakerLineRe.FindAllStringSubmatch(transcript, -1) {
		speaker := strings.TrimSpace(match[1])
		if strings.ToLower(speaker) == nameLower {
			turns = append(turns, speaker+": "+match[2])
		}
	}
	return strings.Join(turns, "\n")
}

// extractParticipants merges the explicit participant list with any speakers
// found in the transcript itself.
func extractParticipants(payload webhook.Payload) []string {
	seen := make(map[string]bool)
	var names []string

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if !seen[key] {
			seen[key] = true
			names = append(names, name)
		}
	}

	for _, p := range payload.Participants {
		if p.Name != "" {
			add(p.Name)
		}
	}

	// also pick up any speakers from the transcript not in the participant list
	for _, match := range speakerLineRe.FindAllStringSubmatch(payload.Transcript, -1) {
		add(strings.TrimSpace(match[1]))
	}

	return names
}

// ── helpers ───────────────────────────────────────────────────────────────────

func participantNames(ps []webhook.Participant) string {
	names := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return strings.Join(names, ", ")
}

func formatDuration(secs int) string {
	if secs == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%dm %ds", secs/60, secs%60)
}

func dateStr(startedAt string) string {
	formats := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, startedAt); err == nil {
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

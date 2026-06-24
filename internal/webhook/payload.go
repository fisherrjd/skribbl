package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Payload is the webhook body sent by Transcriptonic.
// transcript and chatMessages are union types ([]Block or string)
// depending on webhookBodyType — we handle both.
type Payload struct {
	WebhookBodyType       string          `json:"webhookBodyType"`       // "simple" | "advanced"
	MeetingSoftware       string          `json:"meetingSoftware"`       // "Teams" | "Google Meet" | "Zoom"
	MeetingTitle          string          `json:"meetingTitle"`
	MeetingStartTimestamp string          `json:"meetingStartTimestamp"` // ISO-8601
	MeetingEndTimestamp   string          `json:"meetingEndTimestamp"`   // ISO-8601
	RawTranscript         json.RawMessage `json:"transcript"`
	RawChatMessages       json.RawMessage `json:"chatMessages"`
}

type TranscriptBlock struct {
	PersonName     string `json:"personName"`
	Timestamp      string `json:"timestamp"`
	TranscriptText string `json:"transcriptText"`
}

type ChatMessage struct {
	PersonName      string `json:"personName"`
	Timestamp       string `json:"timestamp"`
	ChatMessageText string `json:"chatMessageText"`
}

// TranscriptBlocks returns structured blocks (advanced mode) or nil (simple mode).
func (p *Payload) TranscriptBlocks() []TranscriptBlock {
	if p.WebhookBodyType != "advanced" {
		return nil
	}
	var blocks []TranscriptBlock
	if err := json.Unmarshal(p.RawTranscript, &blocks); err != nil {
		return nil
	}
	return blocks
}

// TranscriptString returns a flat speaker-labelled string suitable for LM prompts.
// Works for both simple (already a string) and advanced (formatted from blocks).
func (p *Payload) TranscriptString() string {
	if p.WebhookBodyType == "advanced" {
		blocks := p.TranscriptBlocks()
		var sb strings.Builder
		for _, b := range blocks {
			sb.WriteString(fmt.Sprintf("%s: %s\n", b.PersonName, b.TranscriptText))
		}
		return strings.TrimSpace(sb.String())
	}
	// simple mode — transcript is already a plain string
	var s string
	if err := json.Unmarshal(p.RawTranscript, &s); err != nil {
		return ""
	}
	return s
}

// Participants returns unique speaker names from the transcript.
func (p *Payload) Participants() []string {
	seen := make(map[string]bool)
	var names []string

	if p.WebhookBodyType == "advanced" {
		for _, b := range p.TranscriptBlocks() {
			key := strings.ToLower(b.PersonName)
			if b.PersonName != "" && !seen[key] {
				seen[key] = true
				names = append(names, b.PersonName)
			}
		}
		return names
	}

	// simple mode — parse "Name: text" lines
	var s string
	_ = json.Unmarshal(p.RawTranscript, &s)
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			name := strings.TrimSpace(line[:idx])
			key := strings.ToLower(name)
			if !seen[key] {
				seen[key] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// SpeakerTurns returns all lines spoken by a specific person.
func (p *Payload) SpeakerTurns(name string) string {
	nameLower := strings.ToLower(name)

	if p.WebhookBodyType == "advanced" {
		var turns []string
		for _, b := range p.TranscriptBlocks() {
			if strings.ToLower(b.PersonName) == nameLower {
				turns = append(turns, fmt.Sprintf("%s: %s", b.PersonName, b.TranscriptText))
			}
		}
		return strings.Join(turns, "\n")
	}

	// simple mode — filter lines
	var s string
	_ = json.Unmarshal(p.RawTranscript, &s)
	var turns []string
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			if strings.ToLower(strings.TrimSpace(line[:idx])) == nameLower {
				turns = append(turns, line)
			}
		}
	}
	return strings.Join(turns, "\n")
}

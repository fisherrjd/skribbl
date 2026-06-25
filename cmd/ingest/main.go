// cmd/ingest/main.go — manually ingest a Transcriptonic .txt file into the pipeline.
//
// Usage:
//   go run ./cmd/ingest -file ~/Downloads/TranscripTonic/<file>.txt
//
// Parses the Transcriptonic plain-text format and POSTs it to the local
// webhook as an advanced payload, exactly as if Transcriptonic sent it live.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// matches:  "Person Name (06/24/2026, 11:29 AM)"
var speakerRe = regexp.MustCompile(`^(.+?)\s+\((\d{2}/\d{2}/\d{4}),\s+(\d{1,2}:\d{2}\s+[AP]M)\)$`)

type transcriptBlock struct {
	PersonName     string `json:"personName"`
	Timestamp      string `json:"timestamp"`
	TranscriptText string `json:"transcriptText"`
}

type payload struct {
	WebhookBodyType       string            `json:"webhookBodyType"`
	MeetingSoftware       string            `json:"meetingSoftware"`
	MeetingTitle          string            `json:"meetingTitle"`
	MeetingStartTimestamp string            `json:"meetingStartTimestamp"`
	MeetingEndTimestamp   string            `json:"meetingEndTimestamp"`
	Transcript            []transcriptBlock `json:"transcript"`
	ChatMessages          []any             `json:"chatMessages"`
}

func main() {
	filePath := flag.String("file", "", "path to Transcriptonic .txt file (required)")
	webhookURL := flag.String("url", "http://localhost:5050/webhook/transcribe-tonic", "webhook URL")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("usage: go run ./cmd/ingest -file <path>")
	}

	expanded := expandHome(*filePath)
	data, err := os.ReadFile(expanded)
	if err != nil {
		log.Fatalf("reading file: %v", err)
	}

	title, startTS := parseFilename(filepath.Base(expanded))
	blocks := parseTranscript(string(data))

	endTS := startTS
	if len(blocks) > 0 {
		endTS = blocks[len(blocks)-1].Timestamp
	}

	p := payload{
		WebhookBodyType:       "advanced",
		MeetingSoftware:       "Teams",
		MeetingTitle:          title,
		MeetingStartTimestamp: startTS,
		MeetingEndTimestamp:   endTS,
		Transcript:            blocks,
		ChatMessages:          []any{},
	}

	body, _ := json.MarshalIndent(p, "", "  ")

	fmt.Printf("→ title:        %s\n", title)
	fmt.Printf("→ start:        %s\n", startTS)
	fmt.Printf("→ blocks:       %d\n", len(blocks))
	fmt.Printf("→ posting to:   %s\n\n", *webhookURL)

	resp, err := http.Post(*webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("✓ accepted — check logs for processing progress")
	} else {
		fmt.Printf("✗ HTTP %s\n", resp.Status)
	}
}

// parseTranscript parses the Transcriptonic plain-text format into blocks.
func parseTranscript(text string) []transcriptBlock {
	var blocks []transcriptBlock
	lines := strings.Split(strings.TrimSpace(text), "\n")

	var currentSpeaker, currentTS string
	var textLines []string

	flush := func() {
		if currentSpeaker != "" && len(textLines) > 0 {
			blocks = append(blocks, transcriptBlock{
				PersonName:     currentSpeaker,
				Timestamp:      currentTS,
				TranscriptText: strings.TrimSpace(strings.Join(textLines, " ")),
			})
		}
		textLines = nil
	}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if m := speakerRe.FindStringSubmatch(line); m != nil {
			flush()
			currentSpeaker = strings.TrimSpace(m[1])
			currentTS = parseTimestamp(m[2], m[3])
		} else if strings.TrimSpace(line) == "" {
			flush()
			currentSpeaker = ""
		} else {
			textLines = append(textLines, line)
		}
	}
	flush()
	return blocks
}

// parseFilename extracts title and ISO timestamp from the Transcriptonic filename.
// Format: "Teams transcript-<title> at MM-DD-YYYY, HH-MM AM on.txt"
func parseFilename(name string) (title, isoTS string) {
	name = strings.TrimSuffix(name, ".txt")

	// strip "Teams transcript-" prefix
	name = strings.TrimPrefix(name, "Teams transcript-")

	// split on " at "
	parts := strings.SplitN(name, " at ", 2)
	title = strings.TrimSpace(parts[0])

	if len(parts) < 2 {
		return title, time.Now().UTC().Format(time.RFC3339)
	}

	// "06-24-2026, 11-29 AM on" → parse date/time
	datePart := strings.TrimSuffix(strings.TrimSpace(parts[1]), " on")
	// replace hyphens in time part: "11-29 AM" → "11:29 AM"
	// format: "06-24-2026, 11-29 AM"
	re := regexp.MustCompile(`(\d{2})-(\d{2})-(\d{4}),\s+(\d{1,2})-(\d{2})\s+(AM|PM)`)
	if m := re.FindStringSubmatch(datePart); m != nil {
		dtStr := fmt.Sprintf("%s/%s/%s %s:%s %s", m[1], m[2], m[3], m[4], m[5], m[6])
		if t, err := time.ParseInLocation("01/02/2006 3:04 PM", dtStr, time.Local); err == nil {
			return title, t.UTC().Format(time.RFC3339)
		}
	}

	return title, time.Now().UTC().Format(time.RFC3339)
}

func parseTimestamp(date, timeStr string) string {
	combined := date + " " + timeStr
	if t, err := time.ParseInLocation("01/02/2006 3:04 PM", combined, time.Local); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

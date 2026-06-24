package webhook

// Payload mirrors the Transcribe Tonic webhook body.
type Payload struct {
	Event         string        `json:"event"`
	MeetingID     string        `json:"meeting_id"`
	Title         string        `json:"title"`
	MeetingTitle  string        `json:"meeting_title"`
	StartedAt     string        `json:"started_at"`
	StartTime     string        `json:"start_time"`
	EndedAt       string        `json:"ended_at"`
	DurationSecs  int           `json:"duration_secs"`
	Participants  []Participant `json:"participants"`
	Transcript    string        `json:"transcript"`
	RecordingURL  string        `json:"recording_url"`
	TranscriptURL string        `json:"transcript_url"`
}

type Participant struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

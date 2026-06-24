package kb

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// EnsureDirs creates the top-level vault structure.
func EnsureDirs(vaultPath string) error {
	for _, d := range []string{"meetings", "people"} {
		if err := os.MkdirAll(filepath.Join(vaultPath, d), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", d, err)
		}
	}
	return nil
}

// MeetingDir returns (and creates) the directory for a single meeting.
// Format: YYYY-MM-DD-<slug-title>
func MeetingDir(vaultPath, startedAt, title string) (string, error) {
	date := dateSlug(startedAt)
	slug := titleSlug(title)
	dir := filepath.Join(vaultPath, "meetings", date+"-"+slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create meeting dir: %w", err)
	}
	return dir, nil
}

// WriteFile writes content to path, creating parent dirs as needed.
func WriteFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadPersonProfile reads an existing person profile, returning empty string
// (not an error) if the file does not exist yet.
func ReadPersonProfile(vaultPath, name string) (string, error) {
	path := PersonPath(vaultPath, name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read profile for %q: %w", name, err)
	}
	return string(data), nil
}

// WritePersonProfile overwrites a person's profile file.
func WritePersonProfile(vaultPath, name, content string) error {
	return WriteFile(PersonPath(vaultPath, name), content)
}

// PersonPath returns the file path for a person's profile.
func PersonPath(vaultPath, name string) string {
	return filepath.Join(vaultPath, "people", nameSlug(name)+".md")
}

// ── slug helpers ──────────────────────────────────────────────────────────────

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func titleSlug(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func nameSlug(s string) string {
	return titleSlug(s)
}

func dateSlug(startedAt string) string {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, startedAt); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	// fallback to today
	return time.Now().UTC().Format("2006-01-02")
}

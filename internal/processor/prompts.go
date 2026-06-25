package processor

import (
	"fmt"
	"strings"
)

// ── meeting summary ───────────────────────────────────────────────────────────

func buildSummarySystem(dict string) string {
	base := `You are a professional meeting analyst. Your job is to produce clear, accurate, and useful meeting summaries for a personal knowledge base. Be factual — only include what was actually discussed. Do not fabricate information.`
	if dict == "" {
		return base
	}
	return base + `

## Transcription Correction Dictionary

The raw transcript was produced by an automatic speech recognition tool and may contain errors.
The dictionary below maps known misheard or mistranscribed terms to their correct forms.
Apply these corrections silently whenever you encounter them — do not mention the corrections in the summary.

` + dict
}

func buildProfileSystem(dict string) string {
	base := `You are maintaining a professional knowledge base of people that Jad interacts with at work. Your job is to write or update concise, useful profiles based on meeting transcripts. Be factual and professional. Only include information that can be inferred from the available evidence.`
	if dict == "" {
		return base
	}
	return base + `

## Transcription Correction Dictionary

The raw transcript was produced by an automatic speech recognition tool and may contain errors.
The dictionary below maps known misheard or mistranscribed terms to their correct forms.
Apply these corrections silently — do not mention them in the profile.

` + dict
}

func summaryPrompt(title, date, duration, software string, participants []string, transcript string) string {
	// build YAML frontmatter — generated in Go so it's always valid
	yamlLines := make([]string, len(participants))
	for i, p := range participants {
		yamlLines[i] = fmt.Sprintf("  - \"[[%s]]\"", p)
	}
	frontmatter := fmt.Sprintf(
		"---\ndate: %s\nduration: %s\nsoftware: %s\nparticipants:\n%s\ntags:\n  - meeting\n---",
		date, duration, software, strings.Join(yamlLines, "\n"),
	)

	// wikilinks for the footer
	wikilinks := make([]string, len(participants))
	for i, p := range participants {
		wikilinks[i] = fmt.Sprintf("[[%s]]", p)
	}
	footerLinks := strings.Join(wikilinks, " · ")

	return fmt.Sprintf(`Analyze the following meeting transcript and produce a summary in markdown for an Obsidian knowledge base.

Output the note using EXACTLY this structure — do not add, remove, or rename any sections:

%s

# %s

> [!abstract] Overview
> (2–3 sentences describing what the meeting was about and its overall outcome — written as a single flowing paragraph inside the callout)

## Key Discussion Points
(bullet points — use **bold** for the topic label followed by a colon and detail)

## Decisions Made
(bullet list of concrete decisions; if none write exactly: *None recorded.*)

## Action Items

| Owner | Action | Status |
|---|---|---|
(one row per action item — Owner must be a wikilink e.g. [[Name]], Status must be ⬜; if no action items write exactly: *None recorded.*)

## Next Steps
(bullet list of follow-ups, deadlines, or planned activities; if none write exactly: *None recorded.*)

---
%s

---

Transcript:
%s`, frontmatter, title, footerLinks, transcript)
}

// ── person profile ────────────────────────────────────────────────────────────

func profilePrompt(name, date, meetingTitle, existingProfile, speakerTurns, allParticipants string) string {
	existing := existingProfile
	if existing == "" {
		existing = "(no existing profile — this is the first recorded meeting with this person)"
	}

	return fmt.Sprintf(`Write or update the Obsidian profile note for %s using their existing profile and their contributions in a recent meeting.

Guidelines:
- Rewrite as a single cohesive markdown document
- Incorporate new information, update anything that has changed
- Do not remove accurate existing information unless contradicted
- Keep it concise and useful as a quick reference
- Base all claims on evidence from the transcript only
- Do not invent or assume information that is not present

Output the note using EXACTLY this structure — do not add, remove, or rename any sections:

---
name: %s
role: (inferred job title — or leave blank if unknown)
company: (inferred company — or leave blank if unknown)
last_updated: %s
tags:
  - person
---

# %s

> [!abstract]
> (one sentence professional summary — who they are and what they do)

## Role & Organisation
(job title, team, and company — 1 to 3 lines)

## Background & Expertise
(bullet list of relevant skills, experience, and domain knowledge)

## Working Style
(bullet list of how they communicate, engage in meetings, and areas of focus)

## Meeting History

| Date | Meeting | Notes |
|---|---|---|
(one row per meeting — Date as YYYY-MM-DD, Meeting as plain title, Notes as a single line on their role or contribution)

## Key Topics & Interests
(bullet list of recurring themes, projects, and technologies they discuss)

## Notes
(any other useful context — omit this section entirely if there is nothing to add)

---

Existing Profile:
%s

---

Recent Meeting: %s (%s)
Other participants for context: %s

Their speaker turns from this meeting:
%s`, name, name, name, date, name, existing, meetingTitle, date, allParticipants, speakerTurns)
}

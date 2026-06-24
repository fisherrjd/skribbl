package processor

import "fmt"

// ── meeting summary ───────────────────────────────────────────────────────────

var summarySystem = `You are a professional meeting analyst. Your job is to produce clear, accurate, and useful meeting summaries for a personal knowledge base. Be factual — only include what was actually discussed. Do not fabricate information.`

func summaryPrompt(title, date, duration, participants, transcript string) string {
	return fmt.Sprintf(`Analyze the following meeting transcript and produce a comprehensive summary in markdown.

Use exactly this structure:

# %s

**Date:** %s
**Duration:** %s
**Participants:** %s

## Overview
(2–3 sentences describing what the meeting was about and its overall outcome)

## Key Discussion Points
(bullet points covering the main topics discussed)

## Decisions Made
(list any concrete decisions reached; if none, write "None recorded")

## Action Items
(list action items with owner where identifiable; if none, write "None recorded")

## Next Steps
(follow-up meetings, deadlines, or planned activities mentioned; if none, write "None recorded")

---

Transcript:
%s`, title, date, duration, participants, transcript)
}

// ── person profile ────────────────────────────────────────────────────────────

var profileSystem = `You are maintaining a professional knowledge base of people that Jad interacts with at work. Your job is to write or update concise, useful profiles based on meeting transcripts. Be factual and professional. Only include information that can be inferred from the available evidence.`

func profilePrompt(name, date, meetingTitle, existingProfile, speakerTurns, allParticipants string) string {
	existing := existingProfile
	if existing == "" {
		existing = "(no existing profile — this is the first recorded meeting with this person)"
	}

	return fmt.Sprintf(`Update the profile for %s using their existing profile and their contributions in a recent meeting.

Guidelines:
- Rewrite as a single cohesive markdown document
- Incorporate new information, update anything that has changed
- Do not remove accurate existing information unless contradicted
- Keep it concise and useful as a quick reference
- Base all claims on evidence from the transcript

Use this structure (omit sections where there is genuinely no information):

# %s

**Last Updated:** %s

## Role & Organisation
(job title, team, company)

## Background & Expertise
(relevant professional background, skills, domain knowledge)

## Working Style
(how they communicate, engage in meetings, areas of focus)

## Meeting History
(bullet list — date · meeting title · one-line note on their role/contribution)

## Key Topics & Interests
(recurring themes, projects, technologies they discuss)

## Notes
(anything else worth remembering)

---

Existing Profile:
%s

---

Recent Meeting: %s (%s)
Other participants for context: %s

Their speaker turns from this meeting:
%s`, name, name, date, existing, meetingTitle, date, allParticipants, speakerTurns)
}

package processor

import (
	"fmt"
	"regexp"
	"strings"

	"skribbl/internal/kb"
)

// ── meeting summary ───────────────────────────────────────────────────────────

func buildSummarySystem(dict string) string {
	base := `You are a professional meeting analyst. Your job is to produce clear, accurate, and useful meeting summaries for a personal knowledge base. Be factual — only include what was actually discussed. Do not fabricate information.

Accuracy rules:
- Preserve uncertainty and intent. Do not turn exploratory language ("could", "maybe", "probably", "I think", "we should consider") into firm plans, decisions, or commitments.
- Decisions require explicit agreement, commitment, or a clearly stated outcome. If an idea was only discussed, put it in Key Discussion Points or Next Steps using tentative wording.
- Action items require a clear owner and requested/accepted follow-up. If ownership is suggested but not confirmed, phrase the action as tentative (for example, "Consider...", "Look into... if picked up") rather than definitive.
- For technical architecture, keep causal relationships and boundaries precise. Do not merge separate systems, pipelines, artifacts, or responsibilities into a single process unless the transcript explicitly does so.
- Attribute opinions and second-hand feedback carefully. If a participant says they "got a vibe" or reports informal feedback, summarize it as that participant's perception, not as an official team position.`
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

// yamlString renders s as a YAML double-quoted scalar. Meeting titles routinely
// contain characters YAML treats specially (":", "|", "#"), so they can never be
// emitted bare.
func yamlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func summaryPrompt(title, date, duration, software string, participants []string, transcript string) string {
	// build YAML frontmatter — generated in Go so it's always valid
	yamlLines := make([]string, len(participants))
	for i, p := range participants {
		yamlLines[i] = fmt.Sprintf("  - \"[[people/%s|%s]]\"", kb.NameSlug(p), p)
	}
	frontmatter := fmt.Sprintf(
		"---\ntitle: %s\ndate: %s\nduration: %s\nsoftware: %s\nparticipants:\n%s\ntags:\n  - meeting\n---",
		yamlString(title), date, duration, software, strings.Join(yamlLines, "\n"),
	)

	// wikilinks for the footer — people/<slug>|Name so Obsidian resolves correctly
	wikilinks := make([]string, len(participants))
	for i, p := range participants {
		wikilinks[i] = fmt.Sprintf("[[people/%s|%s]]", kb.NameSlug(p), p)
	}
	footerLinks := strings.Join(wikilinks, " · ")

	return fmt.Sprintf(`Analyze the following meeting transcript and produce a summary in markdown for an Obsidian knowledge base.

Before writing, distinguish between:
- confirmed decisions/commitments,
- assigned action items,
- tentative proposals or candidate approaches,
- general discussion/background.

Use cautious wording for tentative items. Do not promote suggestions, preferences, or brainstorming into decisions or assigned work.

Output the note using EXACTLY this structure — do not add, remove, or rename any sections:

%s

# %s

> [!abstract] Overview
> (2–3 sentences describing what the meeting was about and its overall outcome — written as a single flowing paragraph inside the callout)

## Key Discussion Points
(bullet points — use **bold** for the topic label followed by a colon and detail)

## Decisions Made
(bullet list of concrete decisions only — require explicit agreement/commitment; if none write exactly: *None recorded.*)

## Action Items

| Owner | Action | Status |
|---|---|---|
(one row per action item — Owner must be a wikilink without an alias/piped display name, e.g. [[people/jade-fisher]], because pipes break markdown tables; Status must be ⬜; include only clear owner + follow-up, and use tentative wording when ownership is suggested but not confirmed; if no action items write exactly: *None recorded.*)

## Next Steps
(bullet list of follow-ups, deadlines, planned activities, or tentative improvements under discussion; if none write exactly: *None recorded.*)

---
%s

---

Transcript:
%s`, frontmatter, title, footerLinks, transcript)
}

var wikilinkAliasPattern = regexp.MustCompile(`\[\[[^\]\n|]+\s*\|\s*[^\]\n]+\]\]`)

// stripTableWikilinkAliases removes aliases from wikilinks inside markdown
// tables. Markdown table cells are pipe-delimited, so aliases like
// [[people/jade-fisher|Jade Fisher]] split the row into extra columns.
func stripTableWikilinkAliases(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}

		lines[i] = wikilinkAliasPattern.ReplaceAllStringFunc(line, func(link string) string {
			inner := strings.TrimSuffix(strings.TrimPrefix(link, "[["), "]]")
			target, _, found := strings.Cut(inner, "|")
			if !found {
				return link
			}
			return "[[" + strings.TrimSpace(target) + "]]"
		})
	}
	return strings.Join(lines, "\n")
}

func normalizeGeneratedSummary(markdown string) string {
	return normalizeActionItemTable(stripTableWikilinkAliases(markdown))
}

func normalizeActionItemTable(markdown string) string {
	lines := strings.Split(markdown, "\n")
	inActionItems := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Action Items" {
			inActionItems = true
			continue
		}
		if inActionItems && strings.HasPrefix(trimmed, "## ") {
			inActionItems = false
		}
		if !inActionItems || !strings.HasPrefix(trimmed, "|") {
			continue
		}

		cells := splitMarkdownTableRow(line)
		if len(cells) < 3 {
			continue
		}

		switch {
		case strings.EqualFold(cells[0], "Owner") && strings.EqualFold(cells[1], "Action") && strings.EqualFold(cells[2], "Status"):
			lines[i] = "| Owner | Action | Status |"
		case isMarkdownSeparatorRow(cells):
			lines[i] = "|---|---|---|"
		case len(cells) > 3:
			owner := cells[0]
			status := cells[len(cells)-1]
			actionCells := cells[1 : len(cells)-1]
			if len(actionCells) > 1 && isWikilink(actionCells[0]) {
				actionCells = actionCells[1:]
			}
			lines[i] = fmt.Sprintf("| %s | %s | %s |", owner, strings.Join(actionCells, " / "), status)
		case len(cells) == 3:
			lines[i] = fmt.Sprintf("| %s | %s | %s |", cells[0], cells[1], cells[2])
		}
	}

	return strings.Join(lines, "\n")
}

func splitMarkdownTableRow(line string) []string {
	rawCells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	cells := make([]string, 0, len(rawCells))
	for _, cell := range rawCells {
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells
}

func isMarkdownSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return true
}

func isWikilink(s string) bool {
	return strings.HasPrefix(s, "[[") && strings.HasSuffix(s, "]]")
}

// ── person profile ────────────────────────────────────────────────────────────

// meetingHistoryBlock returns a Dataview query block for the last 3 months of
// meetings featuring this person. Generated in Go so it is always syntactically
// correct and never hallucinated by the LLM.
func meetingHistoryBlock(name string) string {
	slug := kb.NameSlug(name)
	return fmt.Sprintf(
		"## Meeting History\n\n```dataview\nTABLE WITHOUT ID link(file.link, default(title, file.folder)) AS Meeting, date AS Date, software AS Via\nFROM \"meetings\"\nWHERE file.name = \"summary\"\nAND contains(string(participants), \"people/%s\")\nAND date >= date(today) - dur(3 months)\nSORT date DESC\n```",
		slug,
	)
}

// stripMeetingHistory removes the ## Meeting History section from an existing
// profile before passing it to the LLM, so the LLM never sees or tries to
// preserve an old static table or a Dataview block.
func stripMeetingHistory(profile string) string {
	lines := strings.Split(profile, "\n")
	var out []string
	skip := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## Meeting History") {
			skip = true
			continue
		}
		if skip && strings.HasPrefix(line, "## ") {
			skip = false
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func profilePrompt(name, date, meetingTitle, existingProfile, speakerTurns, allParticipants string) string {
	existing := existingProfile
	if existing == "" {
		existing = "(no existing profile — this is the first recorded meeting with this person)"
	} else {
		existing = stripMeetingHistory(existing)
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
%s`, name, name, date, name, existing, meetingTitle, date, allParticipants, speakerTurns)
}

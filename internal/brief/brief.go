// Package brief renders a SessionStart briefing from a project's current reasoning state in flmnt —
// the read half of the continuity loop. Read-only, stream-scoped, LLM-free. Every read runs through
// the authenticated router GraphQL (memoryEntries/memoryKeyframe), so the router enforces the session
// and the caller's rights to the workspace.
package brief

import (
	"strings"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// Config controls a briefing render. GQL is the authenticated router GraphQL client; ProjectID scopes
// the reads to the caller's workspace.
type Config struct {
	GQL          *apiclient.Client
	ProjectID    string
	MaxDecisions int
	MaxMistakes  int
}

type envelope struct {
	EntryType string `json:"entryType"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
}

const queryEntries = `query($s: ID!, $limit: Int){ memoryEntries(streamId: $s, limit: $limit){ entryType timestamp content } }`
const queryKeyframe = `query($s: ID!){ memoryKeyframe(streamId: $s){ content } }`

// entries reads the newest `limit` entries of a stream via memoryEntries (newest-first). Best-effort:
// returns nil on any error so a missing stream degrades to an empty section.
func (cfg Config) entries(streamID string, limit int) []envelope {
	var out struct {
		MemoryEntries []envelope `json:"memoryEntries"`
	}
	if err := cfg.GQL.Query(queryEntries, map[string]any{"s": streamID, "limit": limit}, &out); err != nil {
		return nil
	}
	return out.MemoryEntries
}

// keyframe reads a stream's latest keyframe content via memoryKeyframe; "" when none exists.
func (cfg Config) keyframe(streamID string) string {
	var out struct {
		MemoryKeyframe *struct {
			Content string `json:"content"`
		} `json:"memoryKeyframe"`
	}
	if err := cfg.GQL.Query(queryKeyframe, map[string]any{"s": streamID}, &out); err != nil || out.MemoryKeyframe == nil {
		return ""
	}
	return out.MemoryKeyframe.Content
}

// Render assembles the briefing: latest keyframe (current understanding) + recent decisions +
// recent/uncorrected mistakes. Returns "" when there's no memory yet (nothing to inject).
func Render(cfg Config) (string, error) {
	if cfg.MaxDecisions == 0 {
		cfg.MaxDecisions = 8
	}
	if cfg.MaxMistakes == 0 {
		cfg.MaxMistakes = 4
	}
	domain := cfg.ProjectID + "::domain"
	mistakes := cfg.ProjectID + "::mistake"

	var b strings.Builder

	domainEntries := cfg.entries(domain, 200)

	// Current state: prefer the project's real curated keyframe; fall back to the latest derived
	// session recap only when there's no real keyframe yet (a fresh repo).
	if kf := cfg.keyframe(domain); strings.TrimSpace(kf) != "" {
		b.WriteString("Current state: " + oneLine(kf) + "\n\n")
	} else if rec := latestContent(domainEntries, "session.recap"); rec != "" {
		b.WriteString("Current state: " + oneLine(rec) + "\n\n")
	}

	// Commits are the primary, curated decision signal (real rationale that landed).
	if chgs := pick(domainEntries, "commit.recorded", 6); len(chgs) > 0 {
		b.WriteString("Recent changes (commits):\n")
		for _, ch := range chgs {
			b.WriteString("- " + oneLine(ch.Content) + "\n")
		}
		b.WriteString("\n")
	}
	if decs := pick(domainEntries, "decision.made", cfg.MaxDecisions); len(decs) > 0 {
		b.WriteString("Recent decisions:\n")
		for _, d := range decs {
			b.WriteString("- " + oneLine(d.Content) + "\n")
		}
		b.WriteString("\n")
	}

	if mis := pick(cfg.entries(mistakes, 60), "decision.mistake", cfg.MaxMistakes); len(mis) > 0 {
		b.WriteString("Recent mistakes (avoid repeating):\n")
		for _, m := range mis {
			b.WriteString("- " + oneLine(m.Content) + "\n")
		}
	}

	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", nil
	}
	return "## Project memory (flmnt)\n\n" + out + "\n", nil
}

func pick(entries []envelope, entryType string, max int) []envelope {
	var out []envelope
	for _, e := range entries {
		if e.EntryType == entryType {
			out = append(out, e)
			if len(out) >= max {
				break
			}
		}
	}
	return out
}

// latestContent returns the content of the most recent entry of entryType, given entries in
// newest-first order; "" if none.
func latestContent(entries []envelope, entryType string) string {
	for _, e := range entries {
		if e.EntryType == entryType {
			return e.Content
		}
	}
	return ""
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

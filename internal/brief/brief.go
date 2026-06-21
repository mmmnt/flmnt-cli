// Package brief renders a SessionStart briefing from a project's current reasoning state in flmnt —
// the read half of the continuity loop. Read-only, stream-scoped, LLM-free.
package brief

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config controls a briefing render against a flmnt endpoint (resolved by the caller — prod by
// default, localhost for devs). AuthHeader is "Bearer <token>" for remote, "" for a local stack.
type Config struct {
	Endpoint     string
	ProjectID    string
	AuthHeader   string
	MaxDecisions int
	MaxMistakes  int
	HTTP         *http.Client
}

type envelope struct {
	EntryType string `json:"entryType"`
	Timestamp string `json:"timestamp"`
	Payload   struct {
		Content string `json:"content"`
		Title   string `json:"title"`
	} `json:"payload"`
}

type keyframeResp struct {
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

var errNotFound = errors.New("not found")

func (cfg Config) client() *http.Client {
	if cfg.HTTP != nil {
		return cfg.HTTP
	}
	return &http.Client{Timeout: 8 * time.Second}
}

func (cfg Config) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+path, nil)
	if err != nil {
		return err
	}
	if cfg.AuthHeader != "" {
		req.Header.Set("Authorization", cfg.AuthHeader)
	}
	resp, err := cfg.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s -> %d: %s", path, resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
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

	var kf keyframeResp
	if err := cfg.get("/streams/"+domain+"/keyframes/latest", &kf); err == nil && strings.TrimSpace(kf.Content) != "" {
		b.WriteString("Current state: " + oneLine(kf.Content) + "\n\n")
	}

	var domainEntries []envelope
	if err := cfg.get("/streams/"+domain+"/entries?limit=200&direction=backwards", &domainEntries); err == nil {
		// Commits are the primary, curated decision signal (real rationale that landed).
		if chgs := pick(domainEntries, "commit.recorded", 6); len(chgs) > 0 {
			b.WriteString("Recent changes (commits):\n")
			for _, ch := range chgs {
				b.WriteString("- " + oneLine(firstNonEmpty(ch.Payload.Title, ch.Payload.Content)) + "\n")
			}
			b.WriteString("\n")
		}
		if decs := pick(domainEntries, "decision.made", cfg.MaxDecisions); len(decs) > 0 {
			b.WriteString("Recent decisions:\n")
			for _, d := range decs {
				b.WriteString("- " + oneLine(firstNonEmpty(d.Payload.Content, d.Payload.Title)) + "\n")
			}
			b.WriteString("\n")
		}
	}

	var mistakeEntries []envelope
	if err := cfg.get("/streams/"+mistakes+"/entries?limit=60&direction=backwards", &mistakeEntries); err == nil {
		if mis := pick(mistakeEntries, "decision.mistake", cfg.MaxMistakes); len(mis) > 0 {
			b.WriteString("Recent mistakes (avoid repeating):\n")
			for _, m := range mis {
				b.WriteString("- " + oneLine(firstNonEmpty(m.Payload.Content, m.Payload.Title)) + "\n")
			}
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

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

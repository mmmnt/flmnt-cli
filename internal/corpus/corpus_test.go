package corpus

import (
	"strings"
	"testing"
)

func TestDocNodeBecomesAFileNamedAfterItsTitle(t *testing.T) {
	r := Render([]Entry{{
		ID:        "doc-1",
		EntryType: "decision.made",
		Content:   "DOC-NODE · THE METHODOLOGY v4.1 (page 3801315) — the planning and process stack.",
	}})

	if len(r.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(r.Files))
	}
	if r.Files[0].Name != "the-methodology.md" {
		t.Errorf("want the-methodology.md, got %q", r.Files[0].Name)
	}
	if r.Files[0].Title != "THE METHODOLOGY v4.1" {
		t.Errorf("want title THE METHODOLOGY v4.1, got %q", r.Files[0].Title)
	}
}

func TestAnEntryWhoseCausationChainReachesADocNodeIsASectionOfThatDocument(t *testing.T) {
	r := Render([]Entry{
		{ID: "doc-1", EntryType: "decision.made", Content: "DOC-NODE · THE METHODOLOGY v4.1 (page 1) — the stack."},
		{ID: "kf", CausationID: "doc-1", EntryType: "keyframe.written", Content: "navigation"},
		{ID: "sec", CausationID: "kf", EntryType: "decision.made", Content: "METHODOLOGY §1a · CORE THESIS. Thesis: precision or halt."},
	})

	if len(r.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(r.Files))
	}
	if !strings.Contains(r.Files[0].Markdown, "CORE THESIS") {
		t.Errorf("section reached through a keyframe is missing:\n%s", r.Files[0].Markdown)
	}
}

func TestASectionsHeadingIsItsFirstSentenceAndItsBodyTheRest(t *testing.T) {
	r := Render([]Entry{
		{ID: "doc-1", EntryType: "decision.made", Content: "DOC-NODE · THE METHODOLOGY v4.1 (page 1) — the stack."},
		{ID: "sec", CausationID: "doc-1", EntryType: "decision.made", Content: "METHODOLOGY §1a · CORE THESIS. Thesis: precision or halt. Corollary: never proceed plausibly."},
	})

	for _, want := range []string{"## METHODOLOGY §1a · CORE THESIS\n", "\nThesis: precision or halt. Corollary: never proceed plausibly.\n"} {
		if !strings.Contains(r.Files[0].Markdown, want) {
			t.Errorf("want %q in\n%q", want, r.Files[0].Markdown)
		}
	}
}

func TestEverySectionIsStampedWithTheEntryItCameFrom(t *testing.T) {
	r := Render([]Entry{
		{ID: "doc-1", EntryType: "decision.made", Content: "DOC-NODE · THE METHODOLOGY v4.1 (page 1) — the stack."},
		{ID: "1768ca87", CausationID: "doc-1", EntryType: "decision.made", Timestamp: "2026-08-05T19:56:10.188Z", Content: "METHODOLOGY §1a · CORE THESIS. Thesis: precision or halt."},
	})

	want := "## METHODOLOGY §1a · CORE THESIS\n`1768ca87` · 2026-08-05T19:56:10.188Z\n\nThesis: precision or halt.\n"
	if !strings.Contains(r.Files[0].Markdown, want) {
		t.Errorf("want\n%q\ngot\n%q", want, r.Files[0].Markdown)
	}
}

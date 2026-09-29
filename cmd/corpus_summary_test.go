package cmd

import (
	"strings"
	"testing"

	"github.com/mmmnt/flmnt-cli/internal/corpus"
)

func TestCorpusSummaryNamesEveryDocumentItsSectionsAndWhatItLeftOut(t *testing.T) {
	r := corpus.Report{
		Files:      []corpus.File{{Name: "the-methodology.md", Sections: 48}, {Name: "rulings.md", Sections: 30}},
		Unrendered: map[string]int{"keyframe.written": 50, "exploration.committed": 4},
	}

	out := corpusSummary("training", r, []string{"architecture.md"})

	for _, want := range []string{
		"training/the-methodology.md",
		"48 sections",
		"training/rulings.md",
		"keyframe.written 50",
		"exploration.committed 4",
		"architecture.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in\n%s", want, out)
		}
	}
}

func TestCorpusSummarySaysNothingAboutStraysWhenThereAreNone(t *testing.T) {
	out := corpusSummary("training", corpus.Report{Files: []corpus.File{{Name: "a.md", Sections: 1}}}, nil)

	if strings.Contains(out, "not generated") {
		t.Errorf("a clean directory must not be reported as having strays; got\n%s", out)
	}
}

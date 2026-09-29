package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/*
`gate` resolved a server URL and never registered --server-url, so `flmnt gate --server-url …` died

	with "unknown flag" — inside a UserPromptSubmit hook, a hard failure. `sync` did the same, while
	its own comment described --server-url as part of its resolution chain. This is the v1.10.3
	--project defect one flag over: the sibling audit CLAUDE.md asks for when one is found.

	Swept rather than listed, and it fails if the sweep matches nothing, so it cannot quietly stop
	covering what it is for.
*/
func TestEveryCommandThatResolvesAServerURLAcceptsTheFlag(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	swept := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		if !strings.Contains(text, "resolveAuthServerURL(cmd") && !strings.Contains(text, "resolveRemoteServerURL(cmd") {
			continue
		}
		swept++
		if !strings.Contains(text, `Flags().String("server-url"`) {
			t.Errorf("%s resolves a server URL but registers no --server-url flag", file)
		}
	}
	if swept == 0 {
		t.Fatal("the sweep matched no source file — it has stopped covering what it is for")
	}
}

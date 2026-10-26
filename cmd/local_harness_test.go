package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runLocalHarness runs `flmnt local <args>` in a checkout holding cfg, with a stub docker on PATH
// that prints the volumes path it was given and its arguments.
func runLocalHarness(t *testing.T, cfg string, args ...string) (string, string, error) {
	t.Helper()
	home, project, bin := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\necho \"$FLMNT_VOLUMES|$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if cfg != "" {
		if err := os.WriteFile(filepath.Join(project, ".flmnt.json"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	var out, errb bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	rootCmd.SetArgs(append([]string{"local"}, args...))
	err := rootCmd.Execute()
	real, _ := filepath.EvalSymlinks(project)
	return strings.ReplaceAll(strings.ReplaceAll(out.String(), real, "<project>"), project, "<project>"), home, err
}

func TestLocalUpStartsTheStackWithTheVolumesPath(t *testing.T) {
	out, home, err := runLocalHarness(t, `{"compose_files":["docker/local.yml"],"env_file":".env.local"}`, "up")
	if err != nil {
		t.Fatalf("local up: %v", err)
	}

	want := filepath.Join(home, ".flmnt", "volumes") + "|compose -f <project>/docker/local.yml --env-file <project>/.env.local up -d\n"
	if out != want {
		t.Fatalf("docker got\n %q\nwant\n %q", out, want)
	}
}

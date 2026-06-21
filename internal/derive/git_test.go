package derive

import "testing"

func TestParseGitLog(t *testing.T) {
	out := gitRS + "abc123" + gitUS + "2026-01-01T00:00:00Z" + gitUS + "feat: add thing" + gitUS + "body line 1\nbody line 2" + gitGS + "\nfile1.go\nfile2.go\n" +
		gitRS + "def456" + gitUS + "2026-01-02T00:00:00Z" + gitUS + `Revert "feat: add thing"` + gitUS + "" + gitGS + "\nfile1.go\n"

	commits := parseGitLog(out)
	if len(commits) != 2 {
		t.Fatalf("want 2 commits, got %d", len(commits))
	}
	if commits[0].SHA != "abc123" || commits[0].Subject != "feat: add thing" {
		t.Errorf("commit0 wrong: %+v", commits[0])
	}
	if commits[0].Body != "body line 1\nbody line 2" {
		t.Errorf("commit0 body=%q", commits[0].Body)
	}
	if len(commits[0].Files) != 2 || commits[0].Files[0] != "file1.go" {
		t.Errorf("commit0 files=%v", commits[0].Files)
	}
	if commits[0].IsRevert() {
		t.Errorf("commit0 should not be a revert")
	}
	if !commits[1].IsRevert() {
		t.Errorf("commit1 should be a revert: %q", commits[1].Subject)
	}
}

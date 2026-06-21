package derive

import (
	"os/exec"
	"strings"
)

// Commit is one git commit correlated to a session — the ground-truth artifact layer.
type Commit struct {
	SHA     string   `json:"sha"`
	Time    string   `json:"time"`
	Subject string   `json:"subject"`
	Body    string   `json:"body,omitempty"`
	Files   []string `json:"files,omitempty"`
}

// IsRevert reports a git revert/reset commit — a factual mistake/supersede signal.
func (c Commit) IsRevert() bool {
	s := strings.ToLower(c.Subject)
	return strings.HasPrefix(s, "revert ") || strings.HasPrefix(s, "revert:")
}

// ASCII separators chosen so they never appear in commit text.
const (
	gitRS = "\x1e" // record separator: starts each commit
	gitUS = "\x1f" // unit separator: between metadata fields
	gitGS = "\x1d" // group separator: ends metadata, before the file list
)

// CommitsInWindow returns commits whose date falls in [from,to], scoped to branch when known
// (else all refs), in repo. from/to are ISO timestamps; either may be empty. Best-effort:
// returns an error the caller can ignore (e.g. branch gone, not a repo) to skip git enrichment.
func CommitsInWindow(repo, branch, from, to string) ([]Commit, error) {
	scope := "--all"
	if branch != "" {
		scope = branch
	}
	args := []string{"-C", repo, "log", scope, "--no-merges", "--date=iso-strict",
		"--pretty=format:" + gitRS + "%H" + gitUS + "%aI" + gitUS + "%s" + gitUS + "%b" + gitGS, "--name-only"}
	if from != "" {
		args = append(args, "--since="+from)
	}
	if to != "" {
		args = append(args, "--until="+to)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, err
	}
	return parseGitLog(string(out)), nil
}

// parseGitLog parses the gitRS/gitUS/gitGS-delimited `git log --name-only` output.
func parseGitLog(out string) []Commit {
	var commits []Commit
	for _, chunk := range strings.Split(out, gitRS) {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		metaFiles := strings.SplitN(chunk, gitGS, 2)
		fields := strings.Split(metaFiles[0], gitUS)
		if len(fields) < 4 {
			continue
		}
		c := Commit{
			SHA:     strings.TrimSpace(fields[0]),
			Time:    strings.TrimSpace(fields[1]),
			Subject: strings.TrimSpace(fields[2]),
			Body:    strings.TrimSpace(fields[3]),
		}
		if len(metaFiles) == 2 {
			for _, f := range strings.Split(strings.TrimSpace(metaFiles[1]), "\n") {
				if f = strings.TrimSpace(f); f != "" {
					c.Files = append(c.Files, f)
				}
			}
		}
		commits = append(commits, c)
	}
	return commits
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

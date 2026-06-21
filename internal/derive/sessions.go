package derive

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ClaudeProjectsDir is ~/.claude/projects — the root of Claude Code's per-project transcripts.
func ClaudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Project is one encoded project directory paired with the real cwd its sessions ran in.
type Project struct {
	Dir string // absolute path to the ~/.claude/projects/<encoded> directory
	Cwd string // real working directory (read from the transcripts, not decoded from the name)
}

// DiscoverProjects lists every project dir under ~/.claude/projects with its true cwd.
func DiscoverProjects() ([]Project, error) {
	root, err := ClaudeProjectsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var projects []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		projects = append(projects, Project{Dir: dir, Cwd: firstCwd(dir)})
	}
	return projects, nil
}

// SessionFiles returns the session .jsonl paths in a project dir, oldest-name first.
func SessionFiles(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// SubagentFiles returns the subagent transcript paths nested under a main session file
// (<dir>/<session-id>/subagents/*.jsonl). Subagents are enrichment for their parent session,
// not standalone sessions — they never get their own keyframe.
func SubagentFiles(sessionFile string) []string {
	base := strings.TrimSuffix(sessionFile, ".jsonl")
	matches, _ := filepath.Glob(filepath.Join(base, "subagents", "*.jsonl"))
	sort.Strings(matches)
	return matches
}

// ProjectForRepo finds the project dir whose sessions ran in repoPath (exact cwd match).
func ProjectForRepo(repoPath string) (Project, bool, error) {
	want := strings.TrimRight(repoPath, "/")
	projects, err := DiscoverProjects()
	if err != nil {
		return Project{}, false, err
	}
	for _, p := range projects {
		if strings.TrimRight(p.Cwd, "/") == want {
			return p, true, nil
		}
	}
	return Project{}, false, nil
}

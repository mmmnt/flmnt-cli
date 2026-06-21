// Package derive turns local Claude Code session transcripts (and, later, git history)
// into structured reasoning memory for flmnt. This file is the transcript reader.
package derive

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
)

// Record is a permissive view of one JSONL line in a Claude Code session transcript.
// Only the fields we extract are typed; everything else is ignored. Unknown/extra
// record shapes parse without error and simply carry empty fields.
type Record struct {
	Type       string          `json:"type"`
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid"`
	Timestamp  string          `json:"timestamp"`
	SessionID  string          `json:"sessionId"`
	Cwd        string          `json:"cwd"`
	GitBranch  string          `json:"gitBranch"`
	IsMeta     bool            `json:"isMeta"`
	Message    json.RawMessage `json:"message"`
	Attachment *Attachment     `json:"attachment"`
	ToolResult *ToolUseResult  `json:"toolUseResult"`
	PRNumber   int             `json:"prNumber"`
	PRURL      string          `json:"prUrl"`
}

// Attachment covers file edits (edited_text_file), file reads (file), and hook output.
type Attachment struct {
	Type     string `json:"type"`
	Filename string `json:"filename"`
}

// ToolUseResult carries the result of a tool execution: a Bash exit code, or the
// type/path of a file-writing tool (create/update).
type ToolUseResult struct {
	ExitCode *int   `json:"exitCode"`
	Type     string `json:"type"`     // "create" | "update" for file-writing tools
	FilePath string `json:"filePath"`
}

// Message is the parsed `message` envelope (role/content), read best-effort.
type Message struct {
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	RawContent json.RawMessage `json:"content"`
}

// Blocks returns message content as blocks, handling BOTH shapes Claude Code emits: a JSON
// array of content blocks, or a plain string (collapsed to a single text block). This is the
// fix for missing user text — much of it is string-shaped, which an array-only parse drops.
func (m Message) Blocks() []ContentBlock {
	c := bytes.TrimSpace(m.RawContent)
	if len(c) == 0 {
		return nil
	}
	if c[0] == '[' {
		var blocks []ContentBlock
		_ = json.Unmarshal(c, &blocks)
		return blocks
	}
	var s string
	if json.Unmarshal(c, &s) == nil && s != "" {
		return []ContentBlock{{Type: "text", Text: s}}
	}
	return nil
}

// ContentBlock is one element of message.content (text, thinking, tool_use, tool_result).
type ContentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`     // tool_use: tool name
	IsError bool            `json:"is_error"` // tool_result
	Input   json.RawMessage `json:"input"`    // tool_use: arguments
}

// ParsedMessage decodes the record's message envelope, tolerating the string-vs-array
// content variants by returning an empty Message on any mismatch.
func (r Record) ParsedMessage() Message {
	var m Message
	if len(r.Message) > 0 {
		_ = json.Unmarshal(r.Message, &m)
	}
	return m
}

// IsToolError reports a failed tool result, checked against BOTH schema shapes: a non-zero
// toolUseResult.exitCode (Bash) and an is_error tool_result content block (Read/Edit/etc.).
func (r Record) IsToolError() bool {
	if r.ToolResult != nil && r.ToolResult.ExitCode != nil && *r.ToolResult.ExitCode != 0 {
		return true
	}
	for _, c := range r.ParsedMessage().Blocks() {
		if c.Type == "tool_result" && c.IsError {
			return true
		}
	}
	return false
}

// EditedFiles returns paths this record changed — from edited_text_file attachments and from
// create/update tool results.
func (r Record) EditedFiles() []string {
	var files []string
	if r.Attachment != nil && r.Attachment.Type == "edited_text_file" && r.Attachment.Filename != "" {
		files = append(files, r.Attachment.Filename)
	}
	if r.ToolResult != nil && (r.ToolResult.Type == "create" || r.ToolResult.Type == "update") && r.ToolResult.FilePath != "" {
		files = append(files, r.ToolResult.FilePath)
	}
	return files
}

// UserText returns the concatenated text of a non-meta user message (decision-candidate source),
// or "" for meta/tool-result/non-user records.
func (r Record) UserText() string {
	if r.Type != "user" || r.IsMeta {
		return ""
	}
	var b []string
	for _, c := range r.ParsedMessage().Blocks() {
		if c.Type == "text" && c.Text != "" {
			b = append(b, c.Text)
		}
	}
	return joinNonEmpty(b, "\n")
}

func joinNonEmpty(parts []string, sep string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += p
	}
	return out
}

const (
	maxLineBytes = 16 * 1024 * 1024 // session lines can be large (full-file attachments)
	startBufKB   = 1024 * 1024
)

// ParseSession reads a session .jsonl into records, tolerating malformed lines.
func ParseSession(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var recs []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, startBufKB), maxLineBytes)
	for sc.Scan() {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) != nil {
			continue // skip malformed lines rather than failing the session
		}
		recs = append(recs, r)
	}
	return recs, sc.Err()
}

// firstCwd returns the working directory a project dir's sessions ran in, read from the
// first record that carries one. Reads line-by-line and stops early (no full parse).
func firstCwd(dir string) string {
	sessions, _ := SessionFiles(dir)
	for _, s := range sessions {
		f, err := os.Open(s)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, startBufKB), maxLineBytes)
		for sc.Scan() {
			var r Record
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue
			}
			if r.Cwd != "" {
				f.Close()
				return r.Cwd
			}
		}
		f.Close()
	}
	return ""
}

// SessionSummary is the read-only inventory of one session (Phase-0 --dry-run output).
type SessionSummary struct {
	Path       string
	SessionID  string
	Cwd        string
	GitBranch  string
	FirstTs    string
	LastTs     string
	Records    int
	ByType     map[string]int
	ToolErrors int // tool_result with non-zero exit code (mistake candidates)
	FileEdits  int // edited_text_file attachments (file-change provenance)
	PRLinks    int // pr-link records (commit/artifact provenance)
	UserMsgs   int // non-meta user messages (decision candidates)
}

// Summarize tallies the high-value signals in a parsed session.
func Summarize(path string, recs []Record) SessionSummary {
	s := SessionSummary{Path: path, ByType: map[string]int{}}
	for _, r := range recs {
		s.Records++
		s.ByType[r.Type]++
		if s.SessionID == "" && r.SessionID != "" {
			s.SessionID = r.SessionID
		}
		if s.Cwd == "" && r.Cwd != "" {
			s.Cwd = r.Cwd
		}
		if s.GitBranch == "" && r.GitBranch != "" {
			s.GitBranch = r.GitBranch
		}
		if r.Timestamp != "" {
			if s.FirstTs == "" {
				s.FirstTs = r.Timestamp
			}
			s.LastTs = r.Timestamp
		}
		if r.IsToolError() {
			s.ToolErrors++
		}
		if len(r.EditedFiles()) > 0 {
			s.FileEdits++
		}
		if r.Type == "pr-link" {
			s.PRLinks++
		}
		if r.Type == "user" && !r.IsMeta {
			s.UserMsgs++
		}
	}
	return s
}

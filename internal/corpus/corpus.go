// Package corpus renders a flmnt doctrine stream to markdown. The structure is derived, never
// declared: a DOC-NODE entry is a document, the entries whose causation chain reaches it are its
// sections, and a `decision.superseded` entry replaces the entry its causationId names. Nothing
// here is hand-maintained, so a doctrine ruling recorded in flmnt cannot drift from the markdown.
package corpus

import (
	"regexp"
	"strings"
)

// Entry is one event in a stream, as memorySlice returns it.
type Entry struct {
	ID          string
	CausationID string
	EntryType   string
	Timestamp   string
	Content     string
}

// File is one rendered document.
type File struct {
	Name     string
	Title    string
	Markdown string
}

// Report is everything a render produced.
type Report struct {
	Files []File
}

const docNodePrefix = "DOC-NODE · "

var (
	trailingVersion = regexp.MustCompile(`\s+v\d+(\.\d+)*$`)
	nonSlug         = regexp.MustCompile(`[^a-z0-9]+`)
)

// Render turns a stream slice into documents.
func Render(entries []Entry) Report {
	byID := make(map[string]Entry, len(entries))
	docs := make(map[string]bool)
	for _, e := range entries {
		byID[e.ID] = e
		if strings.HasPrefix(e.Content, docNodePrefix) {
			docs[e.ID] = true
		}
	}

	var r Report
	fileOf := make(map[string]int)
	bodies := make(map[string]*strings.Builder)
	for _, e := range entries {
		if !docs[e.ID] {
			continue
		}
		title := docTitle(e.Content)
		fileOf[e.ID] = len(r.Files)
		bodies[e.ID] = &strings.Builder{}
		r.Files = append(r.Files, File{Name: slug(title) + ".md", Title: title})
	}

	for _, e := range entries {
		if docs[e.ID] || e.EntryType != "decision.made" {
			continue
		}
		if owner := ownerDoc(e, byID, docs); owner != "" {
			bodies[owner].WriteString(section(e.Content))
		}
	}
	for id, i := range fileOf {
		r.Files[i].Markdown = bodies[id].String()
	}
	return r
}

// ownerDoc walks causationId until it lands on a DOC-NODE; "" when the chain reaches none. Visited
// ids are tracked so a self-referencing or cyclic chain terminates instead of spinning.
func ownerDoc(e Entry, byID map[string]Entry, docs map[string]bool) string {
	seen := map[string]bool{}
	for cur, ok := e, true; ok && !seen[cur.ID]; cur, ok = byID[cur.CausationID] {
		seen[cur.ID] = true
		if docs[cur.ID] {
			return cur.ID
		}
	}
	return ""
}

// section renders one entry as a markdown section: its opening sentence becomes the heading, the
// rest the body. Doctrine entries are authored as "TITLE. prose…", so the split is the author's own
// and not a guess at where a title ends.
func section(content string) string {
	heading, body := content, ""
	if i := strings.Index(content, ". "); i >= 0 {
		heading, body = content[:i], strings.TrimSpace(content[i+2:])
	}
	return "## " + strings.TrimSpace(heading) + "\n\n" + body + "\n"
}

// docTitle is the document's name: what follows the DOC-NODE marker, up to the page citation or the
// subtitle dash — whichever comes first.
func docTitle(content string) string {
	rest := strings.TrimPrefix(content, docNodePrefix)
	for _, cut := range []string{" (page", " — "} {
		if i := strings.Index(rest, cut); i >= 0 {
			rest = rest[:i]
		}
	}
	return strings.TrimSpace(rest)
}

// slug is the file name. A trailing version is dropped so a version bump renames no file.
func slug(title string) string {
	s := trailingVersion.ReplaceAllString(title, "")
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

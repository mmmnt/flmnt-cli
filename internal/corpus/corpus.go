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
	var r Report
	for _, e := range entries {
		if !strings.HasPrefix(e.Content, docNodePrefix) {
			continue
		}
		title := docTitle(e.Content)
		r.Files = append(r.Files, File{Name: slug(title) + ".md", Title: title})
	}
	return r
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

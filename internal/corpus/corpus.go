// Package corpus renders a flmnt doctrine stream to markdown. The structure is derived, never
// declared: a DOC-NODE entry is a document, the entries whose causation chain reaches it are its
// sections, and a `decision.superseded` entry replaces the entry its causationId names. Nothing
// here is hand-maintained, so a doctrine ruling recorded in flmnt cannot drift from the markdown.
package corpus

import (
	"regexp"
	"sort"
	"strconv"
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
	Sections int
}

// Report is everything a render produced. Unrendered counts, by entry type, what the stream held and
// the documents do not — keyframes, explorations, plan snapshots. Reporting it keeps the omission
// visible: a renderer that silently drops entry types is how a corpus goes stale unnoticed.
type Report struct {
	Files      []File
	Unrendered map[string]int
}

const (
	docNodePrefix  = "DOC-NODE · "
	decisionType   = "decision.made"
	rulingsOwner   = ""
	rulingsTitle   = "Rulings recorded outside the document tree"
	supersededType = "decision.superseded"
)

var (
	trailingVersion = regexp.MustCompile(`\s+v\d+(\.\d+)*$`)
	nonSlug         = regexp.MustCompile(`[^a-z0-9]+`)
)

// Render turns a stream slice into documents. Every doctrine entry lands in exactly one place — a
// section or the Superseded appendix — so nothing the stream holds can be lost in the rendering.
//
// A stream with no DOC-NODE renders nothing at all. rulings.md exists to catch doctrine that belongs
// to a corpus and was never wired into it; with no corpus there is nothing for it to supplement, and
// rendering one would turn an ordinary project stream into a single file of every decision it holds.
func Render(streamID string, entries []Entry) Report {
	x := index(entries)

	r := Report{Unrendered: map[string]int{}}
	if len(x.docs) == 0 {
		return r
	}
	fileOf := make(map[string]int)
	bodies := make(map[string]*strings.Builder)
	replaced := make(map[string]*strings.Builder)
	for _, e := range entries {
		if !x.docs[e.ID] {
			continue
		}
		title := docTitle(e.Content)
		fileOf[e.ID] = len(r.Files)
		bodies[e.ID] = &strings.Builder{}
		replaced[e.ID] = &strings.Builder{}
		bodies[e.ID].WriteString(header(streamID, e, title, x))
		r.Files = append(r.Files, File{Name: slug(title) + ".md", Title: title})
	}

	var slots []string
	seen := map[string]bool{}
	for _, e := range entries {
		if x.docs[e.ID] {
			continue
		}
		if !x.renderable(e.ID) {
			r.Unrendered[e.EntryType]++
			continue
		}
		if root := x.root(e).ID; !seen[root] {
			seen[root] = true
			slots = append(slots, root)
		}
	}

	for _, root := range slots {
		owner := x.ownerDoc(x.byID[root])
		if owner == rulingsOwner {
			if _, open := bodies[owner]; !open {
				fileOf[owner] = len(r.Files)
				bodies[owner] = &strings.Builder{}
				replaced[owner] = &strings.Builder{}
				bodies[owner].WriteString(rulingsHeader(streamID, x))
				r.Files = append(r.Files, File{Name: "rulings.md", Title: rulingsTitle})
			}
		}
		for _, id := range x.lineage(root) {
			if by := x.superseders[id]; len(by) > 0 {
				replaced[owner].WriteString(retired(x.byID[id], by))
				continue
			}
			bodies[owner].WriteString(section(x.byID[id]))
			r.Files[fileOf[owner]].Sections++
		}
	}

	for id, i := range fileOf {
		md := bodies[id].String()
		if past := replaced[id].String(); past != "" {
			md += "## Superseded\n\nWhat these entries said before the ruling that replaced them. Kept so a reader\nmeeting the old wording elsewhere can see it was answered, not forgotten.\n\n" + past
		}
		r.Files[i].Markdown = md
	}
	return r
}

// streamIndex answers the three questions rendering asks of a stream: what each id is, what replaced
// it, and where a supersession chain begins.
type streamIndex struct {
	byID        map[string]Entry
	docs        map[string]bool
	pos         map[string]int
	superseders map[string][]string
	// high water is what a reader needs to judge age: the newest entry this render saw, and how many
	// it read. Without it the markdown looks equally current the day it is written and a month later.
	count    int
	newestID string
	newestAt string
}

func index(entries []Entry) streamIndex {
	x := streamIndex{
		byID:        make(map[string]Entry, len(entries)),
		docs:        map[string]bool{},
		pos:         make(map[string]int, len(entries)),
		superseders: map[string][]string{},
	}
	x.count = len(entries)
	for i, e := range entries {
		x.byID[e.ID] = e
		x.pos[e.ID] = i
		if strings.HasPrefix(e.Content, docNodePrefix) {
			x.docs[e.ID] = true
		}
		if e.Timestamp > x.newestAt {
			x.newestAt, x.newestID = e.Timestamp, e.ID
		}
	}
	for _, e := range entries {
		if e.EntryType == supersededType && x.renderable(e.CausationID) {
			x.superseders[e.CausationID] = append(x.superseders[e.CausationID], e.ID)
		}
	}
	return x
}

// renderable marks the entry types that carry doctrine. A DOC-NODE is the document, not a section.
func (x streamIndex) renderable(id string) bool {
	e, ok := x.byID[id]
	return ok && !x.docs[id] && (e.EntryType == decisionType || e.EntryType == supersededType)
}

// root is where a supersession chain starts — the section whose position every later ruling takes. A
// supersession of something that is not doctrine (a keyframe, or an entry outside this stream) roots
// at itself, so it renders in its own right instead of disappearing.
func (x streamIndex) root(e Entry) Entry {
	seen := map[string]bool{}
	for e.EntryType == supersededType && x.renderable(e.CausationID) && !seen[e.ID] {
		seen[e.ID] = true
		e = x.byID[e.CausationID]
	}
	return e
}

// lineage is a root and everything that supersedes it, transitively, in stream order. Two rulings may
// amend the same section — they are branches, not a chain, and both are current.
func (x streamIndex) lineage(root string) []string {
	out := []string{}
	queued := map[string]bool{root: true}
	for stack := []string{root}; len(stack) > 0; {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, id)
		for _, by := range x.superseders[id] {
			if !queued[by] {
				queued[by] = true
				stack = append(stack, by)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return x.pos[out[i]] < x.pos[out[j]] })
	return out
}

// rulingsHeader opens the file for doctrine that never got wired to a DOC-NODE. Dropping those
// entries would be the worst outcome available: they are the newest rulings, and the ones most
// likely to overturn what the seeded documents still say.
func rulingsHeader(streamID string, x streamIndex) string {
	return "# " + rulingsTitle + "\n\n" +
		"> GENERATED by `flmnt corpus` from `" + streamID + "`.\n" +
		"> Do not edit: a correction made here is lost on the next render. Record it in flmnt instead.\n" +
		highWater(x) + "\n" +
		"Decisions in this stream whose causal chain reaches no DOC-NODE. They are doctrine all the\n" +
		"same — usually later rulings — so they render here rather than vanishing.\n\n"
}

// header opens a document: its title, the banner that stops anyone hand-editing it, and the DOC-NODE's
// own prose as the scope statement.
func header(streamID string, doc Entry, title string, x streamIndex) string {
	scope := strings.TrimSpace(docScope(doc.Content))
	return "# " + title + "\n\n" +
		"> GENERATED by `flmnt corpus` from `" + streamID + "`, DOC-NODE `" + doc.ID + "`.\n" +
		"> Do not edit: a correction made here is lost on the next render. Record it in flmnt instead.\n" +
		highWater(x) +
		"\n" + scope + "\n\n"
}

// highWater says what the render saw, so age is visible on the page rather than guessed at. Compare
// the newest entry named here against the stream and you know whether this file is behind.
func highWater(x streamIndex) string {
	return "> Rendered from " + strconv.Itoa(x.count) + " entries; newest `" + x.newestID + "` at " + x.newestAt + ".\n"
}

// docScope is the DOC-NODE prose that follows the title — what the document owns.
func docScope(content string) string {
	rest := strings.TrimPrefix(content, docNodePrefix)
	if i := strings.Index(rest, " — "); i >= 0 {
		return rest[i+len(" — "):]
	}
	return ""
}

// ownerDoc walks causationId until it lands on a DOC-NODE; the rulings file when the chain reaches
// none. Visited ids are tracked so a self-referencing or cyclic chain terminates instead of spinning.
func (x streamIndex) ownerDoc(e Entry) string {
	seen := map[string]bool{}
	for cur, ok := e, true; ok && !seen[cur.ID]; cur, ok = x.byID[cur.CausationID] {
		seen[cur.ID] = true
		if x.docs[cur.ID] {
			return cur.ID
		}
	}
	return rulingsOwner
}

// retired renders one replaced entry into the appendix — its own words, and every id that answered
// them. Two rulings can amend the same section, so this names all of them, not the last one seen.
func retired(e Entry, by []string) string {
	names := make([]string, 0, len(by))
	for _, id := range by {
		names = append(names, "`"+id+"`")
	}
	return "### " + strings.TrimSpace(headingOf(e.Content)) + "\n`" + e.ID + "` · " + e.Timestamp +
		" — replaced by " + strings.Join(names, ", ") + "\n\n" + bodyOf(e.Content) + "\n\n"
}

// section renders one entry as a markdown section: its opening sentence becomes the heading, the
// rest the body. Doctrine entries are authored as "TITLE. prose…", so the split is the author's own
// and not a guess at where a title ends. The stamp is what makes a rendered claim checkable: a reader
// who doubts a line can go read the entry it came from.
func section(e Entry) string {
	return "## " + strings.TrimSpace(headingOf(e.Content)) + "\n`" + e.ID + "` · " + e.Timestamp +
		"\n\n" + bodyOf(e.Content) + "\n\n"
}

func headingOf(content string) string {
	if i := strings.Index(content, ". "); i >= 0 {
		return content[:i]
	}
	return content
}

func bodyOf(content string) string {
	if i := strings.Index(content, ". "); i >= 0 {
		return strings.TrimSpace(content[i+2:])
	}
	return ""
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

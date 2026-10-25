// Package output formats everything the CLI prints. All functions are pure.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/oyasmi/trilium-cli/internal/attrspec"
	"github.com/oyasmi/trilium-cli/internal/etapi"
)

// Time converts an ETAPI utc* timestamp ("2026-10-25 08:35:57.952Z") to
// "YYYY-MM-DD HH:MM" in loc. Unparseable input is returned unchanged.
func Time(utc string, loc *time.Location) string {
	if utc == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05.999Z", utc)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, utc)
		if err != nil {
			return utc
		}
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

// IsSystemID reports whether the ID belongs to Trilium's hidden subtree,
// launchers, options and similar internal notes.
func IsSystemID(id string) bool { return strings.HasPrefix(id, "_") }

// VisibleChildren counts children, ignoring system notes (unless the note
// itself is a system note, where those are its normal content).
func VisibleChildren(n *etapi.Note) int {
	if IsSystemID(n.NoteID) {
		return len(n.ChildNoteIDs)
	}
	c := 0
	for _, id := range n.ChildNoteIDs {
		if !IsSystemID(id) {
			c++
		}
	}
	return c
}

// IsArchived reports whether the note carries an #archived label (owned or inherited).
func IsArchived(n *etapi.Note) bool {
	return slices.ContainsFunc(n.Attributes, func(a etapi.Attribute) bool {
		return a.Type == attrspec.Label && a.Name == "archived"
	})
}

// NoteLine is one list row: id, type, [mime, modified,] title, [+children], [(archived)].
func NoteLine(n *etapi.Note, long bool, loc *time.Location) string {
	f := []string{n.NoteID, n.Type}
	if long {
		f = append(f, dash(n.Mime), dash(Time(n.UTCDateModified, loc)))
	}
	f = append(f, n.Title)
	if c := VisibleChildren(n); c > 0 {
		f = append(f, fmt.Sprintf("+%d", c))
	}
	if IsArchived(n) {
		f = append(f, "(archived)")
	}
	return strings.Join(f, "\t")
}

// TreeLine is a NoteLine indented by two spaces per level below the first.
func TreeLine(n *etapi.Note, depth int, long bool, loc *time.Location) string {
	return strings.Repeat("  ", max(depth-1, 0)) + NoteLine(n, long, loc)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// TypeMime renders "text (text/html)", or just the type when mime is empty.
func TypeMime(typ, mime string) string {
	if mime == "" {
		return typ
	}
	return typ + " (" + mime + ")"
}

// Attr renders an attribute in CLI notation, e.g. #status=done or ~seeAlso=abc.
func Attr(a etapi.Attribute) string {
	return attrspec.Format(a.Type, a.Name, a.Value, a.IsInheritable)
}

// inheritedAttr omits "(inheritable)": every inherited attribute has it.
func inheritedAttr(a etapi.Attribute) string {
	return attrspec.Format(a.Type, a.Name, a.Value, false)
}

// SplitAttrs separates a note's own attributes from inherited ones.
func SplitAttrs(n *etapi.Note) (owned, inherited []etapi.Attribute) {
	for _, a := range n.Attributes {
		if a.NoteID == n.NoteID {
			owned = append(owned, a)
		} else {
			inherited = append(inherited, a)
		}
	}
	return owned, inherited
}

// AttrLine is one row of `attr`: id, notation, and (from owner) when inherited.
func AttrLine(a etapi.Attribute, noteID string) string {
	if a.NoteID != noteID {
		return a.AttributeID + "\t" + inheritedAttr(a) + "\t(from " + a.NoteID + ")"
	}
	return a.AttributeID + "\t" + Attr(a)
}

// Header options for `get`.
type Header struct {
	Note        *etapi.Note
	Attachments int
	AllAttrs    bool
	Content     string // optional "content:" line, e.g. the plain-text view notice
}

// NoteHeader renders the YAML-like metadata block of `get`.
func NoteHeader(h Header, loc *time.Location) string {
	n := h.Note
	var b strings.Builder
	b.WriteString("---\n")
	kv(&b, "id", n.NoteID)
	kv(&b, "title", n.Title)
	kv(&b, "type", TypeMime(n.Type, n.Mime))
	if len(n.ParentNoteIDs) > 0 {
		kv(&b, "parents", strings.Join(n.ParentNoteIDs, ", "))
	}
	owned, inherited := SplitAttrs(n)
	if len(owned) > 0 {
		kv(&b, "attrs", joinAttrs(owned, ""))
	}
	if h.AllAttrs && len(inherited) > 0 {
		kv(&b, "inherited", joinAttrs(inherited, n.NoteID))
	}
	kv(&b, "children", fmt.Sprint(VisibleChildren(n)))
	kv(&b, "attachments", fmt.Sprint(h.Attachments))
	kv(&b, "created", Time(n.UTCDateCreated, loc))
	kv(&b, "modified", Time(n.UTCDateModified, loc))
	if n.IsProtected {
		kv(&b, "protected", "yes (content is not readable through ETAPI)")
	}
	if h.Content != "" {
		kv(&b, "content", h.Content)
	}
	b.WriteString("---\n")
	return b.String()
}

// RevisionHeader renders the metadata block of `rev get`.
func RevisionHeader(r *etapi.Revision, content string, loc *time.Location) string {
	var b strings.Builder
	b.WriteString("---\n")
	kv(&b, "id", r.RevisionID)
	kv(&b, "note", r.NoteID)
	kv(&b, "title", r.Title)
	kv(&b, "type", TypeMime(r.Type, r.Mime))
	kv(&b, "created", Time(r.UTCDateCreated, loc))
	if r.UTCDateLastEdited != "" {
		kv(&b, "edited", Time(r.UTCDateLastEdited, loc))
	}
	if content != "" {
		kv(&b, "content", content)
	}
	b.WriteString("---\n")
	return b.String()
}

func kv(b *strings.Builder, k, v string) {
	b.WriteString(k)
	b.WriteString(": ")
	b.WriteString(v)
	b.WriteByte('\n')
}

// joinAttrs renders attributes space-separated; with self set, each one
// gets "(from <owner>)" (used for inherited attributes).
func joinAttrs(attrs []etapi.Attribute, self string) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		s := Attr(a)
		if self != "" {
			s = inheritedAttr(a) + " (from " + a.NoteID + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// Notice texts used in get headers.
const (
	TextViewNotice = "plain-text view (read-only; not the stored HTML)"
)

// BinaryNotice tells how to save binary content instead of printing it.
func BinaryNotice(mime, saveCmd string) string {
	return fmt.Sprintf("binary (%s), save with: %s", dash(mime), saveCmd)
}

// Q wraps a title in double quotes.
func Q(s string) string { return `"` + s + `"` }

// Ref renders an ID followed by the alias the user typed, if any:
// "e2GMUhHk0e9v (@inbox)".
func Ref(id, alias string) string {
	if alias == "" || alias == id {
		return id
	}
	return id + " (" + alias + ")"
}

// Sizes renders "2431 → 2467 bytes"; before < 0 means unknown ("?").
func Sizes(before, after int) string {
	b := "?"
	if before >= 0 {
		b = fmt.Sprint(before)
	}
	return fmt.Sprintf("%s → %d bytes", b, after)
}

// HumanSize renders a byte count as "80 bytes", "8.9 KB" or "1.2 MB".
func HumanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}

// JSONL writes v as one compact JSON line without HTML escaping.
func JSONL(w io.Writer, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

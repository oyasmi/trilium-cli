package output

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oyasmi/trilium-cli/internal/attrspec"
	"github.com/oyasmi/trilium-cli/internal/config"
	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/ref"
	"github.com/oyasmi/trilium-cli/internal/testutil"
)

var loc = time.FixedZone("UTC+8", 8*3600)

func sampleNote() *etapi.Note {
	return &etapi.Note{
		NoteID: "tText01", Title: "会议纪要", Type: "text", Mime: "text/html",
		UTCDateCreated: "2026-10-25 08:55:01.123Z", UTCDateModified: "2026-10-25 08:57:59.999Z",
		ParentNoteIDs: []string{"etapiSandbox01", "xYz12345"},
		ChildNoteIDs:  []string{"c1", "c2", "c3"},
		Attributes: []etapi.Attribute{
			{AttributeID: "a1", NoteID: "tText01", Type: "label", Name: "status", Value: "published"},
			{AttributeID: "a2", NoteID: "tText01", Type: "label", Name: "todo"},
			{AttributeID: "a3", NoteID: "tText01", Type: "relation", Name: "seeAlso", Value: "tCodePy01"},
			{AttributeID: "a4", NoteID: "etapiSandbox01", Type: "label", Name: "project", Value: "etapi", IsInheritable: true},
		},
	}
}

func TestLines(t *testing.T) {
	n := sampleNote()
	bare := &etapi.Note{NoteID: "bk1", Type: "book", Title: "Folder", UTCDateModified: "2026-01-01 00:00:00.000Z",
		Attributes: []etapi.Attribute{{NoteID: "p", Type: "label", Name: "archived", IsInheritable: true}}}
	root := &etapi.Note{NoteID: "root", Type: "text", Title: "root", ChildNoteIDs: []string{"_hidden", "a"}}
	hidden := &etapi.Note{NoteID: "_hidden", Type: "doc", Title: "Hidden", ChildNoteIDs: []string{"_options", "_help"}}
	var b strings.Builder
	for _, x := range []*etapi.Note{n, bare, root, hidden} {
		b.WriteString(NoteLine(x, false, loc) + "\n")
		b.WriteString(NoteLine(x, true, loc) + "\n")
	}
	b.WriteString(TreeLine(n, 1, false, loc) + "\n")
	b.WriteString(TreeLine(bare, 3, false, loc) + "\n")
	for _, a := range n.Attributes {
		b.WriteString(AttrLine(a, n.NoteID) + "\n")
	}
	testutil.Golden(t, "lines", b.String())
}

func TestHeaders(t *testing.T) {
	n := sampleNote()
	var b strings.Builder
	b.WriteString(NoteHeader(Header{Note: n, Attachments: 1}, loc))
	b.WriteString(NoteHeader(Header{Note: n, Attachments: 0, AllAttrs: true, Content: TextViewNotice}, loc))
	img := &etapi.Note{NoteID: "img1", Title: "red.png", Type: "image", Mime: "image/png", IsProtected: true, ParentNoteIDs: []string{"root"}}
	b.WriteString(NoteHeader(Header{Note: img, Content: BinaryNotice(img.Mime, "trilium get img1 -o <file>")}, loc))
	r := &etapi.Revision{RevisionID: "R1a2b3c4", NoteID: "tText01", Title: "旧标题", Type: "text", Mime: "text/html",
		UTCDateCreated: "2026-10-25 08:00:00.000Z", UTCDateLastEdited: "2026-10-25 07:59:00.000Z"}
	b.WriteString(RevisionHeader(r, "", loc))
	b.WriteString(RevisionHeader(&etapi.Revision{RevisionID: "R2", NoteID: "n", Type: "book"}, TextViewNotice, loc))
	testutil.Golden(t, "headers", b.String())
}

func TestTime(t *testing.T) {
	cases := map[string]string{
		"2026-10-25 08:35:57.952Z": "2026-10-25 16:35",
		"2026-12-31 20:00:00.000Z": "2027-01-01 04:00",
		"2026-10-25T08:35:57Z":     "2026-10-25 16:35",
		"":                         "",
		"garbage":                  "garbage",
	}
	for in, want := range cases {
		if got := Time(in, loc); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

func TestSmallHelpers(t *testing.T) {
	if Ref("e2GM", "@inbox") != "e2GM (@inbox)" || Ref("x", "") != "x" || Ref("x", "x") != "x" {
		t.Error("Ref")
	}
	if Sizes(2431, 2467) != "2431 → 2467 bytes" || Sizes(-1, 80) != "? → 80 bytes" {
		t.Error("Sizes")
	}
	for n, want := range map[int64]string{80: "80 bytes", 9113: "8.9 KB", 1258291: "1.2 MB", 3 << 30: "3.0 GB"} {
		if HumanSize(n) != want {
			t.Errorf("HumanSize(%d)=%s", n, HumanSize(n))
		}
	}
	if Q("a") != `"a"` || TypeMime("book", "") != "book" || !IsSystemID("_x") {
		t.Error("helpers")
	}
	var b strings.Builder
	if err := JSONL(&b, map[string]string{"content": "<p>a&b</p>"}); err != nil || b.String() != `{"content":"<p>a&b</p>"}`+"\n" {
		t.Errorf("JSONL %q", b.String())
	}
	if JSONL(&b, func() {}) == nil {
		t.Error("JSONL must fail on functions")
	}
}

func TestErrors(t *testing.T) {
	_, attrErr := attrspec.Parse("bad")
	_, refErr := ref.Parse("@nope")
	cases := []error{
		&etapi.Error{Status: 404, Code: "NOTE_NOT_FOUND", Message: "Note 'abc' not found."},
		&etapi.Error{Status: 404, Code: "ATTACHMENT_NOT_FOUND", Message: "Attachment 'x' not found."},
		&etapi.Error{Status: 401, Code: "NOT_AUTHENTICATED", Message: "Not authenticated"},
		&etapi.Error{Status: 400, Code: "NOTE_IS_PROTECTED", Message: "Note 'p' is protected"},
		&etapi.Error{Status: 429, Message: "HTTP 429 from server"},
		&etapi.Error{Status: 502, Message: "HTTP 502 from server"},
		&etapi.NetworkError{URL: "https://notes.example.com/etapi", Err: errors.New("dial tcp: connection refused")},
		&config.Error{Msg: "TRILIUM_URL is not set.", Hint: "Set it"},
		refErr,
		attrErr,
		content.ErrTooManySources,
		Usage("see: trilium help get", "-c and -m can't be used together"),
		Fail("", "no match for --old #1"),
		NotFound(HintSearch, "nothing"),
		fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
		errors.New("plain"),
	}
	var b strings.Builder
	for _, e := range cases {
		s, code := ErrorText(e)
		fmt.Fprintf(&b, "[%d] %s", code, s)
		s, code = ErrorJSON(e)
		fmt.Fprintf(&b, "[%d] %s", code, s)
	}
	testutil.Golden(t, "errors", b.String())
}

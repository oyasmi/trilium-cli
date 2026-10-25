package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
	"github.com/oyasmi/trilium-cli/internal/ref"
)

// target is a resolved note reference.
type target struct {
	ID    string
	Alias string // the @shortcut typed by the user, "" for plain IDs
}

// String renders "id" or "id (@alias)" for result lines.
func (t target) String() string { return output.Ref(t.ID, t.Alias) }

// parseRefs validates references locally, before any request is made.
func parseRefs(args ...string) ([]ref.Ref, error) {
	out := make([]ref.Ref, 0, len(args))
	for _, a := range args {
		r, err := ref.Parse(a)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (rt *Runtime) resolveRef(ctx context.Context, c *etapi.Client, r ref.Ref) (target, error) {
	id, err := r.Resolve(ctx, c, rt.now())
	if err != nil {
		return target{}, err
	}
	t := target{ID: id}
	if r.IsAlias() {
		t.Alias = r.Raw
	}
	return t, nil
}

// resolve parses and resolves one note reference.
func (rt *Runtime) resolve(ctx context.Context, c *etapi.Client, s string) (target, error) {
	r, err := ref.Parse(s)
	if err != nil {
		return target{}, err
	}
	return rt.resolveRef(ctx, c, r)
}

// setup returns the client after validating references; refs are resolved in order.
func (rt *Runtime) setup(ctx context.Context, refs ...string) (*etapi.Client, []target, error) {
	parsed, err := parseRefs(refs...)
	if err != nil {
		return nil, nil, err
	}
	c, err := rt.Client()
	if err != nil {
		return nil, nil, err
	}
	ts := make([]target, len(parsed))
	for i, r := range parsed {
		if ts[i], err = rt.resolveRef(ctx, c, r); err != nil {
			return nil, nil, err
		}
	}
	return c, ts, nil
}

// guardWrite refuses content/metadata changes to system notes without --force.
func guardWrite(id string, force bool) error {
	if output.IsSystemID(id) && !force {
		return output.Usage("add --force if you really mean to change it", "%s is a Trilium system note", id)
	}
	return nil
}

// guardStructure refuses to delete or move root and system notes.
func guardStructure(id, verb string) error {
	if id == "root" || output.IsSystemID(id) {
		return output.Usage("", "refusing to %s %s: it is a Trilium system note", verb, id)
	}
	return nil
}

const contentHint = `give the content as an argument, with -f FILE, or with -f - (stdin), e.g.:
      trilium append <note> "<p>text</p>"
      trilium append <note> -f body.html`

// readContent reads the command's content from its argument, -f FILE or -f -.
func (rt *Runtime) readContent(args []string, argIndex int, file string, required bool) ([]byte, content.Source, error) {
	in := content.Input{File: file}
	if len(args) > argIndex {
		in.Arg, in.HasArg = args[argIndex], true
	}
	b, src, err := content.Read(in, rt.Stdin)
	if err != nil {
		if errors.Is(err, content.ErrTooManySources) {
			return nil, src, err
		}
		return nil, src, output.Usage("", "can't read content: %v", err)
	}
	if required && src == content.None {
		return nil, src, output.Usage(contentHint, "no content given")
	}
	return b, src, nil
}

// warnNoTags warns when content written to a text note has no HTML tags.
func (rt *Runtime) warnNoTags(noteType string, b []byte) {
	if noteType == content.TypeText && len(strings.TrimSpace(string(b))) > 0 && !content.HasHTMLTags(string(b)) {
		rt.warn("text notes store HTML; this content has no tags and will render as one paragraph. Wrap it in <p>…</p>, or create a code note (-t code) for plain text.")
	}
}

// isBinaryNote reports whether a note holds binary content.
func isBinaryNote(n *etapi.Note) bool { return content.BinaryType(n.Type, n.Mime) }

// sendBinary reports whether content must be uploaded as octet-stream.
func sendBinary(typ, mime string, b []byte) bool { return content.IsBinary(typ, mime, b) }

func isNotFound(err error) bool {
	var ae *etapi.Error
	return errors.As(err, &ae) && (ae.NotFound() || ae.Status == http.StatusNotFound)
}

// findBranch locates the branch placing note under parent. Branch IDs are
// usually "<parent>_<note>"; otherwise the note's branches are searched.
func findBranch(ctx context.Context, c *etapi.Client, noteID, parentID string) (*etapi.Branch, error) {
	b, err := c.GetBranch(ctx, parentID+"_"+noteID)
	if err == nil && b.NoteID == noteID && b.ParentNoteID == parentID {
		return b, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	n, err := c.GetNote(ctx, noteID)
	if err != nil {
		return nil, err
	}
	for _, id := range n.ParentBranchIDs {
		b, err := c.GetBranch(ctx, id)
		if err != nil {
			return nil, err
		}
		if b.ParentNoteID == parentID {
			return b, nil
		}
	}
	return nil, output.NotFound("list its placements with: trilium branch ls "+noteID, "%s is not placed under %s", noteID, parentID)
}

// checkCycle refuses to place note under newParent when newParent is the
// note itself or one of its descendants. The server does not check this.
func checkCycle(ctx context.Context, c *etapi.Client, noteID, newParent string) error {
	if noteID == newParent {
		return output.Fail("", "can't place %s under itself", noteID)
	}
	res, err := c.Search(ctx, etapi.SearchParams{
		Search:               fmt.Sprintf("note.noteId = %q", newParent),
		AncestorNoteID:       noteID,
		IncludeArchivedNotes: true,
	})
	if err != nil {
		return err
	}
	for _, n := range res {
		if n.NoteID == newParent {
			return output.Fail("", "can't place %s under %s: %s is inside %s, which would create a loop", noteID, newParent, newParent, noteID)
		}
	}
	return nil
}

// writeOut saves data to a file, or to stdout for "-".
func (rt *Runtime) writeOut(path string, data []byte) error {
	if path == "-" {
		_, err := rt.Stdout.Write(data)
		return err
	}
	if err := rt.WriteFile(path, data); err != nil {
		return output.Fail("", "can't write %s: %v", path, err)
	}
	return nil
}

func intPtr(cmd *cobra.Command, name string, v int) *int {
	if !changed(cmd, name) {
		return nil
	}
	return &v
}

func strPtr(cmd *cobra.Command, name string, v string) *string {
	if !changed(cmd, name) {
		return nil
	}
	return &v
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

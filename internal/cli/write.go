package cli

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

// contentResult is the --json output of content-changing commands.
type contentResult struct {
	NoteID       string `json:"noteId"`
	BytesBefore  *int   `json:"bytesBefore"`
	BytesAfter   int    `json:"bytesAfter"`
	Replacements *int   `json:"replacements,omitempty"`
}

const maxRetries = 3

// change computes new content from the current one. It also returns kept,
// which tells whether stored content that differs from what was written
// still contains this change (another writer changed the note right after
// us); nil means only the exact bytes count.
type change func(n *etapi.Note, old []byte) (updated []byte, kept func(stored []byte) bool, err error)

// rewrite applies fn to a note's content with optimistic concurrency: the
// blobId is compared right before writing and the stored content is read
// back afterwards. ETAPI has no conditional write, so this narrows the
// window for lost updates but can't close it: two writers can both pass the
// check and the later write wins without either of them noticing.
//
// With redo set (append), a write whose change was overwritten is redone.
// Without it (edit), that case is reported as an unknown outcome instead,
// because the edit may no longer apply to the newer content.
func rewrite(ctx context.Context, c *etapi.Client, id string, fn change, redo bool) (before, after int, err error) {
	wrote := false
	verify := "check the note with: trilium get -c " + id
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Random backoff so that concurrent writers fall out of step.
			select {
			case <-ctx.Done():
				return 0, 0, ctx.Err()
			case <-time.After(time.Duration(50+rand.IntN(250)) * time.Millisecond * time.Duration(attempt)):
			}
		}
		n, err := c.GetNote(ctx, id)
		if err != nil {
			return 0, 0, afterWrite(err, wrote, verify)
		}
		if isBinaryNote(n) {
			return 0, 0, output.Fail("replace it with: trilium put "+id+" -f <file>", "%s has binary content (%s) and can't be edited as text", id, n.Mime)
		}
		old, err := c.GetContent(ctx, id)
		if err != nil {
			return 0, 0, afterWrite(err, wrote, verify)
		}
		updated, kept, err := fn(n, old)
		if err != nil {
			return 0, 0, afterWrite(err, wrote, verify)
		}
		latest, err := c.GetNote(ctx, id)
		if err != nil {
			return 0, 0, afterWrite(err, wrote, verify)
		}
		if latest.BlobID != n.BlobID {
			continue
		}
		wrote = true
		if err := c.PutContent(ctx, id, bytes.NewReader(updated), sendBinary(n.Type, n.Mime, updated)); err != nil {
			return 0, 0, afterWrite(err, true, verify)
		}
		stored, err := c.GetContent(ctx, id)
		if err != nil {
			return 0, 0, output.Fail(verify, "%s was written, but reading it back failed: %v", id, err)
		}
		if bytes.Equal(stored, updated) || (kept != nil && kept(stored)) {
			return len(old), len(updated), nil
		}
		if !redo {
			return 0, 0, output.Fail(verify, "%s was written, but another writer changed it right after; the result is unknown", id)
		}
	}
	if wrote {
		return 0, 0, output.Fail(verify, "%s kept changing while writing (another writer?); your change may be missing", id)
	}
	return 0, 0, output.Fail("", "note changed concurrently, please retry (nothing was changed)")
}

// afterWrite makes sure an error that follows a write doesn't read as if
// nothing had happened.
func afterWrite(err error, wrote bool, verify string) error {
	if !wrote {
		return err
	}
	msg, _, code := output.Describe(err)
	msg = strings.Replace(msg, " (nothing was changed)", "", 1)
	return &output.Error{Msg: msg + " (a write was already attempted, so the outcome is unknown)", Hint: verify, Exit: code}
}

func appendCmd(rt *Runtime) *cobra.Command {
	var (
		file       string
		top, force bool
	)
	cmd := &cobra.Command{
		Use:   "append <note> [content]",
		Short: "Add content to the end of a note",
		Long: `Add content to the end of a note (or the start with --top).
Text notes (HTML) are joined byte for byte. For other notes (code etc.) a
newline is inserted when needed so that lines don't run together.`,
		Example: `  trilium append @today "<p>Called Alice about the budget.</p>"
  trilium append <id> -f section.html
  trilium append <code-note> -f - --top < header.txt`,
		Args: rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			piece, _, err := rt.readContent(args, 1, file, true)
			if err != nil {
				return err
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			t := ts[0]
			if err := guardWrite(t.ID, force); err != nil {
				return err
			}
			warned := false
			before, after, err := rewrite(cmd.Context(), c, t.ID, func(n *etapi.Note, old []byte) ([]byte, func([]byte) bool, error) {
				if !warned {
					rt.warnNoTags(n.Type, piece)
					warned = true
				}
				kept := func(stored []byte) bool { return len(piece) == 0 || bytes.Contains(stored, piece) }
				return joinContent(n.Type, old, piece, top), kept, nil
			}, true)
			if err != nil {
				return err
			}
			b := before
			return rt.emit(contentResult{NoteID: t.ID, BytesBefore: &b, BytesAfter: after},
				fmt.Sprintf("appended %s %+d bytes: %s", t, after-before, output.Sizes(before, after)))
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "read content from FILE (- for stdin)")
	cmd.Flags().BoolVar(&top, "top", false, "add to the start instead of the end")
	cmd.Flags().BoolVar(&force, "force", false, "allow changing Trilium system notes")
	return cmd
}

// joinContent appends (or prepends) piece. Non-HTML notes get a newline
// between the parts when the first one doesn't end with one.
func joinContent(noteType string, old, piece []byte, top bool) []byte {
	first, second := old, piece
	if top {
		first, second = piece, old
	}
	out := make([]byte, 0, len(old)+len(piece)+1)
	out = append(out, first...)
	if noteType != content.TypeText && len(first) > 0 && len(second) > 0 && first[len(first)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, second...)
}

func putCmd(rt *Runtime) *cobra.Command {
	var (
		file  string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "put <note> [content]",
		Short: "Replace a note's whole content",
		Long: `Replace a note's whole content. No revision is saved automatically; run
"trilium rev add <note>" first if you want one. Image and file notes accept
new binary content with -f.`,
		Example: `  trilium put <id> -f page.html
  trilium put <image-note> -f new.png`,
		Args: rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, _, err := rt.readContent(args, 1, file, true)
			if err != nil {
				return err
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			t := ts[0]
			if err := guardWrite(t.ID, force); err != nil {
				return err
			}
			ctx := cmd.Context()
			n, err := c.GetNote(ctx, t.ID)
			if err != nil {
				return err
			}
			before := -1
			if !isBinaryNote(n) {
				old, err := c.GetContent(ctx, t.ID)
				if err != nil {
					return err
				}
				before = len(old)
				rt.warnNoTags(n.Type, body)
			}
			if err := c.PutContent(ctx, t.ID, bytes.NewReader(body), sendBinary(n.Type, n.Mime, body)); err != nil {
				return err
			}
			res := contentResult{NoteID: t.ID, BytesAfter: len(body)}
			if before >= 0 {
				res.BytesBefore = &before
			}
			return rt.emit(res, fmt.Sprintf("updated %s content: %s", t, output.Sizes(before, len(body))))
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "read content from FILE (- for stdin)")
	cmd.Flags().BoolVar(&force, "force", false, "allow changing Trilium system notes")
	return cmd
}

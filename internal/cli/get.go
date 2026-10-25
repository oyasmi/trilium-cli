package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

type getOpts struct {
	contentOnly, metaOnly, text, allAttrs bool
	out                                   string
}

// getResult is what one `get` fetches for one note.
type getResult struct {
	note        *etapi.Note
	content     []byte
	hasContent  bool
	attachments []etapi.Attachment
	err         error
}

// getJSON is the --json shape: the ETAPI note plus content and attachments.
type getJSON struct {
	etapi.Note
	Content     *string            `json:"content,omitempty"`
	ContentView string             `json:"contentView,omitempty"`
	Attachments []etapi.Attachment `json:"attachments"`
}

func getCmd(rt *Runtime) *cobra.Command {
	var o getOpts
	cmd := &cobra.Command{
		Use:   "get <note>...",
		Short: "Show a note's details and content",
		Long: `Show each note's metadata header followed by its raw content (HTML for text
notes). Binary content is never printed; save it with -o FILE.
--text shows a read-only plain-text view of text notes: don't use it for edit --old.`,
		Example: `  trilium get <id1> <id2>
  trilium get <id> -c            # raw content only
  trilium get <id> -o photo.png  # save content to a file`,
		Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.contentOnly && o.metaOnly {
				return output.Usage("see: trilium help get", "-c and -m can't be used together")
			}
			if o.out != "" && len(args) != 1 {
				return output.Usage("", "-o works with exactly one note")
			}
			if o.out != "" && (o.metaOnly || o.text) {
				return output.Usage("", "-o saves the raw content; it can't be combined with -m or --text")
			}
			c, ts, err := rt.setup(cmd.Context(), args...)
			if err != nil {
				return err
			}
			results := fetchNotes(cmd.Context(), c, ts, o)
			for i, r := range results {
				if r.err != nil {
					return r.err
				}
				if o.out != "" {
					return saveContent(rt, ts[i].ID, o.out, r)
				}
				if i > 0 && !rt.JSON {
					rt.println("")
				}
				if err := printNote(rt, r, o); err != nil {
					return err
				}
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&o.contentOnly, "content", "c", false, "print only the raw content")
	f.BoolVarP(&o.metaOnly, "meta", "m", false, "print only the metadata header")
	f.BoolVar(&o.text, "text", false, "plain-text view of text notes (read-only)")
	f.BoolVar(&o.allAttrs, "all-attrs", false, "also list inherited attributes")
	f.StringVarP(&o.out, "output", "o", "", "save the raw content to FILE (- for stdout)")
	return cmd
}

// fetchNotes loads all notes concurrently: the note and its attachments in
// parallel, then the content unless it is binary (or -o asks for it anyway).
func fetchNotes(ctx context.Context, c *etapi.Client, ts []target, o getOpts) []getResult {
	results := make([]getResult, len(ts))
	var wg sync.WaitGroup
	for i, t := range ts {
		wg.Go(func() {
			r := &results[i]
			var inner sync.WaitGroup
			var attErr error
			if !o.contentOnly && o.out == "" {
				inner.Go(func() { r.attachments, attErr = c.ListAttachments(ctx, t.ID) })
			}
			r.note, r.err = c.GetNote(ctx, t.ID)
			if r.err == nil && !o.metaOnly && (o.out != "" || !r.note.IsProtected && !isBinaryNote(r.note)) {
				r.content, r.err = c.GetContent(ctx, t.ID)
				r.hasContent = r.err == nil
			}
			inner.Wait()
			if r.err == nil {
				r.err = attErr
			}
		})
	}
	wg.Wait()
	return results
}

func saveContent(rt *Runtime, id, path string, r getResult) error {
	if err := rt.writeOut(path, r.content); err != nil {
		return err
	}
	if path != "-" {
		if rt.JSON {
			return output.JSONL(rt.Stdout, map[string]any{"noteId": id, "file": path, "bytes": len(r.content)})
		}
		rt.printf("saved %s to %s (%d bytes)\n", id, path, len(r.content))
	}
	return nil
}

func printNote(rt *Runtime, r getResult, o getOpts) error {
	n := r.note
	binary := isBinaryNote(n) || (r.hasContent && content.IsBinary(n.Type, n.Mime, r.content))
	body := r.content
	notice := ""
	if n.IsProtected && (o.contentOnly || o.text) {
		return output.Fail("protected notes can't be read or changed through ETAPI", "%s is protected", n.NoteID)
	}
	switch {
	case binary:
		if o.contentOnly || o.text {
			return output.Fail("save it with: trilium get "+n.NoteID+" -o <file>",
				"%s has binary content (%s) and can't be printed", n.NoteID, n.Mime)
		}
		notice = output.BinaryNotice(n.Mime, "trilium get "+n.NoteID+" -o <file>")
		body = nil
	case o.text && n.Type == content.TypeText && r.hasContent:
		body = []byte(content.PlainText(string(body)))
		notice = output.TextViewNotice
	}

	if rt.JSON {
		v := getJSON{Note: *n, Attachments: r.attachments}
		if v.Attachments == nil {
			v.Attachments = []etapi.Attachment{}
		}
		if r.hasContent && !binary {
			s := string(body)
			v.Content = &s
			if notice == output.TextViewNotice {
				v.ContentView = "text"
			}
		}
		return output.JSONL(rt.Stdout, v)
	}

	if !o.contentOnly {
		fmt.Fprint(rt.Stdout, output.NoteHeader(output.Header{
			Note: n, Attachments: len(r.attachments), AllAttrs: o.allAttrs, Content: notice,
		}, rt.Loc))
	}
	if o.metaOnly || body == nil {
		return nil
	}
	if _, err := rt.Stdout.Write(body); err != nil {
		return err
	}
	// Keep the terminal tidy after the header; -c prints the bytes exactly.
	if !o.contentOnly && len(body) > 0 && !strings.HasSuffix(string(body), "\n") {
		rt.println("")
	}
	return nil
}

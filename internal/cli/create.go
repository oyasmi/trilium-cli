package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/attrspec"
	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
	"github.com/oyasmi/trilium-cli/internal/ref"
)

func createCmd(rt *Runtime) *cobra.Command {
	var (
		parent, typ, mime, file, id string
		attrs                       []string
		pos                         int
	)
	cmd := &cobra.Command{
		Use:   "create <title> [content]",
		Short: "Create a note (default parent: @inbox)",
		Long: `Create a note. Text notes store HTML. With -f FILE the type, mime and title
are inferred from the file (e.g. .py → code note); explicit flags win.
-t search stores the content as the saved search query (#searchString).
If TRILIUM_AGENT is set, the note gets #agent=<name>.`,
		Example: `  trilium create "Meeting notes" "<p>Agenda…</p>" -p <parent>
  trilium create -f script.py -p <parent> -a '#status=draft'
  trilium create "Todo" -t search '#status=todo'`,
		Args: rangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" && len(args) == 0 {
				return output.Usage("usage: trilium create <title> [content]", "a title is required (or use -f FILE)")
			}
			title, contentArgs := "", args
			switch {
			case file != "" && len(args) == 2:
				return content.ErrTooManySources
			case len(args) > 0:
				title, contentArgs = args[0], args[1:]
			}
			if file != "" && file != "-" && title == "" {
				title = filepath.Base(file)
			}
			if title == "" {
				return output.Usage("", "a title is required when reading content from stdin")
			}
			body, _, err := rt.readContent(contentArgs, 0, file, false)
			if err != nil {
				return err
			}
			ops, err := attrspec.ParseAll(attrs)
			if err != nil {
				return err
			}
			for _, op := range ops {
				if op.Kind == attrspec.Remove {
					return output.Usage("", "-a %s: a new note has no attributes to remove", op)
				}
			}
			if rt.Getenv("TRILIUM_AGENT") != "" {
				ops = append(ops, attrspec.Op{Kind: attrspec.Add, Type: attrspec.Label, Name: "agent", Value: strings.TrimSpace(rt.Getenv("TRILIUM_AGENT")), HasValue: true})
			}
			if err := validateRelationTargets(ops); err != nil {
				return err
			}
			if id != "" && !ref.ValidID(id) {
				return output.Usage("", "--id must be 4-32 letters, digits or _")
			}

			typ, mime = inferTypeMime(cmd, typ, mime, file, title, body)
			var searchQuery string
			if typ == "search" {
				searchQuery, body = string(body), nil
			}
			binary := sendBinary(typ, mime, body)

			c, ts, err := rt.setup(cmd.Context(), parent)
			if err != nil {
				return err
			}
			p := ts[0]
			if !binary {
				rt.warnNoTags(typ, body)
			}
			def := etapi.CreateNoteDef{ParentNoteID: p.ID, Title: title, Type: typ, Mime: mime, NoteID: id, NotePosition: intPtr(cmd, "pos", pos)}
			if !binary {
				def.Content = string(body)
			}
			ctx := cmd.Context()
			res, err := c.CreateNote(ctx, def)
			if err != nil {
				return err
			}
			n := &res.Note
			if binary {
				if err := c.PutContent(ctx, n.NoteID, bytes.NewReader(body), true); err != nil {
					return output.Fail("the note exists but is empty; retry with: trilium put "+n.NoteID+" -f <file>", "created %s but uploading its content failed: %v", n.NoteID, err)
				}
			}
			if typ == "search" && searchQuery != "" {
				ops = append([]attrspec.Op{{Kind: attrspec.Add, Type: attrspec.Label, Name: "searchString", Value: searchQuery, HasValue: true}}, ops...)
			}
			created, err := rt.createAttrs(ctx, c, n.NoteID, ops)
			n.Attributes = append(n.Attributes, created...)
			if err != nil {
				return output.Fail("", "created %s, but adding attributes failed: %v", n.NoteID, err)
			}
			return rt.emit(n, "created "+n.NoteID+" "+n.Type+" "+output.Q(n.Title)+" in "+p.String())
		},
	}
	f := cmd.Flags()
	f.StringVarP(&parent, "parent", "p", "@inbox", "parent note")
	f.StringVarP(&typ, "type", "t", "text", "note type: text, code, mermaid, search, book, file, image, …")
	f.StringVar(&mime, "mime", "", "mime type (e.g. text/x-python for code notes)")
	f.StringVarP(&file, "file", "f", "", "read content from FILE (- for stdin)")
	f.StringArrayVarP(&attrs, "attr", "a", nil, "add an attribute, e.g. '#status=draft' (repeatable)")
	f.StringVar(&id, "id", "", "use this note ID (fails if it exists)")
	f.IntVar(&pos, "pos", 0, "position among siblings (10, 20, 30 …)")
	return cmd
}

// inferTypeMime applies the precedence: explicit flags, then what the file
// tells, then the title's extension for code notes.
func inferTypeMime(cmd *cobra.Command, typ, mime, file, title string, body []byte) (string, string) {
	fileType, fileMime := "", ""
	if file != "" && file != "-" {
		fileType, fileMime = content.InferFromFile(file, body)
	}
	if !changed(cmd, "type") && fileType != "" {
		typ = fileType
	}
	if changed(cmd, "mime") {
		return typ, mime
	}
	switch {
	case fileType == typ && fileMime != "":
		return typ, fileMime
	case typ == content.TypeCode:
		if m := content.MimeFromTitle(title); m != "" {
			return typ, m
		}
		if m := content.MimeFromTitle(file); m != "" {
			return typ, m
		}
		return typ, "text/plain"
	case (typ == content.TypeFile || typ == content.TypeImage) && fileMime != "":
		return typ, fileMime
	}
	return typ, ""
}

// validateRelationTargets checks relation targets locally before any request.
func validateRelationTargets(ops []attrspec.Op) error {
	for _, op := range ops {
		if op.Type == attrspec.Relation && op.Value != "" {
			if _, err := ref.Parse(op.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

// createAttrs adds attributes to a new note (set and add behave the same there).
func (rt *Runtime) createAttrs(ctx context.Context, c *etapi.Client, noteID string, ops []attrspec.Op) ([]etapi.Attribute, error) {
	var out []etapi.Attribute
	for _, op := range ops {
		value := op.Value
		if op.Type == attrspec.Relation {
			t, err := rt.resolve(ctx, c, op.Value)
			if err != nil {
				return out, err
			}
			value = t.ID
		}
		a, err := c.CreateAttribute(ctx, etapi.AttributeCreate{NoteID: noteID, Type: op.Type, Name: op.Name, Value: value, IsInheritable: op.Inheritable})
		if err != nil {
			return out, err
		}
		out = append(out, *a)
	}
	return out, nil
}

package cli

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
	"github.com/oyasmi/trilium-cli/internal/ref"
)

func attCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "att <command>",
		RunE:  groupRun,
		Short: "Manage a note's attachments",
		Long:  "Attachments are files or images stored inside a note.",
		Example: `  trilium att ls <note>
  trilium att add <note> -f chart.png
  trilium att get <attachmentId> -o chart.png`,
	}
	cmd.AddCommand(attLsCmd(rt), attGetCmd(rt), addForce(attAddCmd(rt)), addForce(attPutCmd(rt)), addForce(attUpdateCmd(rt)), addForce(attRmCmd(rt)))
	return cmd
}

// attBinary reports whether an attachment holds binary content.
func attBinary(role, mime string) bool { return role == "image" || !content.TextMime(mime) }

func validAttID(id string) error {
	if !ref.ValidID(id) {
		return output.Usage("list attachment IDs with: trilium att ls <note>", "invalid attachment ID %q", id)
	}
	return nil
}

func attLsCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "ls <note>",
		Short: "List attachments",
		Long: `List attachments: ID, role, mime, size and title. The size is Trilium's
contentLength (characters for text, bytes for binary content).`,
		Example: "  trilium att ls <note>",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			list, err := c.ListAttachments(cmd.Context(), ts[0].ID)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				rt.info("no attachments")
				return nil
			}
			for _, a := range list {
				line := strings.Join([]string{a.AttachmentID, a.Role, output.Clean(a.Mime), strconv.Itoa(a.ContentLength), output.Clean(a.Title)}, "\t")
				if err := rt.emit(a, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func attGetCmd(rt *Runtime) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "get <attachmentId> [-o FILE]",
		Short: "Print or save an attachment's content",
		Long:  "Print a text attachment's content, or save any attachment with -o FILE (binary content needs -o).",
		Example: `  trilium att get <attachmentId>
  trilium att get <attachmentId> -o photo.png`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validAttID(id); err != nil {
				return err
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := c.GetAttachment(ctx, id)
			if err != nil {
				return err
			}
			if out == "" && attBinary(a.Role, a.Mime) {
				return output.Fail("save it with: trilium att get "+id+" -o <file>", "attachment %s is binary (%s) and can't be printed", id, a.Mime)
			}
			data, err := c.GetAttachmentContent(ctx, id)
			if err != nil {
				return err
			}
			if out == "" {
				if rt.JSON {
					s := string(data)
					return output.JSONL(rt.Stdout, struct {
						etapi.Attachment
						Content string `json:"content"`
					}{*a, s})
				}
				_, err := rt.Stdout.Write(data)
				return err
			}
			if err := rt.writeOut(out, data); err != nil {
				return err
			}
			if out != "-" {
				return rt.emit(map[string]any{"attachmentId": id, "file": out, "bytes": len(data)},
					fmt.Sprintf("saved %s to %s (%d bytes)", id, out, len(data)))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "save the content to FILE (- for stdout)")
	return cmd
}

func attAddCmd(rt *Runtime) *cobra.Command {
	var (
		file, title, role, mime string
		pos                     int
	)
	cmd := &cobra.Command{
		Use:   "add <note> [-f FILE | content]",
		Short: "Add an attachment to a note",
		Long: `Add an attachment. With -f the title defaults to the file name and the mime
is inferred; image/* files get the role "image". For images, an "embed:" line
shows the HTML to put into a text note.`,
		Example: `  trilium att add <note> -f chart.png
  trilium att add <note> "name,qty" --title data.csv`,
		Args: rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, _, err := rt.readContent(args, 1, file, true)
			if err != nil {
				return err
			}
			if title == "" {
				if file == "" || file == "-" {
					return output.Usage("example: trilium att add <note> \"text\" --title notes.txt", "--title is required unless the content comes from -f FILE")
				}
				title = filepath.Base(file)
			}
			if mime == "" {
				_, mime = content.InferFromFile(title, body)
				if file != "" && file != "-" {
					_, mime = content.InferFromFile(file, body)
				}
			}
			if role == "" {
				role = "file"
				if strings.HasPrefix(mime, "image/") {
					role = "image"
				}
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := guardPlace(ts[0].ID, forceFlag(cmd)); err != nil {
				return err
			}
			ctx := cmd.Context()
			binary := attBinary(role, mime) || content.IsBinary("", mime, body)
			def := etapi.AttachmentCreate{OwnerID: ts[0].ID, Role: role, Mime: mime, Title: title, Position: intPtr(cmd, "pos", pos)}
			if !binary {
				def.Content = string(body)
			}
			a, err := c.CreateAttachment(ctx, def)
			if err != nil {
				return err
			}
			if binary {
				if err := c.PutAttachmentContent(ctx, a.AttachmentID, bytes.NewReader(body), true); err != nil {
					return output.Fail("retry with: trilium att put "+a.AttachmentID+" -f <file>", "created attachment %s but uploading its content failed: %v", a.AttachmentID, err)
				}
				// Report the attachment as stored, not as it was before the upload.
				if a, err = c.GetAttachment(ctx, a.AttachmentID); err != nil {
					return err
				}
			}
			line := fmt.Sprintf("added attachment %s to %s: %s %s %d bytes", a.AttachmentID, ts[0], output.Q(a.Title), a.Mime, len(body))
			if a.Role == "image" {
				line += "\nembed: " + embedHTML(a)
			}
			return rt.emit(a, line)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&file, "file", "f", "", "read content from FILE (- for stdin)")
	f.StringVar(&title, "title", "", "attachment title (default: file name)")
	f.StringVar(&role, "role", "", "file or image (default: inferred from mime)")
	f.StringVar(&mime, "mime", "", "mime type (default: inferred)")
	f.IntVar(&pos, "pos", 0, "position among attachments")
	return cmd
}

// embedHTML is the <img> tag Trilium uses for image attachments.
func embedHTML(a *etapi.Attachment) string {
	return fmt.Sprintf(`<img src="api/attachments/%s/image/%s">`, a.AttachmentID, url.PathEscape(a.Title))
}

func attPutCmd(rt *Runtime) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:     "put <attachmentId> [-f FILE | content]",
		Short:   "Replace an attachment's content",
		Long:    "Replace an attachment's content.",
		Example: "  trilium att put <attachmentId> -f new.png",
		Args:    rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validAttID(id); err != nil {
				return err
			}
			body, _, err := rt.readContent(args, 1, file, true)
			if err != nil {
				return err
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := c.GetAttachment(ctx, id)
			if err != nil {
				return err
			}
			if err := guardPlace(a.OwnerID, forceFlag(cmd)); err != nil {
				return err
			}
			before := -1
			binary := attBinary(a.Role, a.Mime) || content.IsBinary("", a.Mime, body)
			if !binary {
				old, err := c.GetAttachmentContent(ctx, id)
				if err != nil {
					return err
				}
				before = len(old)
			}
			if err := c.PutAttachmentContent(ctx, id, bytes.NewReader(body), binary); err != nil {
				return err
			}
			res := contentResult{NoteID: a.OwnerID, BytesAfter: len(body)}
			if before >= 0 {
				res.BytesBefore = &before
			}
			return rt.emit(struct {
				AttachmentID string `json:"attachmentId"`
				contentResult
			}{id, res}, fmt.Sprintf("updated attachment %s content: %s", id, output.Sizes(before, len(body))))
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "read content from FILE (- for stdin)")
	return cmd
}

func attUpdateCmd(rt *Runtime) *cobra.Command {
	var (
		title, role, mime string
		pos               int
	)
	cmd := &cobra.Command{
		Use:     "update <attachmentId> [--title T] [--role R] [--mime M] [--pos N]",
		Short:   "Change an attachment's title, role, mime or position",
		Long:    "Change an attachment's title, role, mime or position.",
		Example: "  trilium att update <attachmentId> --title chart-2026.png",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validAttID(id); err != nil {
				return err
			}
			p := etapi.AttachmentPatch{Title: strPtr(cmd, "title", title), Role: strPtr(cmd, "role", role), Mime: strPtr(cmd, "mime", mime), Position: intPtr(cmd, "pos", pos)}
			if p.Title == nil && p.Role == nil && p.Mime == nil && p.Position == nil {
				return output.Usage("", "nothing to update: give --title, --role, --mime or --pos")
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			before, err := c.GetAttachment(ctx, id)
			if err != nil {
				return err
			}
			if err := guardPlace(before.OwnerID, forceFlag(cmd)); err != nil {
				return err
			}

			a, err := c.PatchAttachment(ctx, id, p)
			if err != nil {
				return err
			}
			var parts []string
			if p.Title != nil {
				parts = append(parts, "title: "+output.Q(before.Title)+" → "+output.Q(a.Title))
			}
			if p.Role != nil {
				parts = append(parts, "role: "+before.Role+" → "+a.Role)
			}
			if p.Mime != nil {
				parts = append(parts, "mime: "+before.Mime+" → "+a.Mime)
			}
			if p.Position != nil {
				parts = append(parts, fmt.Sprintf("position: %d → %d", before.Position, a.Position))
			}
			return rt.emit(a, "updated attachment "+id+" "+strings.Join(parts, ", "))
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new title")
	f.StringVar(&role, "role", "", "file or image")
	f.StringVar(&mime, "mime", "", "new mime type")
	f.IntVar(&pos, "pos", 0, "new position")
	return cmd
}

func attRmCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <attachmentId>",
		Short:   "Delete an attachment",
		Long:    "Delete an attachment.",
		Example: "  trilium att rm <attachmentId>",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := validAttID(id); err != nil {
				return err
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := c.GetAttachment(ctx, id)
			if err != nil {
				return err
			}
			if err := guardPlace(a.OwnerID, forceFlag(cmd)); err != nil {
				return err
			}
			if err := c.DeleteAttachment(ctx, id); err != nil {
				return err
			}
			return rt.emit(a, fmt.Sprintf("deleted attachment %s %s from %s", id, output.Q(a.Title), a.OwnerID))
		},
	}
}

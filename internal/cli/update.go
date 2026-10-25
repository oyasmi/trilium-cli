package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

func updateCmd(rt *Runtime) *cobra.Command {
	var (
		title, typ, mime string
		force            bool
	)
	cmd := &cobra.Command{
		Use:   "update <note> [--title T] [--type TYPE] [--mime M]",
		Short: "Change a note's title, type or mime",
		Long:  "Change a note's title, type or mime. Only the given fields are sent.",
		Example: `  trilium update <id> --title "Final report"
  trilium update <id> --type code --mime text/x-python`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := etapi.NotePatch{Title: strPtr(cmd, "title", title), Type: strPtr(cmd, "type", typ), Mime: strPtr(cmd, "mime", mime)}
			if p.Title == nil && p.Type == nil && p.Mime == nil {
				return output.Usage("example: trilium update <note> --title \"New title\"", "nothing to update: give --title, --type or --mime")
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			t := ts[0]
			if err := guardWrite(t.ID, force); err != nil {
				return err
			}
			before, err := c.GetNote(cmd.Context(), t.ID)
			if err != nil {
				return err
			}
			after, err := c.PatchNote(cmd.Context(), t.ID, p)
			if err != nil {
				return err
			}
			var changes []string
			if p.Title != nil {
				changes = append(changes, "title: "+output.Q(before.Title)+" → "+output.Q(after.Title))
			}
			if p.Type != nil {
				changes = append(changes, "type: "+before.Type+" → "+after.Type)
			}
			if p.Mime != nil {
				changes = append(changes, "mime: "+orNone(before.Mime)+" → "+orNone(after.Mime))
			}
			return rt.emit(after, "updated "+t.String()+" "+strings.Join(changes, ", "))
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new title")
	f.StringVar(&typ, "type", "", "new note type")
	f.StringVar(&mime, "mime", "", "new mime type")
	f.BoolVar(&force, "force", false, "allow changing Trilium system notes")
	return cmd
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

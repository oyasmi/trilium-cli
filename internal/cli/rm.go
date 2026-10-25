package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

func rmCmd(rt *Runtime) *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:   "rm <note>...",
		Short: "Delete notes",
		Long: `Delete notes. This removes the note from every place it appears in the tree,
together with its whole subtree. Notes with children need -r.
To remove just one placement of a cloned note, use: trilium branch rm
Deleted notes can be restored with: trilium undelete <id>`,
		Example: `  trilium rm <id>
  trilium rm <id1> <id2> -r`,
		Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ts, err := rt.setup(cmd.Context(), args...)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			// Check every note before deleting any of them.
			notes := make([]*etapi.Note, len(ts))
			for i, t := range ts {
				if err := guardStructure(t.ID, "delete"); err != nil {
					return err
				}
				n, err := c.GetNote(ctx, t.ID)
				if err != nil {
					return err
				}
				if k := len(n.ChildNoteIDs); k > 0 && !recursive {
					return output.Usage("add -r to delete the whole subtree", "%s %s has %s; use -r to delete them too",
						n.NoteID, output.Q(n.Title), plural(k, "child", "children"))
				}
				notes[i] = n
			}
			for _, n := range notes {
				if err := c.DeleteNote(ctx, n.NoteID); err != nil {
					return err
				}
				extra := ""
				if k := len(n.ChildNoteIDs); k > 0 {
					extra = fmt.Sprintf("+%s; ", plural(k, "child", "children"))
				}
				line := fmt.Sprintf("deleted %s %s (%sundo: trilium undelete %s)", n.NoteID, output.Q(n.Title), extra, n.NoteID)
				if k := len(n.ParentNoteIDs); k > 1 {
					line += fmt.Sprintf(" (was in %d places; to remove one placement use: trilium branch rm)", k)
				}
				if err := rt.emit(n, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "also delete children")
	return cmd
}

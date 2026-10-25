package cli

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

func mvCmd(rt *Runtime) *cobra.Command {
	var (
		from string
		pos  int
	)
	cmd := &cobra.Command{
		Use:   "mv <note> <new-parent>",
		Short: "Move a note to another parent",
		Long: `Move a note under another parent. The new placement is created before the
old one is removed, so a failure never loses the note. A cloned note (with
several parents) needs --from to say which placement moves.`,
		Example: `  trilium mv <id> <new-parent>
  trilium mv <id> <new-parent> --from <old-parent> --pos 5`,
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs := []string{args[0], args[1]}
			if from != "" {
				refs = append(refs, from)
			}
			c, ts, err := rt.setup(cmd.Context(), refs...)
			if err != nil {
				return err
			}
			t, dest := ts[0], ts[1]
			if err := guardStructure(t.ID, "move"); err != nil {
				return err
			}
			ctx := cmd.Context()
			n, err := c.GetNote(ctx, t.ID)
			if err != nil {
				return err
			}
			var src string
			switch {
			case from != "":
				src = ts[2].ID
				if !slices.Contains(n.ParentNoteIDs, src) {
					return output.Usage("its parents are: "+strings.Join(n.ParentNoteIDs, ", "), "%s is not under %s", t.ID, src)
				}
			case len(n.ParentNoteIDs) == 1:
				src = n.ParentNoteIDs[0]
			default:
				return output.Usage("add --from <parent>; its parents are: "+strings.Join(n.ParentNoteIDs, ", "), "%s is in %d places; say which one to move", t.ID, len(n.ParentNoteIDs))
			}
			if src == dest.ID {
				return output.Usage("to change its position use: trilium branch set "+t.ID+" "+src+" --pos N", "%s is already under %s", t.ID, src)
			}
			if err := guardPlace(dest.ID, forceFlag(cmd)); err != nil {
				return err
			}
			if err := checkCycle(ctx, c, t.ID, dest.ID); err != nil {
				return err
			}
			old, err := findBranch(ctx, c, t.ID, src)
			if err != nil {
				return err
			}
			nb, _, err := c.CreateBranch(ctx, etapi.BranchCreate{NoteID: t.ID, ParentNoteID: dest.ID, Prefix: nonEmpty(old.Prefix), NotePosition: intPtr(cmd, "pos", pos)})
			if err != nil {
				return err
			}
			if err := c.DeleteBranch(ctx, old.BranchID); err != nil {
				return output.Fail("remove the old placement with: trilium branch rm "+t.ID+" "+src,
					"%s now appears under %s, but removing it from %s failed: %v", t.ID, dest.ID, src, err)
			}
			return rt.emit(nb, fmt.Sprintf("moved %s %s: %s → %s", t, output.Q(n.Title), src, dest))
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "the parent to move it away from (for cloned notes)")
	cmd.Flags().IntVar(&pos, "pos", 0, "position among new siblings (10, 20, 30 …)")
	return cmd
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func branchCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch <command>",
		RunE:  groupRun,
		Short: "Manage where a note appears in the tree (clones)",
		Long: `A branch is one placement of a note in the tree; cloning adds another one.
Branches are addressed as <note> <parent>, or by branch ID.`,
		Example: `  trilium branch ls <note>
  trilium branch add <note> <parent>
  trilium branch rm <note> <parent>`,
	}
	cmd.AddCommand(branchLsCmd(rt), addForce(branchAddCmd(rt)), addForce(branchSetCmd(rt)), branchRmCmd(rt))
	return cmd
}

func branchLine(b *etapi.Branch) string {
	s := b.BranchID + "\t" + b.ParentNoteID + "\t" + strconv.Itoa(b.NotePosition)
	if b.Prefix != "" {
		s += "\t" + output.Clean(b.Prefix)
	}
	return s
}

func branchLsCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "ls <note>",
		Short:   "List the places a note appears",
		Long:    "List the note's placements: branch ID, parent, position and prefix.",
		Example: "  trilium branch ls <note>",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			n, err := c.GetNote(cmd.Context(), ts[0].ID)
			if err != nil {
				return err
			}
			branches, err := getBranches(cmd.Context(), c, n.ParentBranchIDs)
			if err != nil {
				return err
			}
			for _, b := range branches {
				if err := rt.emit(b, branchLine(b)); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func getBranches(ctx context.Context, c *etapi.Client, ids []string) ([]*etapi.Branch, error) {
	out := make([]*etapi.Branch, len(ids))
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)
	for i, id := range ids {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i], errs[i] = c.GetBranch(ctx, id)
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func branchAddCmd(rt *Runtime) *cobra.Command {
	var (
		prefix string
		pos    int
	)
	cmd := &cobra.Command{
		Use:   "add <note> <parent>",
		Short: "Clone a note into another parent",
		Long: `Make the note also appear under <parent> (a clone: one note, two places).
If it is already there, its prefix/position are updated instead.`,
		Example: "  trilium branch add <note> <parent> --prefix \"2026: \"",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ts, err := rt.setup(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			t, p := ts[0], ts[1]
			if t.ID == "root" {
				return output.Usage("", "root can't be cloned")
			}
			for _, id := range []string{t.ID, p.ID} {
				if err := guardPlace(id, forceFlag(cmd)); err != nil {
					return err
				}
			}
			ctx := cmd.Context()
			if err := checkCycle(ctx, c, t.ID, p.ID); err != nil {
				return err
			}
			b, created, err := c.CreateBranch(ctx, etapi.BranchCreate{NoteID: t.ID, ParentNoteID: p.ID, Prefix: strPtr(cmd, "prefix", prefix), NotePosition: intPtr(cmd, "pos", pos)})
			if err != nil {
				return err
			}
			if !created {
				return rt.emit(b, fmt.Sprintf("updated existing branch %s (%s was already in %s)", b.BranchID, t, p))
			}
			return rt.emit(b, fmt.Sprintf("cloned %s into %s (branch %s)", t, p, b.BranchID))
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "title prefix shown in this placement only")
	cmd.Flags().IntVar(&pos, "pos", 0, "position among siblings (10, 20, 30 …)")
	return cmd
}

// locateBranch resolves "<note> <parent>" or a single branch ID.
func (rt *Runtime) locateBranch(ctx context.Context, args []string) (*etapi.Client, *etapi.Branch, error) {
	if len(args) == 1 {
		c, err := rt.Client()
		if err != nil {
			return nil, nil, err
		}
		b, err := c.GetBranch(ctx, args[0])
		return c, b, err
	}
	c, ts, err := rt.setup(ctx, args[0], args[1])
	if err != nil {
		return nil, nil, err
	}
	b, err := findBranch(ctx, c, ts[0].ID, ts[1].ID)
	return c, b, err
}

func branchSetCmd(rt *Runtime) *cobra.Command {
	var (
		prefix   string
		pos      int
		expanded bool
	)
	cmd := &cobra.Command{
		Use:   "set (<note> <parent> | <branchId>) [--prefix P] [--pos N] [--expanded=true|false]",
		Short: "Change a placement's prefix, position or expanded state",
		Long:  "Change a placement's prefix, position or expanded state. Changing --pos refreshes the parent's ordering.",
		Example: `  trilium branch set <note> <parent> --pos 5
  trilium branch set <note> <parent> --prefix ""`,
		Args: rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := etapi.BranchPatch{Prefix: strPtr(cmd, "prefix", prefix), NotePosition: intPtr(cmd, "pos", pos)}
			if changed(cmd, "expanded") {
				p.IsExpanded = &expanded
			}
			if p.Prefix == nil && p.NotePosition == nil && p.IsExpanded == nil {
				return output.Usage("", "nothing to change: give --prefix, --pos or --expanded")
			}
			ctx := cmd.Context()
			c, b, err := rt.locateBranch(ctx, args)
			if err != nil {
				return err
			}
			for _, id := range []string{b.NoteID, b.ParentNoteID} {
				if err := guardPlace(id, forceFlag(cmd)); err != nil {
					return err
				}
			}
			nb, err := c.PatchBranch(ctx, b.BranchID, p)
			if err != nil {
				return err
			}
			if p.NotePosition != nil {
				if err := c.RefreshNoteOrdering(ctx, nb.ParentNoteID); err != nil {
					return err
				}
			}
			var parts []string
			if p.Prefix != nil {
				parts = append(parts, "prefix "+output.Q(nb.Prefix))
			}
			if p.NotePosition != nil {
				parts = append(parts, fmt.Sprintf("position %d", nb.NotePosition))
			}
			if p.IsExpanded != nil {
				parts = append(parts, fmt.Sprintf("expanded %v", nb.IsExpanded))
			}
			return rt.emit(nb, fmt.Sprintf("updated branch %s: %s", nb.BranchID, strings.Join(parts, ", ")))
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "title prefix (\"\" removes it)")
	cmd.Flags().IntVar(&pos, "pos", 0, "position among siblings")
	cmd.Flags().BoolVar(&expanded, "expanded", false, "show the note expanded in the tree")
	return cmd
}

func branchRmCmd(rt *Runtime) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm (<note> <parent> | <branchId>)",
		Short: "Remove a note from one place",
		Long: `Remove one placement of a note. The note stays in its other places.
Removing its last placement would delete the note; that needs --force
(or use: trilium rm).`,
		Example: "  trilium branch rm <note> <parent>",
		Args:    rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, b, err := rt.locateBranch(ctx, args)
			if err != nil {
				return err
			}
			if err := guardStructure(b.NoteID, "remove"); err != nil {
				return err
			}
			n, err := c.GetNote(ctx, b.NoteID)
			if err != nil {
				return err
			}
			if len(n.ParentBranchIDs) <= 1 && !force {
				return output.Usage("to delete the note use: trilium rm "+n.NoteID+" (or add --force)",
					"this is the only place %s %s appears; removing it would delete the note", n.NoteID, output.Q(n.Title))
			}
			if err := c.DeleteBranch(ctx, b.BranchID); err != nil {
				return err
			}
			return rt.emit(b, fmt.Sprintf("removed %s from %s (branch %s)", b.NoteID, b.ParentNoteID, b.BranchID))
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "allow removing the last placement (deletes the note)")
	return cmd
}

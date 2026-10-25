package cli

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

type treeEntry struct {
	note  *etapi.Note
	depth int
}

// lsJSON adds the tree depth to a note in --json output of `ls -d N`.
type lsJSON struct {
	etapi.Note
	Depth int `json:"depth,omitempty"`
}

func lsCmd(rt *Runtime) *cobra.Command {
	var (
		depth, limit int
		long, all    bool
	)
	cmd := &cobra.Command{
		Use:   "ls [note]",
		Short: "List child notes",
		Long: `List a note's children in tree order (default: root). Archived notes are
included and marked. With -d N, show N levels as an indented tree.`,
		Example: `  trilium ls
  trilium ls <note> -d 2
  trilium ls @today -l`,
		Args: rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if depth < 1 {
				return output.Usage("", "-d must be 1 or more")
			}
			ref := "root"
			if len(args) == 1 {
				ref = args[0]
			}
			c, ts, err := rt.setup(cmd.Context(), ref)
			if err != nil {
				return err
			}
			entries, err := listTree(cmd.Context(), c, ts[0].ID, depth, all)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				rt.info("no matches")
				return nil
			}
			shown := entries
			if limit > 0 && len(entries) > limit {
				shown = entries[:limit]
			}
			for _, e := range shown {
				if rt.JSON {
					v := lsJSON{Note: *e.note}
					if depth > 1 {
						v.Depth = e.depth
					}
					if err := output.JSONL(rt.Stdout, v); err != nil {
						return err
					}
					continue
				}
				rt.println(output.TreeLine(e.note, e.depth, long, rt.Loc))
			}
			if len(shown) < len(entries) {
				rt.info("%d shown; %d more (use -n)", len(shown), len(entries)-len(shown))
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVarP(&depth, "depth", "d", 1, "levels to show")
	f.IntVarP(&limit, "limit", "n", 100, "maximum lines (0 = no limit)")
	f.BoolVarP(&long, "long", "l", false, "also show mime and modification time")
	f.BoolVar(&all, "all", false, "include Trilium system notes (IDs starting with _)")
	return cmd
}

// listTree returns the subtree below parentID in tree order. It needs two
// requests: the parent (for child order) and one search for all descendants
// up to the depth. Notes the search does not return (e.g. in Trilium's
// hidden subtree) are fetched one by one.
func listTree(ctx context.Context, c *etapi.Client, parentID string, depth int, all bool) ([]treeEntry, error) {
	parent, err := c.GetNote(ctx, parentID)
	if err != nil {
		return nil, err
	}
	res, err := c.Search(ctx, etapi.SearchParams{
		Search:               `note.noteId != ""`,
		AncestorNoteID:       parentID,
		AncestorDepth:        fmt.Sprintf("lt%d", depth+1),
		IncludeArchivedNotes: true,
	})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*etapi.Note, len(res)+1)
	for i := range res {
		byID[res[i].NoteID] = &res[i]
	}
	byID[parent.NoteID] = parent

	var out []treeEntry
	level := []treeEntry{{note: parent}}
	for d := 1; d <= depth && len(level) > 0; d++ {
		if err := fetchMissing(ctx, c, byID, level, all); err != nil {
			return nil, err
		}
		var next []treeEntry
		for _, e := range level {
			for _, id := range e.note.ChildNoteIDs {
				if !all && output.IsSystemID(id) && !output.IsSystemID(e.note.NoteID) {
					continue
				}
				if n := byID[id]; n != nil {
					next = append(next, treeEntry{note: n, depth: d})
				}
			}
		}
		level = next
		out = append(out, next...)
	}
	return order(out, parent, depth, all), nil
}

// fetchMissing loads children of the level that the search did not return.
func fetchMissing(ctx context.Context, c *etapi.Client, byID map[string]*etapi.Note, level []treeEntry, all bool) error {
	var missing []string
	for _, e := range level {
		for _, id := range e.note.ChildNoteIDs {
			if !all && output.IsSystemID(id) && !output.IsSystemID(e.note.NoteID) {
				continue
			}
			if byID[id] == nil {
				missing = append(missing, id)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	sem := make(chan struct{}, 8)
	for _, id := range missing {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			n, err := c.GetNote(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			byID[id] = n
		})
	}
	wg.Wait()
	return firstErr
}

// order lays the collected notes out depth-first, the way a tree is read.
func order(entries []treeEntry, parent *etapi.Note, depth int, all bool) []treeEntry {
	byID := map[string]*etapi.Note{}
	for _, e := range entries {
		byID[e.note.NoteID] = e.note
	}
	var out []treeEntry
	var walk func(n *etapi.Note, d int, path map[string]bool)
	walk = func(n *etapi.Note, d int, path map[string]bool) {
		if d > depth {
			return
		}
		for _, id := range n.ChildNoteIDs {
			if !all && output.IsSystemID(id) && !output.IsSystemID(n.NoteID) {
				continue
			}
			child := byID[id]
			if child == nil || path[id] {
				continue
			}
			out = append(out, treeEntry{note: child, depth: d})
			path[id] = true
			walk(child, d+1, path)
			delete(path, id)
		}
	}
	walk(parent, 1, map[string]bool{parent.NoteID: true})
	return out
}

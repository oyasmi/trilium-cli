package cli

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

func searchCmd(rt *Runtime) *cobra.Command {
	var (
		limit                      int
		in, depth, sort            string
		desc, fast, archived, long bool
	)
	cmd := &cobra.Command{
		Use:   "search <query...>",
		Short: "Find notes",
		Long: `Find notes with Trilium's search syntax. Words are joined with spaces.
Values containing '.' or '-' must be quoted: note.title="hello.py"
Full syntax: trilium help search`,
		Example: `  trilium search meeting notes
  trilium search '#status=todo' --in <project>
  trilium search 'note.title *= "report"' --sort dateModified --desc -n 5`,
		Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs := []string{}
			if in != "" {
				refs = append(refs, in)
			}
			c, ts, err := rt.setup(cmd.Context(), refs...)
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			p := etapi.SearchParams{
				Search: query, FastSearch: fast, IncludeArchivedNotes: archived,
				AncestorDepth: depth, OrderBy: sort,
			}
			if len(ts) > 0 {
				p.AncestorNoteID = ts[0].ID
			}
			if desc {
				p.OrderDirection = "desc"
			}
			if limit > 0 {
				p.Limit = limit + 1
			}
			res, err := c.Search(cmd.Context(), p)
			if err != nil {
				return err
			}
			notes := make([]etapi.Note, 0, len(res))
			for _, n := range res {
				// The server returns the --in note itself, and hidden-subtree notes.
				if !output.IsSystemID(n.NoteID) && n.NoteID != p.AncestorNoteID {
					notes = append(notes, n)
				}
			}
			// The server stopped at its limit: there may be more even if
			// filtering left fewer rows than that.
			more := limit > 0 && len(res) >= p.Limit
			if limit > 0 && len(notes) > limit {
				notes, more = notes[:limit], true
			}
			if len(notes) == 0 {
				rt.info("no matches")
				if needsQuotes(query) {
					rt.info(`hint: values containing '.' or '-' must be quoted, e.g. note.title="hello.py"`)
				}
				return nil
			}
			for i := range notes {
				if rt.JSON {
					if err := output.JSONL(rt.Stdout, notes[i]); err != nil {
						return err
					}
					continue
				}
				rt.println(output.NoteLine(&notes[i], long, rt.Loc))
			}
			if more {
				rt.info("%d shown; more available (use -n)", limit)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVarP(&limit, "limit", "n", 20, "maximum results (0 = no limit)")
	f.StringVar(&in, "in", "", "only search under this note")
	f.StringVar(&depth, "depth", "", "depth below --in: eqN, ltN or gtN")
	f.StringVar(&sort, "sort", "", "order by: title, dateModified, dateCreated, #label …")
	f.BoolVar(&desc, "desc", false, "descending order")
	f.BoolVar(&fast, "fast", false, "don't search note content (faster)")
	f.BoolVar(&archived, "archived", false, "include archived notes")
	f.BoolVarP(&long, "long", "l", false, "also show mime and modification time")
	return cmd
}

// unquotedValue finds comparison values that are not quoted.
var unquotedValue = regexp.MustCompile(`(?:!=|\*=\*|\*=|=\*|%=|>=|<=|=|>|<)\s*([^\s"'` + "`" + `()]+)`)

// needsQuotes reports whether the query compares against an unquoted value
// containing '.' or '-', which Trilium silently fails to match.
func needsQuotes(q string) bool {
	for _, m := range unquotedValue.FindAllStringSubmatch(q, -1) {
		if strings.ContainsAny(m[1], ".-") {
			return true
		}
	}
	return false
}

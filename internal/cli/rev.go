package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
	"github.com/oyasmi/trilium-cli/internal/ref"
)

func revCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rev <command>",
		RunE:  groupRun,
		Short: "List, read and save note revisions",
		Long:  "Revisions are saved snapshots of a note's title and content.",
		Example: `  trilium rev ls <note>
  trilium rev add <note>
  trilium rev get <revisionId>`,
	}
	cmd.AddCommand(revLsCmd(rt), revGetCmd(rt), revAddCmd(rt))
	return cmd
}

// newestFirst sorts revisions by creation time, newest first.
func newestFirst(revs []etapi.Revision) {
	slices.SortStableFunc(revs, func(a, b etapi.Revision) int { return strings.Compare(b.UTCDateCreated, a.UTCDateCreated) })
}

func revLsCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "ls <note>",
		Short: "List a note's revisions",
		Long: `List revisions, newest first: ID, created, size and title. The size is
Trilium's contentLength (characters for text content).`,
		Example: "  trilium rev ls <note>",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			revs, err := c.ListRevisions(cmd.Context(), ts[0].ID)
			if err != nil {
				return err
			}
			if len(revs) == 0 {
				rt.info("no revisions")
				return nil
			}
			newestFirst(revs)
			for _, r := range revs {
				line := strings.Join([]string{r.RevisionID, output.Time(r.UTCDateCreated, rt.Loc), strconv.Itoa(r.ContentLength), r.Title}, "\t")
				if err := rt.emit(r, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func revGetCmd(rt *Runtime) *cobra.Command {
	var (
		text bool
		out  string
	)
	cmd := &cobra.Command{
		Use:   "get <revisionId> [--text] [-o FILE]",
		Short: "Show a revision",
		Long:  "Show a revision's metadata and content, like get. --text gives a read-only plain-text view.",
		Example: `  trilium rev get <revisionId>
  trilium rev get <revisionId> --text`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !ref.ValidID(id) {
				return output.Usage("list revision IDs with: trilium rev ls <note>", "invalid revision ID %q", id)
			}
			if out != "" && text {
				return output.Usage("", "-o saves the raw content; it can't be combined with --text")
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			r, err := c.GetRevision(ctx, id)
			if err != nil {
				return err
			}
			binary := content.BinaryType(r.Type, r.Mime)
			if binary && out == "" {
				if text {
					return output.Fail("save it with: trilium rev get "+id+" -o <file>", "revision %s has binary content (%s)", id, r.Mime)
				}
				if rt.JSON {
					return output.JSONL(rt.Stdout, r)
				}
				fmt.Fprint(rt.Stdout, output.RevisionHeader(r, output.BinaryNotice(r.Mime, "trilium rev get "+id+" -o <file>"), rt.Loc))
				return nil
			}
			data, err := c.GetRevisionContent(ctx, id)
			if err != nil {
				return err
			}
			if out != "" {
				if err := rt.writeOut(out, data); err != nil {
					return err
				}
				if out != "-" {
					rt.printf("saved %s to %s (%d bytes)\n", id, out, len(data))
				}
				return nil
			}
			notice := ""
			if text && r.Type == content.TypeText {
				data, notice = []byte(content.PlainText(string(data))), output.TextViewNotice
			}
			if rt.JSON {
				v := struct {
					etapi.Revision
					Content     string `json:"content"`
					ContentView string `json:"contentView,omitempty"`
				}{Revision: *r, Content: string(data)}
				if notice != "" {
					v.ContentView = "text"
				}
				return output.JSONL(rt.Stdout, v)
			}
			fmt.Fprint(rt.Stdout, output.RevisionHeader(r, notice, rt.Loc))
			if _, err := rt.Stdout.Write(data); err != nil {
				return err
			}
			if len(data) > 0 && data[len(data)-1] != '\n' {
				rt.println("")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&text, "text", false, "plain-text view of text revisions (read-only)")
	cmd.Flags().StringVarP(&out, "output", "o", "", "save the raw content to FILE (- for stdout)")
	return cmd
}

func revAddCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "add <note>",
		Short:   "Save a revision of a note now",
		Long:    "Save a snapshot of the note's current title and content as a revision.",
		Example: "  trilium rev add <note>",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			t := ts[0]
			ctx := cmd.Context()
			if err := c.CreateRevision(ctx, t.ID); err != nil {
				return err
			}
			// The endpoint answers 204 without an ID; the newest revision is ours.
			revs, err := c.ListRevisions(ctx, t.ID)
			if err != nil {
				return err
			}
			if len(revs) == 0 {
				return output.Fail("", "Trilium accepted the request but no revision of %s is listed", t.ID)
			}
			newestFirst(revs)
			return rt.emit(revs[0], fmt.Sprintf("created revision %s of %s", revs[0].RevisionID, t))
		},
	}
}

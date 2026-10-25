package cli

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

type replacement struct {
	old, new []byte
	label    string // how the user refers to it in errors, e.g. "--old #2"
}

func editCmd(rt *Runtime) *cobra.Command {
	var (
		olds, news, oldFiles, newFiles []string
		all, force                     bool
	)
	cmd := &cobra.Command{
		Use:   "edit <note> --old TEXT --new TEXT [--old TEXT --new TEXT]...",
		Short: "Replace exact text in a note",
		Long: `Replace exact text in a note's raw content (HTML for text notes), like a
str_replace tool. Pairs are applied in order; nothing is written unless every
--old matches. Each --old must match exactly once unless --all is given.
Use --new "" to delete text, and --old-file/--new-file for long snippets.`,
		Example: `  trilium edit <id> --old "<p>Draft</p>" --new "<p>Final</p>"
  trilium edit <id> --old-file old.html --new-file new.html
  trilium edit <id> --old "2025" --new "2026" --all`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reps, err := collectReplacements(olds, news, oldFiles, newFiles)
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
			count := 0
			before, after, err := rewrite(cmd.Context(), c, t.ID, func(_ *etapi.Note, old []byte) ([]byte, [][]byte, error) {
				out, n, err := applyReplacements(t.ID, old, reps, all)
				count = n
				var check [][]byte
				for _, r := range reps {
					check = append(check, r.new)
				}
				return out, check, err
			})
			if err != nil {
				return err
			}
			b := before
			return rt.emit(contentResult{NoteID: t.ID, BytesBefore: &b, BytesAfter: after, Replacements: &count},
				fmt.Sprintf("edited %s %s: %s", t, plural(count, "replacement", "replacements"), output.Sizes(before, after)))
		},
	}
	f := cmd.Flags()
	f.StringArrayVar(&olds, "old", nil, "exact text to find (repeatable, paired with --new)")
	f.StringArrayVar(&news, "new", nil, "replacement text")
	f.StringArrayVar(&oldFiles, "old-file", nil, "read the text to find from FILE")
	f.StringArrayVar(&newFiles, "new-file", nil, "read the replacement from FILE")
	f.BoolVar(&all, "all", false, "replace every match instead of requiring exactly one")
	f.BoolVar(&force, "force", false, "allow changing Trilium system notes")
	return cmd
}

func collectReplacements(olds, news, oldFiles, newFiles []string) ([]replacement, error) {
	if len(olds) != len(news) {
		return nil, output.Usage("pair every --old with a --new (use --new \"\" to delete)", "got %d --old and %d --new", len(olds), len(news))
	}
	if len(oldFiles) != len(newFiles) {
		return nil, output.Usage("", "got %d --old-file and %d --new-file", len(oldFiles), len(newFiles))
	}
	if len(olds)+len(oldFiles) == 0 {
		return nil, output.Usage("example: trilium edit <note> --old \"<p>a</p>\" --new \"<p>b</p>\"", "nothing to replace: give --old and --new")
	}
	var reps []replacement
	for i := range olds {
		reps = append(reps, replacement{old: []byte(olds[i]), new: []byte(news[i]), label: fmt.Sprintf("--old #%d", i+1)})
	}
	for i := range oldFiles {
		o, err := os.ReadFile(oldFiles[i])
		if err != nil {
			return nil, output.Usage("", "can't read --old-file: %v", err)
		}
		n, err := os.ReadFile(newFiles[i])
		if err != nil {
			return nil, output.Usage("", "can't read --new-file: %v", err)
		}
		reps = append(reps, replacement{old: o, new: n, label: "--old-file " + oldFiles[i]})
	}
	for _, r := range reps {
		if len(r.old) == 0 {
			return nil, output.Usage("", "%s is empty; there is nothing to find", r.label)
		}
	}
	return reps, nil
}

// applyReplacements applies all pairs in order, failing without partial results.
func applyReplacements(id string, b []byte, reps []replacement, all bool) ([]byte, int, error) {
	total := 0
	for _, r := range reps {
		n := bytes.Count(b, r.old)
		switch {
		case n == 0:
			return nil, 0, output.Fail("text must match exactly; text notes are raw HTML. View it with: trilium get -c "+id,
				"%s not found in %s (nothing was changed)", r.label, id)
		case n > 1 && !all:
			return nil, 0, output.Fail("add more surrounding text to make it unique, or use --all to replace every match",
				"%s matches %d times in %s (nothing was changed)", r.label, n, id)
		}
		if all {
			b = bytes.ReplaceAll(b, r.old, r.new)
		} else {
			b = bytes.Replace(b, r.old, r.new, 1)
		}
		total += n
	}
	return b, total, nil
}

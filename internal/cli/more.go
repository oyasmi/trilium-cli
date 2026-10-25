package cli

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
	"github.com/oyasmi/trilium-cli/internal/ref"
)

func historyCmd(rt *Runtime) *cobra.Command {
	var (
		in    string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "history [--in <note>] [-n 20]",
		Short: "Show recently changed notes",
		Long: `Show recent note creations, changes and deletions, newest first.
Deleted notes that can be restored are marked; restore them with: trilium undelete <id>`,
		Example: `  trilium history
  trilium history --in <project> -n 50`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs := []string{}
			if in != "" {
				refs = append(refs, in)
			}
			c, ts, err := rt.setup(cmd.Context(), refs...)
			if err != nil {
				return err
			}
			ancestor := ""
			if len(ts) > 0 {
				ancestor = ts[0].ID
			}
			changes, err := c.History(cmd.Context(), ancestor)
			if err != nil {
				return err
			}
			changes = dedupe(changes)
			slices.SortStableFunc(changes, func(a, b etapi.RecentChange) int { return strings.Compare(b.UTCDate, a.UTCDate) })
			if len(changes) == 0 {
				rt.info("no matches")
				return nil
			}
			more := limit > 0 && len(changes) > limit
			if more {
				changes = changes[:limit]
			}
			for _, ch := range changes {
				title := ch.Title
				if title == "" {
					title = ch.CurrentTitle
				}
				f := []string{output.Time(ch.UTCDate, rt.Loc), ch.NoteID, output.Clean(title)}
				if ch.CurrentIsDeleted {
					f = append(f, "deleted")
					if ch.CanBeUndeleted {
						f = append(f, "(can undelete)")
					}
				}
				if err := rt.emit(ch, strings.Join(f, "\t")); err != nil {
					return err
				}
			}
			if more {
				rt.info("%d shown; more available (use -n)", limit)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "only changes under this note")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "maximum entries (0 = no limit)")
	return cmd
}

// dedupe drops exact duplicate records, which the server sometimes returns.
func dedupe(in []etapi.RecentChange) []etapi.RecentChange {
	seen := map[etapi.RecentChange]bool{}
	out := in[:0:0]
	for _, c := range in {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

var unsafeFileChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]+`)

// fileName turns a note title into a safe file name.
func fileName(title, fallback string) string {
	s := strings.Trim(unsafeFileChars.ReplaceAllString(title, "_"), " .")
	if s == "" {
		return fallback
	}
	return s
}

func exportCmd(rt *Runtime) *cobra.Command {
	var out, format string
	cmd := &cobra.Command{
		Use:   "export <note> [-o FILE] [--format html|markdown]",
		Short: "Export a note and its subtree as a zip file",
		Long:  "Export a note and everything below it as a zip file (default: ./<title>.zip).",
		Example: `  trilium export <note>
  trilium export <note> --format markdown -o notes.zip`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "html" && format != "markdown" {
				return output.Usage("", "--format must be html or markdown")
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			t := ts[0]
			ctx := cmd.Context()
			if out == "" {
				n, err := c.GetNote(ctx, t.ID)
				if err != nil {
					return err
				}
				out = "./" + fileName(n.Title, t.ID) + ".zip"
			}
			data, err := c.Export(ctx, t.ID, format)
			if err != nil {
				return err
			}
			if err := rt.writeOut(out, data); err != nil {
				return err
			}
			if out == "-" {
				return nil
			}
			return rt.emit(map[string]any{"noteId": t.ID, "file": out, "bytes": len(data)},
				fmt.Sprintf("exported %s to %s (%s)", t, out, output.HumanSize(int64(len(data)))))
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "zip file to write (- for stdout)")
	cmd.Flags().StringVar(&format, "format", "html", "html or markdown")
	return cmd
}

func importCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "import <parent> <file.zip | file.md>",
		Short: "Import a zip export or a Markdown file",
		Long: `Import a Trilium zip export, or a Markdown file. A .md file becomes a rich
text note: Trilium's own importer converts the Markdown to HTML (the CLI does
not convert anything itself).`,
		Example: `  trilium import <parent> backup.zip
  trilium import <parent> report.md`,
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[1]
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".zip" && ext != ".md" && ext != ".markdown" {
				return output.Usage("", "can import .zip or .md files, not %q", filepath.Base(path))
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return output.Usage("", "can't read %s: %v", path, err)
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := guardPlace(ts[0].ID, forceFlag(cmd)); err != nil {
				return err
			}
			ctx := cmd.Context()
			if ext != ".zip" {
				info, err := c.AppInfo(ctx)
				if err != nil {
					return err
				}
				if data, err = markdownZip(filepath.Base(path), data, info.AppVersion); err != nil {
					return err
				}
			}
			res, err := c.Import(ctx, ts[0].ID, bytes.NewReader(data))
			if err != nil {
				return err
			}
			return rt.emit(res.Note, fmt.Sprintf("imported %s %s into %s", res.Note.NoteID, output.Q(res.Note.Title), ts[0]))
		},
	}
}

type metaFile struct {
	FormatVersion int        `json:"formatVersion"`
	AppVersion    string     `json:"appVersion"`
	Files         []metaNote `json:"files"`
}

type metaNote struct {
	IsClone      bool     `json:"isClone"`
	NoteID       string   `json:"noteId"`
	NotePath     []string `json:"notePath"`
	Title        string   `json:"title"`
	NotePosition int      `json:"notePosition"`
	Prefix       *string  `json:"prefix"`
	IsExpanded   bool     `json:"isExpanded"`
	Type         string   `json:"type"`
	Mime         string   `json:"mime"`
	Attributes   []any    `json:"attributes"`
	Format       string   `json:"format"`
	DataFileName string   `json:"dataFileName"`
	Attachments  []any    `json:"attachments"`
}

// markdownZip packs a Markdown file with a !!!meta.json that tells Trilium's
// importer to convert it into a text note.
func markdownZip(name string, md []byte, appVersion string) ([]byte, error) {
	id := randomID(12)
	meta := metaFile{FormatVersion: 2, AppVersion: appVersion, Files: []metaNote{{
		NoteID: id, NotePath: []string{id}, Title: strings.TrimSuffix(name, filepath.Ext(name)), NotePosition: 10,
		Type: "text", Mime: "text/html", Attributes: []any{}, Format: "markdown", DataFileName: name, Attachments: []any{},
	}}}
	mj, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct {
		name string
		data []byte
	}{{"!!!meta.json", mj}, {name, md}} {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func randomID(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func undeleteCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "undelete <noteId>",
		Short:   "Restore a deleted note",
		Long:    "Restore a deleted note (and the subtree deleted with it). Find candidates with: trilium history",
		Example: "  trilium undelete <noteId>",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !ref.ValidID(id) {
				return output.Usage("", "invalid note ID %q", id)
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := c.Undelete(ctx, id); err != nil {
				return err
			}
			n, err := c.GetNote(ctx, id)
			if err != nil {
				return err
			}
			return rt.emit(n, fmt.Sprintf("restored %s %s", n.NoteID, output.Q(n.Title)))
		},
	}
}

func infoCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "info",
		Short:   "Check the connection and show server details",
		Long:    "Check that TRILIUM_URL and TRILIUM_ETAPI_TOKEN work, and show server details.",
		Example: "  trilium info",
		Args:    exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			var (
				wg      sync.WaitGroup
				app     *etapi.AppInfo
				m       *etapi.Metrics
				appErr  error
				metrErr error
			)
			wg.Go(func() { app, appErr = c.AppInfo(ctx) })
			wg.Go(func() { m, metrErr = c.Metrics(ctx) })
			wg.Wait()
			if appErr != nil {
				return appErr
			}
			agent := rt.cfg.Agent
			if rt.JSON {
				v := map[string]any{"server": rt.cfg.Server, "appInfo": app, "auth": "ok", "agent": agent}
				if metrErr == nil {
					v["metrics"] = m
				}
				return output.JSONL(rt.Stdout, v)
			}
			if agent == "" {
				agent = "(not set)"
			}
			notes := "(unavailable on this server version)"
			if metrErr == nil {
				notes = fmt.Sprintf("%d active, %d deleted", m.Database.ActiveNotes, m.Database.DeletedNotes)
			}
			rt.printf("server:  %s (Trilium %s)\nauth:    ok\nnotes:   %s\nagent:   %s\n", rt.cfg.Server, app.AppVersion, notes, agent)
			return nil
		},
	}
}

var backupNameRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

func backupCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "backup [name]",
		Short:   "Create a database backup on the server",
		Long:    "Create a database backup on the Trilium server, named backup-<name>.db (default name: now).",
		Example: "  trilium backup before-cleanup",
		Args:    rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "now"
			if len(args) == 1 {
				name = args[0]
			}
			if !backupNameRE.MatchString(name) {
				return output.Usage("", "backup names may use letters, digits and _ (at most 32)")
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			if err := c.Backup(cmd.Context(), name); err != nil {
				return err
			}
			file := "backup-" + name + ".db"
			return rt.emit(map[string]string{"file": file}, "backup created on server: "+file)
		},
	}
}

func versionCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Show the CLI version",
		Long:    "Show the CLI version.",
		Example: "  trilium version",
		Args:    exactArgs(0),
		RunE: func(*cobra.Command, []string) error {
			if rt.JSON {
				return output.JSONL(rt.Stdout, map[string]string{"version": rt.Version, "commit": rt.Commit})
			}
			rt.println(versionLine(rt))
			return nil
		},
	}
}

// Package cli implements the trilium commands on top of the ETAPI client.
package cli

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/config"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

// Runtime holds everything commands touch outside their own logic, so that
// tests can replace it. Commands never use os.* or time.Now() directly.
type Runtime struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Getenv  func(string) string
	Now     func() time.Time
	Loc     *time.Location
	Version string
	Commit  string
	// HTTPOptions are extra options for the ETAPI client (tests use them).
	HTTPOptions []etapi.Option
	// WriteFile and ReadFile are used for -o and -f.
	WriteFile func(name string, data []byte) error

	JSON   bool
	cfg    *config.Config
	client *etapi.Client
}

// Client returns the ETAPI client, reading the configuration on first use.
func (rt *Runtime) Client() (*etapi.Client, error) {
	if rt.client != nil {
		return rt.client, nil
	}
	cfg, err := config.Load(rt.Getenv)
	if err != nil {
		return nil, err
	}
	opts := append([]etapi.Option{etapi.WithUserAgent("trilium-cli/" + rt.Version)}, rt.HTTPOptions...)
	rt.cfg = cfg
	rt.client = etapi.New(cfg.URL, cfg.Token, opts...)
	return rt.client, nil
}

func (rt *Runtime) now() time.Time { return rt.Now().In(rt.Loc) }

func (rt *Runtime) printf(format string, a ...any) { fmt.Fprintf(rt.Stdout, format, a...) }

func (rt *Runtime) println(s string) { fmt.Fprintln(rt.Stdout, s) }

// info writes an informational line (e.g. "no matches") to stderr.
func (rt *Runtime) info(format string, a ...any) { fmt.Fprintf(rt.Stderr, format+"\n", a...) }

func (rt *Runtime) warn(format string, a ...any) {
	fmt.Fprintf(rt.Stderr, "warning: "+format+"\n", a...)
}

// emit prints a write result: the entity as JSON in --json mode, the text line otherwise.
func (rt *Runtime) emit(v any, text string) error {
	if rt.JSON {
		return output.JSONL(rt.Stdout, v)
	}
	rt.println(text)
	return nil
}

//go:embed help/search.md
var searchHelp string

const rootHelp = `trilium - work with Trilium Notes from the command line

Everyday
  search <query>          Find notes (Trilium search syntax)
  ls [note]               List child notes
  get <note>...           Show a note's details and content
  create <title> [text]   Create a note (default parent: @inbox)
  append <note> [text]    Add content to the end of a note
  edit <note>             Replace exact text in a note
  put <note> [text]       Replace a note's whole content
  rm <note>...            Delete notes

Organize
  update  attr  mv  branch  att (attachments)  rev (revisions)

More
  export  import  history  undelete  info  backup

<note> can be a note ID, "root", or a shortcut: @inbox @today @2026-10-25
Long content: use -f FILE (or -f - for stdin).
Details: trilium help <command>   Search syntax: trilium help search
`

const cmdHelpTemplate = `Usage: {{.UseLine}}

{{.Long}}
{{- if .HasAvailableSubCommands}}

Commands:
{{- range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}
{{- end}}
{{- if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- if .HasExample}}

Examples:
{{.Example}}
{{- end}}
`

// NewRoot builds the command tree.
func NewRoot(rt *Runtime) *cobra.Command {
	root := &cobra.Command{
		Use:           "trilium",
		Short:         "Work with Trilium Notes from the command line",
		Version:       rt.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(rt.Stdout, rootHelp)
			return nil
		},
	}
	root.SetVersionTemplate(versionLine(rt) + "\n")
	root.PersistentFlags().BoolVar(&rt.JSON, "json", false, "print JSON Lines (one ETAPI entity per line)")
	root.SetOut(rt.Stdout)
	root.SetErr(rt.Stderr)
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return output.Usage("see: trilium help "+helpPath(cmd), "%v", err)
	})
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		searchCmd(rt), lsCmd(rt), getCmd(rt), createCmd(rt), appendCmd(rt), editCmd(rt), putCmd(rt), rmCmd(rt),
		updateCmd(rt), attrCmd(rt), addForce(mvCmd(rt)), branchCmd(rt), attCmd(rt), revCmd(rt),
		historyCmd(rt), exportCmd(rt), addForce(importCmd(rt)), undeleteCmd(rt), infoCmd(rt), backupCmd(rt), versionCmd(rt),
	)
	setHelp(root, rt)
	return root
}

func setHelp(root *cobra.Command, rt *Runtime) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.SetUsageTemplate(cmdHelpTemplate)
		c.SetHelpTemplate("{{.UsageString}}")
		c.DisableFlagsInUseLine = true
		c.InitDefaultHelpFlag()
		_ = c.Flags().MarkHidden("help")
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	for _, c := range root.Commands() {
		walk(c)
	}
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c == root {
			fmt.Fprint(rt.Stdout, rootHelp)
			return
		}
		c.SetOut(rt.Stdout)
		defaultHelp(c, args)
	})
	help := &cobra.Command{
		Use:   "help [command | search]",
		Short: "Show help for a command, or the search syntax",
		Long:  "Show help for a command. \"trilium help search\" shows the search syntax.",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "search" {
				fmt.Fprint(rt.Stdout, searchHelp)
				return nil
			}
			c, rest, err := root.Find(args)
			if err != nil || c == nil || len(rest) > 0 {
				return output.Usage("run: trilium help", "unknown help topic %q", strings.Join(args, " "))
			}
			c.HelpFunc()(c, nil)
			return nil
		},
	}
	help.SetUsageTemplate(cmdHelpTemplate)
	root.SetHelpCommand(help)
}

func helpPath(cmd *cobra.Command) string {
	p := strings.TrimPrefix(cmd.CommandPath(), "trilium")
	return strings.TrimSpace(p)
}

func versionLine(rt *Runtime) string {
	commit := rt.Commit
	if commit == "" {
		commit = "unknown"
	}
	return fmt.Sprintf("trilium %s (commit %s)", rt.Version, commit)
}

// Execute runs the CLI and returns the process exit code.
func Execute(ctx context.Context, rt *Runtime, args []string) int {
	root := NewRoot(rt)
	if args == nil {
		args = []string{} // nil would make cobra read os.Args
	}
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return output.ExitOK
	}
	if isCobraUsage(err) {
		err = output.Usage("run: trilium help", "%v", err)
	}
	var (
		text string
		code int
	)
	if rt.JSON {
		text, code = output.ErrorJSON(err)
	} else {
		text, code = output.ErrorText(err)
	}
	fmt.Fprint(rt.Stderr, text)
	return code
}

// isCobraUsage detects errors cobra raises for unknown commands.
func isCobraUsage(err error) bool {
	var oe *output.Error
	if errors.As(err, &oe) {
		return false
	}
	return strings.HasPrefix(err.Error(), "unknown command") || strings.HasPrefix(err.Error(), "unknown flag") ||
		strings.Contains(err.Error(), "flag needs an argument")
}

// Argument validators that return usage errors with a pointer to help.

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return output.Usage("run: trilium help", "unknown command %q", args[0])
	}
	return nil
}

func exactArgs(n int) cobra.PositionalArgs { return rangeArgs(n, n) }

func rangeArgs(lo, hi int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch {
		case len(args) < lo:
			return argCountError(cmd, "missing arguments")
		case hi >= 0 && len(args) > hi:
			return argCountError(cmd, "too many arguments")
		}
		return nil
	}
}

func minArgs(n int) cobra.PositionalArgs { return rangeArgs(n, -1) }

func argCountError(cmd *cobra.Command, msg string) error {
	return output.Usage("usage: "+cmd.UseLine()+"   (more: trilium help "+helpPath(cmd)+")", "%s: %s", cmd.CommandPath(), msg)
}

// changed reports whether the user set a flag explicitly.
func changed(cmd *cobra.Command, name string) bool {
	f := cmd.Flags().Lookup(name)
	return f != nil && f.Changed
}

// groupRun handles a command group called without a valid subcommand.
func groupRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		cmd.HelpFunc()(cmd, nil)
		return nil
	}
	return output.Usage("run: trilium help "+helpPath(cmd), "unknown command %q for %s", args[0], cmd.CommandPath())
}

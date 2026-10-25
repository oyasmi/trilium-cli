package cli

import (
	"context"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oyasmi/trilium-cli/internal/attrspec"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/output"
)

func attrCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attr <note> [ops...]",
		Short: "List or change a note's labels and relations",
		Long: `Without ops, list the note's own attributes (--all adds inherited ones).
Ops use Trilium's notation; quote them for the shell:
  #name=value   set a label (changes the existing one, or adds it)
  +#name=value  add another label with the same name
  -#name        remove all such labels      -#name=value  remove just that one
  ~name=<note>  set a relation (also +~ and -~)
  #name(inheritable)=value   make it inheritable
Values with spaces can be quoted: '#title="a b"'.`,
		Example: `  trilium attr <id>
  trilium attr <id> '#status=done' '-#todo' '~seeAlso=<other-id>'
  trilium attr <id> '+#tag=urgent'`,
		// Flag parsing is done by hand so that ops like '-#todo' are not
		// mistaken for flags.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, raw []string) error {
			args, all, force, help, err := splitAttrArgs(raw)
			if err != nil {
				return err
			}
			if help {
				cmd.HelpFunc()(cmd, nil)
				return nil
			}
			rt.JSON = rt.JSON || slices.Contains(raw, "--json")
			if len(args) == 0 {
				return argCountError(cmd, "missing arguments")
			}
			ops, err := attrspec.ParseAll(args[1:])
			if err != nil {
				return err
			}
			if err := validateRelationTargets(ops); err != nil {
				return err
			}
			c, ts, err := rt.setup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			t := ts[0]
			if len(ops) > 0 {
				if err := guardWrite(t.ID, force); err != nil {
					return err
				}
			}
			n, err := c.GetNote(cmd.Context(), t.ID)
			if err != nil {
				return err
			}
			if len(ops) == 0 {
				return rt.listAttrs(n, all)
			}
			return rt.applyAttrOps(cmd.Context(), c, n, t, ops)
		},
	}
	cmd.Flags().Bool("all", false, "also list inherited attributes")
	cmd.Flags().Bool("force", false, "allow changing Trilium system notes")
	return cmd
}

// splitAttrArgs separates attr's flags from its arguments. Anything starting
// with "-#" or "-~" is an op; "--" ends the flags.
func splitAttrArgs(raw []string) (args []string, all, force, help bool, err error) {
	for i, a := range raw {
		switch a {
		case "--all":
			all = true
		case "--force":
			force = true
		case "--json":
		case "-h", "--help":
			help = true
		case "--":
			return append(args, raw[i+1:]...), all, force, help, nil
		default:
			if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "-#") && !strings.HasPrefix(a, "-~") {
				return nil, false, false, false, output.Usage("see: trilium help attr", "unknown flag: %s", a)
			}
			args = append(args, a)
		}
	}
	return args, all, force, help, nil
}

func (rt *Runtime) listAttrs(n *etapi.Note, all bool) error {
	owned, inherited := output.SplitAttrs(n)
	list := owned
	if all {
		list = append(list, inherited...)
	}
	if len(list) == 0 {
		rt.info("no attributes")
		return nil
	}
	for _, a := range list {
		if rt.JSON {
			if err := output.JSONL(rt.Stdout, a); err != nil {
				return err
			}
			continue
		}
		rt.println(output.AttrLine(a, n.NoteID))
	}
	return nil
}

type removedAttr struct {
	etapi.Attribute
	Removed bool `json:"removed"`
}

func (rt *Runtime) applyAttrOps(ctx context.Context, c *etapi.Client, n *etapi.Note, t target, ops []attrspec.Op) error {
	owned, _ := output.SplitAttrs(n)
	find := func(op attrspec.Op, value string, matchValue bool) []int {
		var idx []int
		for i, a := range owned {
			if a.Type == op.Type && a.Name == op.Name && (!matchValue || a.Value == value) {
				idx = append(idx, i)
			}
		}
		return idx
	}
	create := func(op attrspec.Op, value string) (*etapi.Attribute, error) {
		a, err := c.CreateAttribute(ctx, etapi.AttributeCreate{NoteID: n.NoteID, Type: op.Type, Name: op.Name, Value: value, IsInheritable: op.Inheritable})
		if err == nil {
			owned = append(owned, *a)
		}
		return a, err
	}
	for _, op := range ops {
		value := op.Value
		if op.Type == attrspec.Relation && op.Value != "" {
			r, err := rt.resolve(ctx, c, op.Value)
			if err != nil {
				return err
			}
			value = r.ID
		}
		switch op.Kind {
		case attrspec.Add:
			a, err := create(op, value)
			if err != nil {
				return err
			}
			if err := rt.emit(a, "added "+output.Attr(*a)+" on "+t.String()); err != nil {
				return err
			}
		case attrspec.Set:
			a, err := rt.setAttr(ctx, c, n.NoteID, &owned, find(op, "", false), op, value, create)
			if err != nil {
				return err
			}
			if err := rt.emit(a, "set "+output.Attr(*a)+" on "+t.String()); err != nil {
				return err
			}
		case attrspec.Remove:
			idx := find(op, value, op.HasValue)
			if len(idx) == 0 {
				rt.warn("%s has no %s to remove", t, attrspec.Format(op.Type, op.Name, value, false))
				continue
			}
			var names []string
			var removed []etapi.Attribute
			for _, i := range idx {
				if err := c.DeleteAttribute(ctx, owned[i].AttributeID); err != nil {
					return err
				}
				names = append(names, output.Attr(owned[i]))
				removed = append(removed, owned[i])
			}
			owned = deleteIndexes(owned, idx)
			if rt.JSON {
				for _, a := range removed {
					if err := output.JSONL(rt.Stdout, removedAttr{a, true}); err != nil {
						return err
					}
				}
				continue
			}
			rt.println("removed " + strings.Join(names, " ") + " from " + t.String())
		}
	}
	return nil
}

// setAttr changes the first matching attribute in place when possible.
// Relations and inheritability can't be patched, so those are re-created.
func (rt *Runtime) setAttr(ctx context.Context, c *etapi.Client, noteID string, owned *[]etapi.Attribute, idx []int, op attrspec.Op, value string,
	create func(attrspec.Op, string) (*etapi.Attribute, error)) (*etapi.Attribute, error) {
	if len(idx) == 0 {
		return create(op, value)
	}
	if len(idx) > 1 {
		rt.warn("%s has %d %s attributes; changed the first one", noteID, len(idx), attrspec.Format(op.Type, op.Name, "", false))
	}
	cur := (*owned)[idx[0]]
	switch {
	case cur.IsInheritable != op.Inheritable || (op.Type == attrspec.Relation && cur.Value != value):
		if err := c.DeleteAttribute(ctx, cur.AttributeID); err != nil {
			return nil, err
		}
		*owned = deleteIndexes(*owned, idx[:1])
		return create(op, value)
	case cur.Value != value:
		a, err := c.PatchAttribute(ctx, cur.AttributeID, etapi.AttributePatch{Value: &value})
		if err != nil {
			return nil, err
		}
		(*owned)[idx[0]] = *a
		return a, nil
	}
	return &cur, nil
}

func deleteIndexes(list []etapi.Attribute, idx []int) []etapi.Attribute {
	drop := map[int]bool{}
	for _, i := range idx {
		drop[i] = true
	}
	out := list[:0:0]
	for i, a := range list {
		if !drop[i] {
			out = append(out, a)
		}
	}
	return out
}

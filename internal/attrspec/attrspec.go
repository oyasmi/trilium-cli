// Package attrspec parses and formats Trilium's attribute notation
// (#label=value, ~relation=target) with set/add/remove prefixes.
package attrspec

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is what an operation does.
type Kind int

// Operation kinds.
const (
	Set    Kind = iota // #name=value: change the existing one or create
	Add                // +#name=value: always create another
	Remove             // -#name[=value]: delete matching ones
)

// Attribute types, as used by ETAPI.
const (
	Label    = "label"
	Relation = "relation"
)

// Op is one parsed attribute argument.
type Op struct {
	Kind        Kind
	Type        string // Label or Relation
	Name        string
	Value       string
	HasValue    bool // an "=" was present
	Inheritable bool
}

// SyntaxError describes an argument that could not be parsed.
type SyntaxError struct {
	Arg    string
	Reason string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("invalid attribute %q: %s", e.Arg, e.Reason)
}

var nameRE = regexp.MustCompile(`^[\p{L}\p{N}_:]+$`)

// ValidName reports whether name only uses letters, digits, "_" and ":".
func ValidName(name string) bool { return nameRE.MatchString(name) }

const inheritableSuffix = "(inheritable)"

// Parse parses one argument such as '#status=done', '+#tag=a', '-~rel'
// or '#name(inheritable)="a b"'.
func Parse(arg string) (Op, error) {
	var op Op
	s := arg
	switch {
	case strings.HasPrefix(s, "+"):
		op.Kind, s = Add, s[1:]
	case strings.HasPrefix(s, "-"):
		op.Kind, s = Remove, s[1:]
	}
	switch {
	case strings.HasPrefix(s, "#"):
		op.Type = Label
	case strings.HasPrefix(s, "~"):
		op.Type = Relation
	default:
		return op, &SyntaxError{arg, `must start with "#" (label) or "~" (relation), optionally prefixed by "+" or "-"`}
	}
	s = s[1:]
	head, value, hasValue := strings.Cut(s, "=")
	if name, ok := strings.CutSuffix(head, inheritableSuffix); ok {
		op.Inheritable = true
		head = name
	}
	op.Name = head
	if op.Name == "" {
		return op, &SyntaxError{arg, "missing attribute name"}
	}
	if !ValidName(op.Name) {
		return op, &SyntaxError{arg, "names may only contain letters, digits, '_' and ':'"}
	}
	if hasValue {
		v, err := unquote(value)
		if err != nil {
			return op, &SyntaxError{arg, err.Error()}
		}
		op.Value, op.HasValue = v, true
	}
	if op.Type == Relation && op.Kind != Remove && op.Value == "" {
		return op, &SyntaxError{arg, "a relation needs a target note, e.g. ~" + op.Name + "=<note>"}
	}
	return op, nil
}

// ParseAll parses every argument, stopping at the first error.
func ParseAll(args []string) ([]Op, error) {
	ops := make([]Op, 0, len(args))
	for _, a := range args {
		op, err := Parse(a)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func unquote(v string) (string, error) {
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return v, nil
	}
	inner := v[1 : len(v)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == '\\' {
			if i+1 >= len(inner) {
				return "", fmt.Errorf("dangling backslash in quoted value")
			}
			i++
			switch inner[i] {
			case '"', '\\':
				b.WriteByte(inner[i])
			default:
				return "", fmt.Errorf(`only \" and \\ escapes are allowed in quoted values`)
			}
			continue
		}
		if c == '"' {
			return "", fmt.Errorf(`unescaped '"' inside quoted value (use \")`)
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}

// Format renders an attribute in the same notation Parse accepts (without
// a +/- prefix). Values containing whitespace or quotes are quoted.
func Format(typ, name, value string, inheritable bool) string {
	var b strings.Builder
	if typ == Relation {
		b.WriteByte('~')
	} else {
		b.WriteByte('#')
	}
	b.WriteString(name)
	if inheritable {
		b.WriteString(inheritableSuffix)
	}
	if value != "" {
		b.WriteByte('=')
		b.WriteString(quote(value))
	}
	return b.String()
}

func quote(v string) string {
	if !strings.ContainsAny(v, " \t\n\r\v\f\"") && !strings.ContainsRune(v, ' ') {
		return v
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(v) + `"`
}

// String renders the op including its +/- prefix.
func (o Op) String() string {
	prefix := ""
	switch o.Kind {
	case Add:
		prefix = "+"
	case Remove:
		prefix = "-"
	}
	s := prefix + Format(o.Type, o.Name, o.Value, o.Inheritable)
	if o.HasValue && o.Value == "" {
		s += "="
	}
	return s
}

package attrspec

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Op
		err  bool
	}{
		{in: "#status=done", want: Op{Kind: Set, Type: Label, Name: "status", Value: "done", HasValue: true}},
		{in: "#todo", want: Op{Kind: Set, Type: Label, Name: "todo"}},
		{in: "#todo=", want: Op{Kind: Set, Type: Label, Name: "todo", HasValue: true}},
		{in: "+#tag=b", want: Op{Kind: Add, Type: Label, Name: "tag", Value: "b", HasValue: true}},
		{in: "-#todo", want: Op{Kind: Remove, Type: Label, Name: "todo"}},
		{in: "-#tag=a", want: Op{Kind: Remove, Type: Label, Name: "tag", Value: "a", HasValue: true}},
		{in: "~seeAlso=abc123", want: Op{Kind: Set, Type: Relation, Name: "seeAlso", Value: "abc123", HasValue: true}},
		{in: "+~seeAlso=@today", want: Op{Kind: Add, Type: Relation, Name: "seeAlso", Value: "@today", HasValue: true}},
		{in: "-~seeAlso", want: Op{Kind: Remove, Type: Relation, Name: "seeAlso"}},
		{in: "-~rel=xyz12345", want: Op{Kind: Remove, Type: Relation, Name: "rel", Value: "xyz12345", HasValue: true}},
		{in: "#project(inheritable)=etapi", want: Op{Kind: Set, Type: Label, Name: "project", Value: "etapi", HasValue: true, Inheritable: true}},
		{in: "#archived(inheritable)", want: Op{Kind: Set, Type: Label, Name: "archived", Inheritable: true}},
		{in: "~template(inheritable)=tpl1", want: Op{Kind: Set, Type: Relation, Name: "template", Value: "tpl1", HasValue: true, Inheritable: true}},
		{in: "#状态=完成", want: Op{Kind: Set, Type: Label, Name: "状态", Value: "完成", HasValue: true}},
		{in: "#title=a b", want: Op{Kind: Set, Type: Label, Name: "title", Value: "a b", HasValue: true}},
		{in: `#title="a b"`, want: Op{Kind: Set, Type: Label, Name: "title", Value: "a b", HasValue: true}},
		{in: `#q="say \"hi\""`, want: Op{Kind: Set, Type: Label, Name: "q", Value: `say "hi"`, HasValue: true}},
		{in: `#p="C:\\dir"`, want: Op{Kind: Set, Type: Label, Name: "p", Value: `C:\dir`, HasValue: true}},
		{in: "#expr=a=b", want: Op{Kind: Set, Type: Label, Name: "expr", Value: "a=b", HasValue: true}},
		{in: "#ns:name=1", want: Op{Kind: Set, Type: Label, Name: "ns:name", Value: "1", HasValue: true}},
		{in: "#a_b2=x", want: Op{Kind: Set, Type: Label, Name: "a_b2", Value: "x", HasValue: true}},
		{in: `#e=""`, want: Op{Kind: Set, Type: Label, Name: "e", HasValue: true}},
		{in: `#one="`, want: Op{Kind: Set, Type: Label, Name: "one", Value: `"`, HasValue: true}},
		{in: "#", err: true},
		{in: "#=x", err: true},
		{in: "-#", err: true},
		{in: "status=done", err: true},
		{in: "", err: true},
		{in: "#my label=x", err: true},
		{in: "#bad-name", err: true},
		{in: "#bad.name=1", err: true},
		{in: "~rel", err: true},
		{in: "~rel=", err: true},
		{in: "+~rel", err: true},
		{in: `#q="a"b"`, err: true},
		{in: `#q="a\nb"`, err: true},
		{in: `#q="a\"`, err: true},
		{in: "#x(inheritable", err: true},
	}
	for _, tc := range cases {
		got, err := Parse(tc.in)
		if tc.err {
			var se *SyntaxError
			if !errors.As(err, &se) || se.Error() == "" {
				t.Errorf("%q: want SyntaxError, got %+v %v", tc.in, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		typ, name, value string
		inh              bool
		want             string
	}{
		{Label, "status", "done", false, "#status=done"},
		{Label, "todo", "", false, "#todo"},
		{Label, "title", "a b", false, `#title="a b"`},
		{Label, "q", `say "hi"`, false, `#q="say \"hi\""`},
		{Label, "p", `C:\dir x`, false, `#p="C:\\dir x"`},
		{Label, "p", `C:\dir`, false, `#p=C:\dir`},
		{Label, "x", `"quoted"`, false, `#x="\"quoted\""`},
		{Label, "nl", "a\nb", true, "#nl(inheritable)=\"a\nb\""},
		{Relation, "seeAlso", "abc123", false, "~seeAlso=abc123"},
		{Relation, "template", "tpl", true, "~template(inheritable)=tpl"},
		{Label, "状态", "进行 中", false, `#状态="进行 中"`},
	}
	for _, tc := range cases {
		s := Format(tc.typ, tc.name, tc.value, tc.inh)
		if s != tc.want {
			t.Errorf("Format: got %s, want %s", s, tc.want)
		}
		op, err := Parse(s)
		if err != nil {
			t.Errorf("%s: %v", s, err)
			continue
		}
		if op.Type != tc.typ || op.Name != tc.name || op.Value != tc.value || op.Inheritable != tc.inh {
			t.Errorf("round trip %s: %+v", s, op)
		}
		if again := Format(op.Type, op.Name, op.Value, op.Inheritable); again != s {
			t.Errorf("second format: %s != %s", again, s)
		}
	}
}

func TestParseAllAndString(t *testing.T) {
	ops, err := ParseAll([]string{"#a=1", "+#b", "-~c", "-#d="})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"#a=1", "+#b", "-~c", "-#d="}
	for i, op := range ops {
		if op.String() != want[i] {
			t.Errorf("String: %s != %s", op.String(), want[i])
		}
	}
	if _, err := ParseAll([]string{"#ok", "bad"}); err == nil {
		t.Error("want error")
	}
	if !ValidName("a:b_1") || ValidName("a b") {
		t.Error("ValidName")
	}
}

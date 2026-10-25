package ref

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oyasmi/trilium-cli/internal/etapi"
)

type fakeCal struct{ calls []string }

func (f *fakeCal) note(kind, v string) (*etapi.Note, error) {
	f.calls = append(f.calls, kind+":"+v)
	if v == "fail" {
		return nil, errors.New("boom")
	}
	return &etapi.Note{NoteID: kind + "_" + v}, nil
}
func (f *fakeCal) Inbox(_ context.Context, d string) (*etapi.Note, error) { return f.note("inbox", d) }
func (f *fakeCal) Day(_ context.Context, d string) (*etapi.Note, error)   { return f.note("day", d) }
func (f *fakeCal) Week(_ context.Context, d string) (*etapi.Note, error)  { return f.note("week", d) }
func (f *fakeCal) Month(_ context.Context, d string) (*etapi.Note, error) { return f.note("month", d) }
func (f *fakeCal) Year(_ context.Context, d string) (*etapi.Note, error)  { return f.note("year", d) }

func TestParseAndResolve(t *testing.T) {
	// 00:30 local time on Jan 1st: yesterday is in the previous year.
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 1, 1, 0, 30, 0, 0, loc)
	cases := []struct {
		in, id, call string
	}{
		{"abc123XYZ", "abc123XYZ", ""},
		{"root", "root", ""},
		{"_hidden", "_hidden", ""},
		{"@inbox", "inbox_2026-01-01", "inbox:2026-01-01"},
		{"@today", "day_2026-01-01", "day:2026-01-01"},
		{"@yesterday", "day_2025-12-31", "day:2025-12-31"},
		{"@tomorrow", "day_2026-01-02", "day:2026-01-02"},
		{"@2026-10-25", "day_2026-10-25", "day:2026-10-25"},
		{"@2024-02-29", "day_2024-02-29", "day:2024-02-29"},
		{"@2026-W43", "week_2026-W43", "week:2026-W43"},
		{"@2026-W53", "week_2026-W53", "week:2026-W53"},
		{"@2026-10", "month_2026-10", "month:2026-10"},
		{"@2026", "year_2026", "year:2026"},
	}
	for _, tc := range cases {
		r, err := Parse(tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if r.IsAlias() != (tc.in[0] == '@') {
			t.Errorf("%s: IsAlias", tc.in)
		}
		f := &fakeCal{}
		id, err := r.Resolve(t.Context(), f, now)
		if err != nil || id != tc.id {
			t.Errorf("%s: got %q %v, want %q", tc.in, id, err, tc.id)
		}
		if tc.call != "" && (len(f.calls) != 1 || f.calls[0] != tc.call) {
			t.Errorf("%s: calls %v", tc.in, f.calls)
		}
		if tc.call == "" && len(f.calls) != 0 {
			t.Errorf("%s: unexpected calls %v", tc.in, f.calls)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "abc", "has space", "a-b-c-d", "@", "@2026-13-45", "@2026-13", "@2026-00", "@2025-02-29", "@2025-W53", "@2026-W00", "@someday", "@26-10-25", "@2026-1-5", "x23456789012345678901234567890123"} {
		_, err := Parse(in)
		var re *Error
		if !errors.As(err, &re) || re.Error() == "" {
			t.Errorf("%q: want *Error, got %v", in, err)
		}
	}
}

func TestResolveError(t *testing.T) {
	r := Ref{Raw: "@x", Kind: Day, Value: "fail"}
	if _, err := r.Resolve(t.Context(), &fakeCal{}, time.Now()); err == nil {
		t.Fatal("want error")
	}
}

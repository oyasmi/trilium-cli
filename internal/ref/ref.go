// Package ref parses note references: note IDs, "root" and @shortcuts
// such as @inbox, @today, @2026-10-25, @2026-W43, @2026-10 and @2026.
package ref

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/oyasmi/trilium-cli/internal/etapi"
)

// Kind tells how a reference is resolved.
type Kind int

// Reference kinds.
const (
	ID    Kind = iota // a plain note ID (including "root")
	Inbox             // @inbox
	Day               // @today, @yesterday, @tomorrow, @YYYY-MM-DD
	Week              // @YYYY-Www
	Month             // @YYYY-MM
	Year              // @YYYY
)

// Ref is a parsed note reference.
type Ref struct {
	Raw  string // as typed by the user
	Kind Kind
	// Value is the note ID for Kind ID, the date string for calendar kinds,
	// and a day offset name ("today", "yesterday", "tomorrow") for relative days.
	Value string
}

// Error is a malformed reference; the CLI treats it as a usage error.
type Error struct {
	Raw    string
	Reason string
}

func (e *Error) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid note reference %q: %s", e.Raw, e.Reason)
	}
	return fmt.Sprintf("invalid note reference %q: use a note ID, \"root\", or a shortcut like @inbox, @today, @2026-10-25", e.Raw)
}

var (
	idRE    = regexp.MustCompile(`^[A-Za-z0-9_]{4,32}$`)
	dayRE   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	weekRE  = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)
	monthRE = regexp.MustCompile(`^\d{4}-(\d{2})$`)
	yearRE  = regexp.MustCompile(`^\d{4}$`)
)

// ValidID reports whether s looks like a Trilium entity ID.
func ValidID(s string) bool { return idRE.MatchString(s) }

// Parse validates s without contacting the server.
func Parse(s string) (Ref, error) {
	r := Ref{Raw: s}
	if s == "" {
		return r, &Error{Raw: s, Reason: "empty"}
	}
	if s[0] != '@' {
		if !ValidID(s) {
			return r, &Error{Raw: s}
		}
		r.Kind, r.Value = ID, s
		return r, nil
	}
	a := s[1:]
	switch {
	case a == "inbox":
		r.Kind = Inbox
	case a == "today" || a == "yesterday" || a == "tomorrow":
		r.Kind, r.Value = Day, a
	case dayRE.MatchString(a):
		if _, err := time.Parse(time.DateOnly, a); err != nil {
			return r, &Error{Raw: s, Reason: "not a real date (use @YYYY-MM-DD)"}
		}
		r.Kind, r.Value = Day, a
	case weekRE.MatchString(a):
		m := weekRE.FindStringSubmatch(a)
		y, _ := strconv.Atoi(m[1])
		w, _ := strconv.Atoi(m[2])
		if w < 1 || w > isoWeeksInYear(y) {
			return r, &Error{Raw: s, Reason: fmt.Sprintf("year %d has no ISO week %02d", y, w)}
		}
		r.Kind, r.Value = Week, a
	case monthRE.MatchString(a):
		mo, _ := strconv.Atoi(monthRE.FindStringSubmatch(a)[1])
		if mo < 1 || mo > 12 {
			return r, &Error{Raw: s, Reason: "month must be 01-12"}
		}
		r.Kind, r.Value = Month, a
	case yearRE.MatchString(a):
		r.Kind, r.Value = Year, a
	default:
		return r, &Error{Raw: s, Reason: "unknown shortcut; use @inbox, @today, @yesterday, @tomorrow, @YYYY-MM-DD, @YYYY-Www, @YYYY-MM or @YYYY"}
	}
	return r, nil
}

func isoWeeksInYear(y int) int {
	_, w := time.Date(y, 12, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
	return w
}

// IsAlias reports whether the reference is an @shortcut.
func (r Ref) IsAlias() bool { return r.Kind != ID }

// Calendar is the subset of the ETAPI client used to resolve shortcuts.
type Calendar interface {
	Inbox(ctx context.Context, date string) (*etapi.Note, error)
	Day(ctx context.Context, date string) (*etapi.Note, error)
	Week(ctx context.Context, week string) (*etapi.Note, error)
	Month(ctx context.Context, month string) (*etapi.Note, error)
	Year(ctx context.Context, year string) (*etapi.Note, error)
}

// Resolve returns the note ID. now must be in the local time zone; relative
// days are computed from it. Calendar notes are created by the server when
// they do not exist yet.
func (r Ref) Resolve(ctx context.Context, cal Calendar, now time.Time) (string, error) {
	var (
		n   *etapi.Note
		err error
	)
	switch r.Kind {
	case ID:
		return r.Value, nil
	case Inbox:
		n, err = cal.Inbox(ctx, now.Format(time.DateOnly))
	case Day:
		n, err = cal.Day(ctx, r.date(now))
	case Week:
		n, err = cal.Week(ctx, r.Value)
	case Month:
		n, err = cal.Month(ctx, r.Value)
	case Year:
		n, err = cal.Year(ctx, r.Value)
	}
	if err != nil {
		return "", err
	}
	return n.NoteID, nil
}

func (r Ref) date(now time.Time) string {
	switch r.Value {
	case "today":
		return now.Format(time.DateOnly)
	case "yesterday":
		return now.AddDate(0, 0, -1).Format(time.DateOnly)
	case "tomorrow":
		return now.AddDate(0, 0, 1).Format(time.DateOnly)
	}
	return r.Value
}

// Package content handles note content: where it is read from, how its type
// is inferred, whether it is binary, and the read-only plain-text view.
package content

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
)

// Source tells where content came from.
type Source int

// Content sources.
const (
	None  Source = iota // nothing given
	Arg                 // positional argument
	File                // -f FILE
	Stdin               // -f -
)

// Input describes the content options of one command invocation.
type Input struct {
	Arg    string
	HasArg bool
	File   string // "" = not given, "-" = stdin
}

// ErrTooManySources is returned when more than one source is given.
var ErrTooManySources = errors.New("give the content only one way: as an argument, with -f FILE, or with -f - (stdin)")

// Read returns the content bytes exactly as given. stdin is only read for
// "-f -"; it is never read implicitly.
func Read(in Input, stdin io.Reader) ([]byte, Source, error) {
	if in.HasArg && in.File != "" {
		return nil, None, ErrTooManySources
	}
	switch {
	case in.HasArg:
		return []byte(in.Arg), Arg, nil
	case in.File == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, Stdin, fmt.Errorf("reading stdin: %w", err)
		}
		return b, Stdin, nil
	case in.File != "":
		b, err := os.ReadFile(in.File)
		if err != nil {
			return nil, File, err
		}
		return b, File, nil
	}
	return nil, None, nil
}

var tagRE = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9-]*(\s[^<>]*)?/?>`)

// HasHTMLTags reports whether s contains at least one HTML tag.
func HasHTMLTags(s string) bool { return tagRE.MatchString(s) }

package output

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oyasmi/trilium-cli/internal/attrspec"
	"github.com/oyasmi/trilium-cli/internal/config"
	"github.com/oyasmi/trilium-cli/internal/content"
	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/ref"
)

// Exit codes.
const (
	ExitOK       = 0
	ExitFailure  = 1 // API, network or operation failure
	ExitUsage    = 2 // bad arguments or configuration
	ExitNotFound = 3 // *_NOT_FOUND
)

// Error is an error raised by the CLI itself, with an optional hint and exit code.
type Error struct {
	Msg  string
	Hint string
	Exit int
}

func (e *Error) Error() string { return e.Msg }

// Usage returns an error that exits with code 2.
func Usage(hint, format string, a ...any) *Error {
	return &Error{Msg: fmt.Sprintf(format, a...), Hint: hint, Exit: ExitUsage}
}

// Fail returns an error that exits with code 1.
func Fail(hint, format string, a ...any) *Error {
	return &Error{Msg: fmt.Sprintf(format, a...), Hint: hint, Exit: ExitFailure}
}

// NotFound returns an error that exits with code 3.
func NotFound(hint, format string, a ...any) *Error {
	return &Error{Msg: fmt.Sprintf(format, a...), Hint: hint, Exit: ExitNotFound}
}

// Hints shared by several commands.
const (
	HintSearch = "find note IDs with: trilium search <words>"
	HintAuth   = "check TRILIUM_ETAPI_TOKEN; tokens are created in Trilium under Options → ETAPI"
)

var notFoundHints = map[string]string{
	"ATTACHMENT_NOT_FOUND": "list a note's attachments with: trilium att ls <note>",
	"REVISION_NOT_FOUND":   "list a note's revisions with: trilium rev ls <note>",
	"BRANCH_NOT_FOUND":     "list a note's placements with: trilium branch ls <note>",
	"ATTRIBUTE_NOT_FOUND":  "list a note's attributes with: trilium attr <note>",
}

// Describe maps any error to its message, hint and exit code.
func Describe(err error) (msg, hint string, code int) {
	var (
		oe *Error
		ae *etapi.Error
		ne *etapi.NetworkError
		ce *config.Error
		re *ref.Error
		se *attrspec.SyntaxError
	)
	switch {
	case errors.As(err, &oe):
		return oe.Msg, oe.Hint, oe.Exit
	case errors.As(err, &ae):
		return describeAPI(ae)
	case errors.As(err, &ne):
		hint := fmt.Sprintf("could not reach TRILIUM_URL (%s)", strings.TrimSuffix(ne.URL, "/etapi"))
		if ne.MaybeApplied {
			hint = "the connection failed after the request was sent, so the change may have been applied; check before retrying (e.g. trilium get -c <id>)"
		}
		return ne.Err.Error(), hint, ExitFailure
	case errors.As(err, &ce):
		return ce.Msg, ce.Hint, ExitUsage
	case errors.As(err, &re):
		return re.Error(), "", ExitUsage
	case errors.As(err, &se):
		return se.Error(), `examples: '#status=done'  '+#tag=x'  '-#todo'  '~seeAlso=<note>'`, ExitUsage
	case errors.Is(err, content.ErrTooManySources):
		return err.Error(), "", ExitUsage
	case errors.Is(err, context.DeadlineExceeded):
		return "the server did not answer in time", "", ExitFailure
	}
	return err.Error(), "", ExitFailure
}

func describeAPI(e *etapi.Error) (string, string, int) {
	msg := e.Error()
	switch {
	case e.NotFound():
		if h, ok := notFoundHints[e.Code]; ok {
			return msg, h, ExitNotFound
		}
		return msg, HintSearch, ExitNotFound
	case e.Code == "NOT_AUTHENTICATED" || e.Status == 401:
		return msg, HintAuth, ExitFailure
	case e.Code == "NOTE_IS_PROTECTED":
		return msg, "protected notes can't be read or changed through ETAPI", ExitFailure
	case e.Status == 429:
		return msg, "too many failed attempts; wait a few minutes", ExitFailure
	}
	return msg, "", ExitFailure
}

// ErrorText renders an error for stderr in text mode.
func ErrorText(err error) (string, int) {
	msg, hint, code := Describe(err)
	s := "error: " + msg + "\n"
	if hint != "" {
		s += "hint: " + hint + "\n"
	}
	return s, code
}

type jsonError struct {
	Error struct {
		Status  int    `json:"status,omitempty"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message"`
	} `json:"error"`
	Hint string `json:"hint,omitempty"`
}

// ErrorJSON renders an error for stderr in --json mode.
func ErrorJSON(err error) (string, int) {
	msg, hint, code := Describe(err)
	var j jsonError
	j.Error.Message = msg
	j.Hint = hint
	var ae *etapi.Error
	if errors.As(err, &ae) {
		j.Error.Status, j.Error.Code, j.Error.Message = ae.Status, ae.Code, ae.Message
	}
	var b strings.Builder
	_ = JSONL(&b, j)
	return b.String(), code
}

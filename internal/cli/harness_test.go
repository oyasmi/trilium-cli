package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oyasmi/trilium-cli/internal/etapi"
	"github.com/oyasmi/trilium-cli/internal/etapi/etapitest"
	"github.com/oyasmi/trilium-cli/internal/testutil"
)

var testLoc = time.FixedZone("UTC+8", 8*3600)

// session runs CLI commands against a fake server and records a transcript.
type session struct {
	t     *testing.T
	srv   *etapitest.Server
	env   map[string]string
	files map[string][]byte
	dir   string
	log   strings.Builder
}

func newSession(t *testing.T) *session {
	t.Helper()
	srv := etapitest.New()
	t.Cleanup(srv.Close)
	return &session{
		t: t, srv: srv,
		env:   map[string]string{"TRILIUM_URL": srv.URL, "TRILIUM_ETAPI_TOKEN": etapitest.Token},
		files: map[string][]byte{},
		dir:   t.TempDir(),
	}
}

// file writes a fixture file and returns its path.
func (s *session) file(name string, data []byte) string {
	p := filepath.Join(s.dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		s.t.Fatal(err)
	}
	return p
}

// exec runs one command and returns stdout, stderr and the exit code.
func (s *session) exec(stdin string, args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	rt := &Runtime{
		Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errb,
		Getenv:      func(k string) string { return s.env[k] },
		Now:         func() time.Time { return time.Date(2026, 10, 25, 12, 0, 0, 0, testLoc) },
		Loc:         testLoc,
		Version:     "1.2.3",
		Commit:      "abc1234",
		HTTPOptions: []etapi.Option{etapi.WithRetryDelay(time.Millisecond)},
		WriteFile: func(name string, data []byte) error {
			s.files[name] = bytes.Clone(data)
			return nil
		},
	}
	code := Execute(s.t.Context(), rt, args)
	return out.String(), errb.String(), code
}

// run executes a command and appends it to the transcript.
func (s *session) run(args ...string) (string, int) { return s.runIn("", args...) }

func (s *session) runIn(stdin string, args ...string) (string, int) {
	out, errs, code := s.exec(stdin, args...)
	out = strings.ReplaceAll(out, s.srv.URL, "http://trilium.test")
	fmt.Fprintf(&s.log, "$ trilium %s\n", shellJoin(s.redact(args)))
	s.log.WriteString(out)
	if out != "" && !strings.HasSuffix(out, "\n") {
		s.log.WriteString("%(no newline)\n")
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(errs, "\n"), "\n") {
		if line != "" {
			s.log.WriteString("stderr| " + line + "\n")
		}
	}
	if code != 0 {
		fmt.Fprintf(&s.log, "exit: %d\n", code)
	}
	s.log.WriteString("\n")
	return out, code
}

func (s *session) redact(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(a, s.dir, "$DIR")
	}
	return out
}

func shellJoin(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\n\"'#~<>|&;*?()$\\!") {
			q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			q[i] = a
		}
	}
	return strings.Join(q, " ")
}

// golden compares the transcript with testdata/<name>.golden.
func (s *session) golden(name string) {
	s.t.Helper()
	testutil.Golden(s.t, name, s.log.String())
}

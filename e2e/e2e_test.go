// Package e2e runs the compiled CLI against a real Trilium server.
//
// It only runs with TRILIUM_E2E=1 and uses TRILIUM_URL / TRILIUM_ETAPI_TOKEN.
// Everything is written below a sandbox note that is deleted afterwards; the
// run fails if root's children differ from before or can't be read. The only
// exception is the calendar test: it runs only when the server has no
// calendar yet, and deletes the Calendar root it created. backup is never
// called (it leaves files on the server that the API can't remove).
package e2e

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

var (
	bin     string // compiled CLI
	sandbox string // note ID of the sandbox
	workDir string
	// extra holds notes outside the sandbox that this run created and must
	// delete (only a Calendar root created by the calendar test).
	extra []string
)

func TestMain(m *testing.M) {
	if os.Getenv("TRILIUM_E2E") != "1" {
		fmt.Println("skipping e2e tests (set TRILIUM_E2E=1, TRILIUM_URL and TRILIUM_ETAPI_TOKEN)")
		os.Exit(0)
	}
	os.Exit(run(m))
}

func run(m *testing.M) (code int) {
	var err error
	workDir, err = os.MkdirTemp("", "trilium-e2e-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()
	bin = filepath.Join(workDir, "trilium")
	build := exec.Command("go", "build", "-o", bin, "../cmd/trilium")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Println("build failed:", err)
		return 1
	}

	before, err := rootChildren()
	if err != nil {
		fmt.Println("can't read root's children, not touching the server:", err)
		return 1
	}
	title := "trilium-cli-e2e-" + randHex(4)
	out, errs, c := cli("", "create", title, "-p", "root", "-t", "book")
	if c != 0 {
		fmt.Println("can't create sandbox:", errs)
		return 1
	}
	sandbox = strings.Fields(out)[1]
	fmt.Println("sandbox:", sandbox, title)

	defer func() {
		if _, errs, c := cli("", "rm", sandbox, "-r"); c != 0 {
			fmt.Println("cleanup failed:", errs)
			code = 1
		}
		for _, id := range extra {
			if _, errs, c := cli("", "rm", id, "-r"); c != 0 {
				fmt.Println("cleanup failed:", errs)
				code = 1
			}
		}
		after, err := rootChildren()
		if err != nil {
			fmt.Println("can't verify root's children after the run:", err)
			code = 1
		} else if !slices.Equal(before, after) {
			fmt.Printf("root children changed!\nbefore: %v\nafter:  %v\n", before, after)
			code = 1
		}
	}()
	return m.Run()
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// cli runs the binary with optional stdin.
func cli(stdin string, args ...string) (string, string, int) {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "TRILIUM_AGENT=")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return out.String(), errb.String(), code
}

func rootChildren() ([]string, error) {
	out, errs, code := cli("", "--json", "get", "root", "-m")
	if code != 0 {
		return nil, fmt.Errorf("exit %d: %s", code, errs)
	}
	var n struct {
		ChildNoteIDs []string `json:"childNoteIds"`
	}
	if err := json.Unmarshal([]byte(out), &n); err != nil {
		return nil, err
	}
	if n.ChildNoteIDs == nil {
		return nil, fmt.Errorf("no childNoteIds in %q", out)
	}
	return n.ChildNoteIDs, nil
}

// calendarRoots returns the IDs of notes labelled #calendarRoot.
func calendarRoots() ([]string, error) {
	out, errs, code := cli("", "search", "#calendarRoot", "--archived", "-n", "0")
	if code != 0 {
		return nil, fmt.Errorf("exit %d: %s", code, errs)
	}
	var ids []string
	for line := range strings.Lines(out) {
		ids = append(ids, strings.Fields(line)[0])
	}
	return ids, nil
}

// ok runs a command that must succeed and returns stdout.
func ok(t *testing.T, args ...string) string {
	t.Helper()
	return okIn(t, "", args...)
}

func okIn(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	out, errs, code := cli(stdin, args...)
	if code != 0 {
		t.Fatalf("trilium %s: exit %d\n%s", strings.Join(args, " "), code, errs)
	}
	return out
}

// fail runs a command that must fail with the given exit code; returns stderr.
func fail(t *testing.T, want int, args ...string) string {
	t.Helper()
	_, errs, code := cli("", args...)
	if code != want {
		t.Fatalf("trilium %s: exit %d, want %d\n%s", strings.Join(args, " "), code, want, errs)
	}
	return errs
}

// id returns the second field of a result line ("created <id> …").
func id(t *testing.T, line string) string {
	t.Helper()
	f := strings.Fields(line)
	if len(f) < 2 {
		t.Fatalf("no id in %q", line)
	}
	return f[1]
}

// folder creates a fresh book under the sandbox for one test.
func folder(t *testing.T) string {
	t.Helper()
	return id(t, ok(t, "create", t.Name(), "-p", sandbox, "-t", "book"))
}

func file(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("missing %q in:\n%s", sub, s)
	}
}

func TestInfo(t *testing.T) {
	out := ok(t, "info")
	contains(t, out, "auth:    ok")
	contains(t, out, "agent:   (not set)")
}

func TestEveryday(t *testing.T) {
	f := folder(t)
	n := id(t, ok(t, "create", "Meeting notes", "<p>Agenda</p>", "-p", f, "-a", "#status=draft", "-a", "~seeAlso="+f))
	py := id(t, ok(t, "create", "-f", file(t, "hello.py", []byte("print('hi')\n")), "-p", f))
	empty := id(t, ok(t, "create", "Empty", "-p", f)) // create-note always sends content
	contains(t, ok(t, "get", py, "-m"), "type: code (text/x-python)")

	contains(t, ok(t, "search", "#status=draft", "--in", f), n)
	contains(t, ok(t, "ls", f), "Empty")

	ok(t, "append", n, "<p>Item</p>")
	ok(t, "append", py, "print('bye')")
	ok(t, "edit", n, "--old", "<p>Agenda</p>", "--new", "<p>Plan</p>")
	if got := ok(t, "get", n, "-c"); got != "<p>Plan</p><p>Item</p>" {
		t.Errorf("content %q", got)
	}
	if got := ok(t, "get", py, "-c"); got != "print('hi')\nprint('bye')" {
		t.Errorf("code content %q", got)
	}
	fail(t, 1, "edit", n, "--old", "nope", "--new", "x")
	out := ok(t, "get", n, py, "--text")
	contains(t, out, "content: plain-text view")
	contains(t, out, "Plan\nItem")
	contains(t, ok(t, "put", empty, "<p>full</p>"), "updated "+empty+" content: 0 → 11 bytes")
	contains(t, ok(t, "rm", empty), "undo: trilium undelete")
	fail(t, 3, "get", empty)
	contains(t, ok(t, "undelete", empty), "restored")
}

func TestAgentLabel(t *testing.T) {
	f := folder(t)
	cmd := exec.Command(bin, "create", "By agent", "<p>x</p>", "-p", f)
	cmd.Env = append(os.Environ(), "TRILIUM_AGENT=e2e-agent")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	contains(t, ok(t, "attr", id(t, string(out))), "#agent=e2e-agent")
}

func TestLargeContentGzip(t *testing.T) {
	f := folder(t)
	body := "<p>" + strings.Repeat("Large content to force a gzip response. ", 200) + "中文</p>"
	n := id(t, ok(t, "create", "Large", "-f", file(t, "large.html", []byte(body)), "-p", f))
	if got := ok(t, "get", n, "-c"); got != body {
		t.Errorf("large content differs (%d vs %d bytes)", len(got), len(body))
	}
}

func TestSearchQuoting(t *testing.T) {
	f := folder(t)
	ok(t, "create", "hello.py", "-t", "code", "-p", f)
	_, errs, code := cli("", "search", "note.title=hello.py", "--in", f)
	if code != 0 || !strings.Contains(errs, `must be quoted`) {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
	contains(t, ok(t, "search", `note.title="hello.py"`, "--in", f), "hello.py")
}

func TestJSONContentIsNotSentAsJSON(t *testing.T) {
	f := folder(t)
	body := `{"a": [1, 2, 3]}`
	n := id(t, ok(t, "create", "-f", file(t, "data.json", []byte(body)), "-p", f))
	contains(t, ok(t, "get", n, "-m"), "type: code (application/json)")
	ok(t, "put", n, `{"b": true}`)
	if got := ok(t, "get", n, "-c"); got != `{"b": true}` {
		t.Errorf("got %q", got)
	}
}

func TestOrganize(t *testing.T) {
	f := folder(t)
	a := id(t, ok(t, "create", "A", "-p", f, "-t", "book"))
	b := id(t, ok(t, "create", "B", "<p>b</p>", "-p", a))
	c := id(t, ok(t, "create", "C", "-p", f, "-t", "book"))

	// Cycle detection: the server would accept these.
	contains(t, fail(t, 1, "branch", "add", a, b), "loop")
	contains(t, fail(t, 1, "mv", a, b), "loop")
	if out := ok(t, "branch", "ls", a); strings.Count(out, "\n") != 1 {
		t.Errorf("a cycle branch was created:\n%s", out)
	}

	contains(t, ok(t, "mv", b, c), "moved "+b)
	contains(t, ok(t, "branch", "add", b, a, "--prefix", "x: "), "cloned "+b+" into "+a)
	contains(t, ok(t, "branch", "add", b, a, "--pos", "5"), "updated existing branch")
	contains(t, ok(t, "branch", "set", b, a, "--pos", "1", "--prefix", ""), "position 1")
	contains(t, ok(t, "rm", b, "-r"), "was in 2 places")
	ok(t, "undelete", b)
	ok(t, "branch", "rm", b, a)
	contains(t, fail(t, 2, "branch", "rm", b, c), "only place")

	ok(t, "update", b, "--title", "B2")
	ok(t, "attr", b, "#status=todo", "+#tag=x", "+#tag=y", "-#tag=x", "#p(inheritable)=1", "~rel="+a)
	out := ok(t, "attr", b)
	for _, want := range []string{"#status=todo", "#tag=y", "#p(inheritable)=1", "~rel=" + a} {
		contains(t, out, want)
	}
	if strings.Contains(out, "#tag=x") {
		t.Error("#tag=x not removed")
	}
	ok(t, "attr", b, "~rel="+c, "#status=done")
	out = ok(t, "attr", b)
	contains(t, out, "~rel="+c)
	contains(t, out, "#status=done")
	contains(t, fail(t, 2, "attr", b, "#bad name"), "names may only contain")

	ok(t, "rev", "add", b)
	ok(t, "put", b, "<p>new</p>")
	revs := ok(t, "rev", "ls", b)
	rev := strings.Fields(revs)[0]
	contains(t, ok(t, "rev", "get", rev), "<p>b</p>")

	contains(t, ok(t, "ls", f, "-d", "2"), "  "+b)
	contains(t, ok(t, "history", "--in", f), b)
}

func TestBinaryRoundTrip(t *testing.T) {
	f := folder(t)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\xcf\xc0\x00\x00\x03\x01\x01\x00\xc9\xfe\x92\xef\x00\x00\x00\x00IEND\xaeB`\x82")
	random := make([]byte, 4096)
	_, _ = rand.Read(random)
	for name, data := range map[string][]byte{"red.png": png, "blob.bin": random} {
		src := file(t, name, data)
		n := id(t, ok(t, "create", "-f", src, "-p", f))
		contains(t, ok(t, "get", n), "content: binary")
		fail(t, 1, "get", n, "-c")
		dst := filepath.Join(t.TempDir(), "out")
		ok(t, "get", n, "-o", dst)
		if got, _ := os.ReadFile(dst); !bytes.Equal(got, data) {
			t.Errorf("%s: note round trip differs", name)
		}
		out := ok(t, "att", "add", n, "-f", src)
		att := strings.Fields(out)[2]
		if name == "red.png" {
			contains(t, out, `embed: <img src="api/attachments/`+att+`/image/red.png">`)
		}
		ok(t, "att", "get", att, "-o", dst)
		if got, _ := os.ReadFile(dst); !bytes.Equal(got, data) {
			t.Errorf("%s: attachment round trip differs", name)
		}
		ok(t, "att", "rm", att)
	}
}

// TestConcurrentAppend checks what append promises: a writer that loses its
// update finds out in most interleavings. ETAPI has no conditional write, so
// a loss that neither writer can see is still possible; that case is logged,
// not failed.
func TestConcurrentAppend(t *testing.T) {
	f := folder(t)
	n := id(t, ok(t, "create", "Log", "<p>start</p>", "-p", f))
	pieces := []string{"<p>one</p>", "<p>two</p>", "<p>three</p>", "<p>four</p>"}
	codes := make([]int, len(pieces))
	var wg sync.WaitGroup
	for i, piece := range pieces {
		wg.Go(func() { _, _, codes[i] = cli("", "append", n, piece) })
	}
	wg.Wait()
	got := ok(t, "get", n, "-c")
	contains(t, got, "<p>start</p>")
	for i, piece := range pieces {
		switch {
		case strings.Contains(got, piece):
		case codes[i] != 0:
			t.Logf("append %s reported failure (exit %d) and is missing: detected conflict", piece, codes[i])
		default:
			t.Logf("append %s succeeded but is missing: undetectable lost update (known limitation)", piece)
		}
	}
}

func TestCreateExistingID(t *testing.T) {
	f := folder(t)
	stable := "e2eStable" + randHex(4)
	ok(t, "create", "Stable", "<p>once</p>", "-p", f, "--id", stable, "-a", "#v=1")
	contains(t, fail(t, 1, "create", "Again", "<p>twice</p>", "-p", f, "--id", stable), "already exists")
	out := ok(t, "get", stable)
	contains(t, out, "title: Stable\n")
	contains(t, out, "attrs: #v=1\n")
	contains(t, out, "<p>once</p>")
}

func TestAttrKeepsOldOnFailure(t *testing.T) {
	f := folder(t)
	n := id(t, ok(t, "create", "Rel", "-p", f))
	ok(t, "attr", n, "~target="+f)
	fail(t, 3, "attr", n, "~target=doesNotExist1")
	contains(t, ok(t, "attr", n), "~target="+f)
	other := id(t, ok(t, "create", "Other", "-p", f))
	ok(t, "attr", n, "~target="+other, "#p(inheritable)=1")
	ok(t, "attr", n, "#p=1")
	out := ok(t, "attr", n)
	if strings.Contains(out, "~target="+f) || !strings.Contains(out, "~target="+other) || strings.Contains(out, "(inheritable)") {
		t.Errorf("replacements:\n%s", out)
	}
}

func TestEditChain(t *testing.T) {
	f := folder(t)
	n := id(t, ok(t, "create", "c.txt", "A", "-t", "code", "-p", f))
	contains(t, ok(t, "edit", n, "--old", "A", "--new", "B", "--old", "B", "--new", "C"), "2 replacements")
	if got := ok(t, "get", n, "-c"); got != "C" {
		t.Errorf("got %q", got)
	}
}

func TestImportExport(t *testing.T) {
	f := folder(t)
	target := id(t, ok(t, "create", "Target", "<p>t</p>", "-p", f))
	md := "# Heading\n\nSome **bold** and a [link](#root/" + target + ").\n\n- [ ] task\n"
	out := ok(t, "import", f, file(t, "Report.md", []byte(md)))
	n := id(t, out)
	html := ok(t, "get", n, "-c")
	for _, want := range []string{"<h2>Heading</h2>", "<strong>bold</strong>", `href="#root/` + target + `"`, `class="todo-list"`} {
		contains(t, html, want)
	}
	contains(t, ok(t, "get", n, "--text"), "link ["+target+"]")

	zip := filepath.Join(t.TempDir(), "export.zip")
	contains(t, ok(t, "export", f, "-o", zip), "exported "+f)
	sub := id(t, ok(t, "create", "Imported", "-p", f, "-t", "book"))
	contains(t, ok(t, "import", sub, zip), "imported")
	contains(t, ok(t, "ls", sub, "-d", "3"), "Report")
}

func TestCalendarShortcut(t *testing.T) {
	// @today writes outside the sandbox. Only run when there is no calendar
	// yet, so that everything it creates can be removed again.
	roots, err := calendarRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) > 0 {
		t.Skip("this server already has a calendar; skipping so no day note is added to it")
	}
	out := ok(t, "get", "@today", "-m")
	contains(t, out, "id: ")
	roots, err = calendarRoots()
	if err != nil || len(roots) != 1 {
		t.Fatalf("expected the calendar root created by @today: %v %v", roots, err)
	}
	extra = append(extra, roots[0])
	contains(t, fail(t, 2, "get", "@2026-13-45"), "not a real date")
}

func TestErrors(t *testing.T) {
	contains(t, fail(t, 3, "get", "doesNotExist1"), "NOTE_NOT_FOUND")
	contains(t, fail(t, 2, "rm", "root"), "system note")
	contains(t, fail(t, 2, "put", "_hidden", "x"), "--force")
	cmd := exec.Command(bin, "info")
	cmd.Env = append(os.Environ(), "TRILIUM_ETAPI_TOKEN=wrong")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err == nil || !strings.Contains(errb.String(), "TRILIUM_ETAPI_TOKEN") {
		t.Errorf("bad token: %v %s", err, errb.String())
	}
}

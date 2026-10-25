package cli

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/oyasmi/trilium-cli/internal/etapi/etapitest"
)

// seed builds a small tree:
//
//	root
//	  Projects (book, #project=etapi inheritable)
//	    Meeting notes (text, #status=draft, ~seeAlso=Script)
//	    Script.py (code)
//	    Old (archived)
//	  Inbox (#inbox)
func seed(s *session) (projects, meeting, script string) {
	srv := s.srv
	projects = srv.Add("projects1", "root", "Projects", "book", "", "")
	meeting = srv.Add("meeting1", projects, "Meeting notes", "text", "text/html", "<h2>Agenda</h2><p>Budget review</p><ul><li>one</li><li>two</li></ul>")
	script = srv.Add("script1", projects, "Script.py", "code", "text/x-python", "print('hi')\n")
	old := srv.Add("old1", projects, "Old", "text", "text/html", "<p>old</p>")
	inbox := srv.Add("inbox1", "root", "Inbox", "text", "text/html", "")
	srv.AddAttr(projects, "label", "project", "etapi", true)
	srv.AddAttr(meeting, "label", "status", "draft", false)
	srv.AddAttr(meeting, "relation", "seeAlso", script, false)
	srv.AddAttr(old, "label", "archived", "", false)
	srv.AddAttr(inbox, "label", "inbox", "", false)
	return projects, meeting, script
}

func TestConfigErrors(t *testing.T) {
	s := newSession(t)
	delete(s.env, "TRILIUM_ETAPI_TOKEN")
	s.run("ls")
	delete(s.env, "TRILIUM_URL")
	s.run("ls")
	s.env["TRILIUM_URL"] = "notes.example.com"
	s.env["TRILIUM_ETAPI_TOKEN"] = "x"
	s.run("ls")
	s.env["TRILIUM_URL"] = s.srv.URL
	s.env["TRILIUM_ETAPI_TOKEN"] = "wrong"
	s.run("ls")
	s.run("--json", "ls")
	s.golden("config")
}

func TestHelpAndVersion(t *testing.T) {
	s := newSession(t)
	s.run()
	s.run("--version")
	s.run("version")
	s.run("help", "get")
	s.run("append", "--help")
	s.run("help", "nope")
	s.run("nope")
	s.run("get")
	s.run("get", "x", "--bogus")
	s.run("get", "bad id!")
	s.golden("help")
	out, _ := s.run("help", "search")
	if !strings.Contains(out, "Quoting rule") {
		t.Error("help search must print the syntax guide")
	}
}

func TestSearch(t *testing.T) {
	s := newSession(t)
	projects, _, _ := seed(s)
	s.run("search", "budget")
	s.run("search", "#status=draft", "-l")
	s.run("search", "#project", "--in", projects)
	s.run("search", "note.noteId", "!=", `""`, "-n", "2")
	s.run("search", "hidden")
	s.run("search", "nothing-here-at-all")
	s.run("search", "note.title=hello.py")
	s.run("--json", "search", "budget")
	s.golden("search")
}

func TestLs(t *testing.T) {
	s := newSession(t)
	projects, meeting, _ := seed(s)
	s.srv.Add("child1", meeting, "Child", "text", "text/html", "")
	s.srv.Add("grandchild1", "child1", "Grandchild", "text", "text/html", "")
	s.run("ls")
	s.run("ls", "--all")
	s.run("ls", projects, "-l")
	s.run("ls", projects, "-d", "3")
	s.run("ls", projects, "-d", "2", "-n", "3")
	s.run("ls", "_hidden")
	s.run("ls", "script1")
	s.run("ls", "missing1")
	s.run("ls", projects, "-d", "0")
	s.run("--json", "ls", projects, "-d", "2", "-n", "2")
	s.golden("ls")
	// Depth 1 needs exactly two requests.
	s.srv.Calls = nil
	s.exec("", "ls", projects)
	if len(s.srv.Calls) != 2 {
		t.Errorf("ls made %d requests: %v", len(s.srv.Calls), s.srv.Calls)
	}
}

func TestGet(t *testing.T) {
	s := newSession(t)
	_, meeting, script := seed(s)
	img := s.srv.Add("img1", "projects1", "red.png", "image", "image/png", "\x89PNG\x00\x01")
	s.srv.AddAttachment(meeting, "image", "image/png", "chart.png", []byte("\x89PNG"))
	s.run("get", meeting)
	s.run("get", meeting, "--all-attrs", "--text")
	s.run("get", meeting, script, "-m")
	s.run("get", script, "-c")
	s.run("get", img)
	s.run("get", img, "-c")
	s.run("get", img, "-o", "out.png")
	s.run("get", script, "-o", "-")
	s.run("get", meeting, "-c", "-m")
	s.run("get", meeting, script, "-o", "x")
	s.run("get", "missing1")
	s.run("get", "@inbox", "-m")
	s.run("--json", "get", meeting, "--text")
	s.run("--json", "get", img)
	s.golden("get")
	if !bytes.Equal(s.files["out.png"], []byte("\x89PNG\x00\x01")) {
		t.Errorf("saved %q", s.files["out.png"])
	}
	// The binary content must not be downloaded without -o.
	s.srv.Calls = nil
	s.exec("", "get", img)
	for _, c := range s.srv.Calls {
		if strings.HasSuffix(c, "/content") {
			t.Errorf("binary content was downloaded: %v", s.srv.Calls)
		}
	}
}

func TestCreate(t *testing.T) {
	s := newSession(t)
	projects, _, _ := seed(s)
	py := s.file("hello.py", []byte("print('hello')\n"))
	png := s.file("red.png", []byte("\x89PNG\r\n\x1a\n\x00\x00binary\xff"))
	md := s.file("notes.md", []byte("# Notes\n"))
	s.run("create", "Quick", "<p>hi</p>")
	s.run("create", "Plain words", "no tags at all", "-p", projects)
	s.run("create", "-f", py, "-p", projects)
	s.run("create", "Renamed", "-f", md, "-p", projects, "-t", "text")
	s.run("create", "-f", png, "-p", projects)
	s.run("create", "lib.go", "-t", "code", "-p", projects)
	s.run("create", "Todo", "-t", "search", "#status=todo", "-p", projects)
	s.run("create", "Tagged", "-p", projects, "-a", "#status=new", "-a", "~seeAlso=meeting1", "-a", "#title=\"a b\"")
	s.run("create", "Fixed", "-p", projects, "--id", "myFixedId1", "--pos", "5")
	s.run("create", "Fixed", "-p", projects, "--id", "myFixedId1")
	s.run("create", "Stdin note", "-f", "-", "-p", projects)
	s.runIn("<p>from stdin</p>", "create", "Stdin note", "-f", "-", "-p", projects)
	s.run("get", "n0012", "-m")
	s.run("get", "n0014", "-m")
	s.run("create", "Bad", "-a", "#bad name")
	s.run("create", "Bad", "-a", "-#x")
	s.run("create", "Two", "x", "-f", py)
	s.run("create")
	s.env["TRILIUM_AGENT"] = "claude-1"
	s.run("--json", "create", "By agent", "<p>x</p>", "-p", projects)
	s.golden("create")
	if got := s.srv.Content("n0010"); !bytes.Equal(got, []byte("\x89PNG\r\n\x1a\n\x00\x00binary\xff")) {
		t.Errorf("binary upload: %q", got)
	}
}

func TestAppendEditPut(t *testing.T) {
	s := newSession(t)
	_, meeting, script := seed(s)
	s.srv.Add("img1", "projects1", "red.png", "image", "image/png", "\x89PNG")
	body := s.file("more.html", []byte("<p>more</p>"))
	s.run("append", meeting, "<p>Action items</p>")
	s.run("append", meeting, "-f", body, "--top")
	s.run("append", script, "print('bye')")
	s.run("append", script, "# header", "--top")
	s.run("append", meeting)
	s.run("append", meeting, "x", "-f", body)
	s.run("append", "_options", "<p>x</p>")
	s.run("append", "img1", "<p>x</p>")
	s.run("get", script, "-c")
	s.run("edit", meeting, "--old", "<p>Budget review</p>", "--new", "<p>Budget approved</p>", "--old", "one", "--new", "first")
	s.run("edit", meeting, "--old", "<li>", "--new", "<li>* ")
	s.run("edit", meeting, "--old", "<li>", "--new", "<li>* ", "--all")
	s.run("edit", meeting, "--old", "missing text", "--new", "x", "--old", "Agenda", "--new", "Plan")
	s.run("edit", meeting, "--old", "Agenda")
	s.run("edit", meeting, "--old", "", "--new", "x")
	s.run("edit", meeting)
	s.run("edit", meeting, "--old-file", s.file("old.txt", []byte("Agenda")), "--new-file", s.file("new.txt", []byte("Plan")))
	s.run("get", meeting, "-c")
	s.run("put", meeting, "<p>replaced</p>")
	s.run("put", meeting, "replaced without tags")
	s.run("--json", "put", script, "-f", s.file("s.py", []byte("pass\n")))
	s.run("--json", "edit", script, "--old", "pass", "--new", "return")
	s.golden("write")
}

func TestAppendRetriesOnConcurrentChange(t *testing.T) {
	s := newSession(t)
	_, meeting, _ := seed(s)
	once := sync.Once{}
	s.srv.AfterWrite = func(srv *etapitest.Server, id string) {
		// Another writer overwrites our first write.
		once.Do(func() { srv.SetContent(id, []byte("<p>other writer</p>")) })
	}
	_, errs, code := s.exec("", "append", meeting, "<p>mine</p>")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	if got := string(s.srv.Content(meeting)); got != "<p>other writer</p><p>mine</p>" {
		t.Errorf("content %q", got)
	}
	s.srv.AfterWrite = func(srv *etapitest.Server, id string) { srv.SetContent(id, []byte("<p>always</p>")) }
	_, errs, code = s.exec("", "append", meeting, "<p>mine</p>")
	if code != 1 || !strings.Contains(errs, "note changed concurrently, please retry") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestRm(t *testing.T) {
	s := newSession(t)
	projects, meeting, script := seed(s)
	s.srv.Add("child1", meeting, "Child", "text", "text/html", "")
	s.run("branch", "add", script, "root")
	s.run("rm", meeting)
	s.run("rm", meeting, "-r")
	s.run("rm", script)
	s.run("rm", "root")
	s.run("rm", "_hidden")
	s.run("rm", projects, "missing1", "-r")
	s.run("undelete", meeting)
	s.run("ls", projects)
	s.golden("rm")
}

package cli

import "testing"

func TestUpdate(t *testing.T) {
	s := newSession(t)
	_, meeting, script := seed(s)
	s.run("update", meeting, "--title", "Weekly sync")
	s.run("update", script, "--type", "code", "--mime", "text/x-go")
	s.run("update", script)
	s.run("update", "_options", "--title", "x")
	s.run("--json", "update", meeting, "--title", "Again")
	s.golden("update")
}

func TestAttr(t *testing.T) {
	s := newSession(t)
	projects, meeting, script := seed(s)
	s.run("attr", meeting)
	s.run("attr", meeting, "--all")
	s.run("attr", meeting, "#status=done", "+#tag=a", "+#tag=b", "-#tag=a", "#todo")
	s.run("attr", meeting, "#status(inheritable)=done", "~seeAlso=root", "+~seeAlso="+projects)
	s.run("attr", meeting, "-~seeAlso="+projects, "#title=\"a b\"", "-#missing")
	s.run("attr", meeting, "+#tag=c", "#tag=d")
	s.run("attr", meeting)
	s.run("attr", meeting, "#ok=1", "#bad name=2")
	s.run("attr", meeting, "~rel")
	s.run("attr", meeting, "~rel=missing1")
	s.run("attr", meeting, "--bogus")
	s.run("attr", "_options", "#x")
	s.run("attr", "_options", "#x", "--force")
	s.run("attr", script)
	s.run("attr", meeting, "--", "-#todo")
	s.run("--json", "attr", meeting, "#status=final", "-#tag")
	s.run("attr")
	s.run("attr", "--help")
	s.golden("attr")
}

func TestMvAndBranch(t *testing.T) {
	s := newSession(t)
	projects, meeting, script := seed(s)
	archive := s.srv.Add("archive1", "root", "Archive", "book", "", "")
	s.srv.Add("child1", meeting, "Child", "text", "text/html", "")
	s.run("mv", meeting, archive)
	s.run("ls", archive)
	s.run("mv", archive, "child1")
	s.run("mv", archive, archive)
	s.run("mv", "root", archive)
	s.run("mv", meeting, archive)
	s.run("branch", "add", script, archive, "--prefix", "2026: ")
	s.run("branch", "add", script, archive, "--pos", "1")
	s.run("branch", "add", archive, "child1")
	s.run("branch", "ls", script)
	s.run("mv", script, "root")
	s.run("mv", script, "root", "--from", projects, "--pos", "1")
	s.run("ls")
	s.run("branch", "set", script, archive, "--pos", "5", "--prefix", "", "--expanded=true")
	s.run("branch", "set", "root_script1", "--prefix", "x")
	s.run("branch", "set", script, archive)
	s.run("branch", "rm", script, archive)
	s.run("branch", "rm", script, "root")
	s.run("branch", "rm", script, projects)
	s.run("branch", "rm", "_options", "_hidden", "--force")
	s.run("--json", "branch", "ls", script)
	s.run("branch")
	s.run("branch", "bogus")
	s.golden("branch")
}

func TestAttachments(t *testing.T) {
	s := newSession(t)
	_, meeting, _ := seed(s)
	png := s.file("red.png", []byte("\x89PNG\r\n\x1a\nimage-bytes\xff"))
	csv := s.file("data.csv", []byte("a,b\n1,2\n"))
	s.run("att", "ls", meeting)
	s.run("att", "add", meeting, "-f", png)
	s.run("att", "add", meeting, "-f", csv)
	s.run("att", "add", meeting, "hello", "--title", "hello.txt")
	s.run("att", "add", meeting, "hello")
	s.run("att", "ls", meeting)
	s.run("att", "get", "att0006")
	s.run("att", "get", "att0006", "-o", "copy.png")
	s.run("att", "get", "att0007")
	s.run("att", "put", "att0007", "x,y\n")
	s.run("att", "put", "att0006", "-f", png)
	s.run("att", "update", "att0007", "--title", "renamed.csv", "--pos", "20")
	s.run("att", "update", "att0007")
	s.run("att", "rm", "att0008")
	s.run("att", "get", "missing99")
	s.run("att", "get", "bad id")
	s.run("--json", "att", "ls", meeting)
	s.golden("att")
}

func TestRevisions(t *testing.T) {
	s := newSession(t)
	_, meeting, _ := seed(s)
	s.run("rev", "ls", meeting)
	s.run("rev", "add", meeting)
	s.run("put", meeting, "<p>new</p>")
	s.run("rev", "add", meeting)
	s.run("rev", "ls", meeting)
	s.run("rev", "get", "r0006")
	s.run("rev", "get", "r0006", "--text")
	s.run("rev", "get", "r0006", "-o", "rev.html")
	s.run("rev", "get", "missing99")
	s.run("--json", "rev", "get", "r0006", "--text")
	s.golden("rev")
}

func TestAttrFailureKeepsOldValue(t *testing.T) {
	s := newSession(t)
	_, meeting, script := seed(s)
	// Missing target: checked before anything is written (F2).
	s.run("attr", meeting, "#status=done", "~seeAlso=doesNotExist1")
	s.run("attr", meeting)
	// Creating the replacement fails: the old relation stays.
	s.srv.Fail = "POST /etapi/attributes"
	s.run("attr", meeting, "~seeAlso=root")
	s.srv.Fail = ""
	s.run("attr", meeting)
	// Removing the old one fails: both are reported.
	s.srv.Fail = "DELETE /etapi/attributes"
	s.run("attr", meeting, "~seeAlso=root")
	s.srv.Fail = ""
	s.run("attr", meeting)
	// A later op fails after earlier ones were applied.
	s.srv.Fail = "PATCH /etapi/attributes"
	s.run("attr", script, "#new=1", "#new=2")
	s.srv.Fail = ""
	s.run("create", "Rel", "-p", "projects1", "-a", "~x=doesNotExist1")
	s.golden("attr-failures")
}

func TestControlCharsAndJSON(t *testing.T) {
	s := newSession(t)
	projects, meeting, _ := seed(s)
	odd := s.srv.Add("odd1", projects, "Tab\tTitle\nSecond line", "text", "text/html", "")
	s.srv.AddAttr(odd, "label", "note", "line1\nline2", false)
	s.run("ls", projects)
	s.run("get", odd, "-m")
	s.run("attr", odd)
	s.run("--json", "ls", projects, "-n", "1")
	png := s.file("p.png", []byte("\x89PNG\r\n\x1a\nxxxxxxxx"))
	s.run("--json", "att", "add", meeting, "-f", png)
	s.run("rev", "add", meeting)
	s.run("--json", "rev", "get", "r0008", "-o", "rev.html")
	s.golden("control-json")
}

func TestSystemNoteGuards(t *testing.T) {
	s := newSession(t)
	_, meeting, script := seed(s)
	att := s.srv.AddAttachment("_options", "file", "text/plain", "x.txt", []byte("x"))
	s.run("mv", script, "_hidden")
	s.run("branch", "add", script, "_hidden")
	s.run("branch", "add", "_options", meeting)
	s.run("branch", "set", "_options", "_hidden", "--pos", "5")
	s.run("att", "add", "_options", "x", "--title", "a.txt")
	s.run("att", "put", att, "y")
	s.run("att", "update", att, "--title", "b.txt")
	s.run("att", "rm", att)
	s.run("import", "_hidden", s.file("a.md", []byte("# a")))
	s.run("branch", "add", script, "_hidden", "--force")
	s.run("att", "rm", att, "--force")
	s.golden("guards")
}

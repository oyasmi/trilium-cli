package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHistoryUndelete(t *testing.T) {
	s := newSession(t)
	projects, meeting, _ := seed(s)
	s.run("rm", meeting)
	s.run("history", "-n", "3")
	s.run("history", "--in", projects, "-n", "0")
	s.run("undelete", meeting)
	s.run("undelete", meeting)
	s.run("undelete", "bad id")
	s.golden("history")
}

func TestExportImport(t *testing.T) {
	s := newSession(t)
	projects, _, _ := seed(s)
	md := s.file("Weekly report.md", []byte("# Report\n\nAll **good**.\n"))
	zipFile := s.file("backup.zip", mdZip(t))
	s.run("export", projects)
	s.run("export", projects, "--format", "markdown", "-o", "out.zip")
	s.run("export", projects, "--format", "pdf")
	s.run("import", projects, md)
	s.run("import", projects, zipFile)
	s.run("import", projects, s.file("x.txt", []byte("x")))
	s.run("import", projects, "/nonexistent/file.md")
	s.golden("export")
	if !bytes.HasPrefix(s.files["./Projects.zip"], []byte("PK")) {
		t.Errorf("export file: %q", s.files["./Projects.zip"])
	}
}

func mdZip(t *testing.T) []byte {
	t.Helper()
	b, err := markdownZip("inner.md", []byte("hi"), "0.106.1")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMarkdownZip(t *testing.T) {
	b := mdZip(t)
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 || zr.File[0].Name != "!!!meta.json" || zr.File[1].Name != "inner.md" {
		t.Fatalf("files: %v", zr.File)
	}
	rc, _ := zr.File[0].Open()
	data, _ := io.ReadAll(rc)
	var meta metaFile
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	f := meta.Files[0]
	if meta.FormatVersion != 2 || meta.AppVersion != "0.106.1" || f.Title != "inner" || f.Format != "markdown" ||
		f.Type != "text" || f.DataFileName != "inner.md" || len(f.NoteID) != 12 || f.NotePath[0] != f.NoteID {
		t.Errorf("meta: %s", data)
	}
	if !strings.Contains(string(data), `"prefix":null`) {
		t.Errorf("prefix must be null: %s", data)
	}
}

func TestInfoBackup(t *testing.T) {
	s := newSession(t)
	seed(s)
	s.run("info")
	s.env["TRILIUM_AGENT"] = "claude-1"
	s.run("info")
	s.run("backup")
	s.run("backup", "before-cleanup")
	s.run("backup", "nightly_1")
	s.run("--json", "backup")
	s.golden("info")
}

func TestCalendarAliases(t *testing.T) {
	s := newSession(t)
	seed(s)
	s.run("append", "@today", "<p>log</p>")
	s.run("get", "@yesterday", "-m")
	s.run("ls", "@2026-10")
	s.run("create", "Entry", "<p>x</p>", "-p", "@2026-W43")
	s.run("attr", "@2026", "#year")
	s.run("get", "@2026-02-30")
	s.golden("calendar")
}

func TestHelpers(t *testing.T) {
	for in, want := range map[string]string{"Q3 / Report: final?": "Q3 _ Report_ final_", "  ..  ": "fallback", "会议纪要": "会议纪要"} {
		if got := fileName(in, "fallback"); got != want {
			t.Errorf("fileName(%q)=%q", in, got)
		}
	}
	if needsQuotes(`note.title="hello.py"`) || !needsQuotes("note.title = hello.py") || !needsQuotes("#date>=2026-10-01") || needsQuotes("#status=done") {
		t.Error("needsQuotes")
	}
	if string(joinContent("code", []byte("a"), []byte("b"), false)) != "a\nb" ||
		string(joinContent("code", []byte("a\n"), []byte("b"), false)) != "a\nb" ||
		string(joinContent("code", []byte(""), []byte("b"), false)) != "b" ||
		string(joinContent("code", []byte("a"), []byte("b"), true)) != "b\na" ||
		string(joinContent("text", []byte("<p>a</p>"), []byte("<p>b</p>"), false)) != "<p>a</p><p>b</p>" {
		t.Error("joinContent")
	}
}

func TestSearchHelpMatchesSkill(t *testing.T) {
	skill, err := os.ReadFile("../../skills/trilium/references/search.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(skill) != searchHelp {
		t.Error("skills/trilium/references/search.md must be identical to internal/cli/help/search.md")
	}
}

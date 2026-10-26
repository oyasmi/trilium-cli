package cli

import (
	"strings"
	"testing"
)

// Behaviours added after the 2026-10-26 CLI review.
func TestReviewFixes(t *testing.T) {
	s := newSession(t)
	projects, meeting, script := seed(s)

	// Several IDs: a bad one doesn't hide the others.
	s.run("get", meeting, "missing1", script, "-m")
	s.run("get", meeting, script, "-c")

	// Text starting with "-" explains itself.
	s.run("append", meeting, "- item")
	s.run("append", meeting, "--", "- item")

	// Inline and file pairs can't be mixed.
	old := s.file("old.txt", []byte("a"))
	s.run("edit", meeting, "--old", "x", "--new", "y", "--old-file", old, "--new-file", old)

	// Removing a last placement deletes the note: children need -r, and the
	// result line says the note is gone.
	s.run("branch", "rm", projects, "root", "--force")
	s.run("branch", "rm", projects, "root", "--force", "-r")

	s.golden("review-fixes")
}

func TestSearchMoreCountsFilteredRows(t *testing.T) {
	s := newSession(t)
	seed(s)
	_, errs, _ := s.exec("", "search", `note.noteId != ""`, "-n", "2")
	if !strings.Contains(errs, "more available") {
		t.Errorf("want a more-available hint, got %q", errs)
	}
}

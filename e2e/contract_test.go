package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/oyasmi/trilium-cli/internal/config"
	"github.com/oyasmi/trilium-cli/internal/etapi"
)

// The tests in this file check, against the real server, the behaviour that
// internal/etapi/etapitest fakes. When one fails, update the fake (and the
// CLI) rather than the test.
//
// Not checked here on purpose: create-note with an existing noteId. Trilium
// 0.106.1 overwrites the note, and deleting such a note afterwards corrupts
// the server's in-memory attribute index (label searches fail until a
// restart). The fake models the overwrite as observed on 2026-10-25.

func client(t *testing.T) (*etapi.Client, *config.Config) {
	t.Helper()
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	return etapi.New(cfg.URL, cfg.Token), cfg
}

func newNote(t *testing.T, c *etapi.Client, parent, title string) string {
	t.Helper()
	res, err := c.CreateNote(t.Context(), etapi.CreateNoteDef{ParentNoteID: parent, Title: title, Type: "text", Content: "<p>x</p>"})
	if err != nil {
		t.Fatal(err)
	}
	return res.Note.NoteID
}

func TestContractBranches(t *testing.T) {
	c, _ := client(t)
	f := folder(t)
	a, b := newNote(t, c, f, "A"), newNote(t, c, f, "B")
	ctx := t.Context()
	br, created, err := c.CreateBranch(ctx, etapi.BranchCreate{NoteID: b, ParentNoteID: a})
	if err != nil || !created {
		t.Fatalf("new branch: created=%v err=%v", created, err)
	}
	if br.BranchID != a+"_"+b {
		t.Errorf("branch IDs are expected to be <parent>_<child>, got %s", br.BranchID)
	}
	if _, created, err = c.CreateBranch(ctx, etapi.BranchCreate{NoteID: b, ParentNoteID: a}); err != nil || created {
		t.Errorf("existing branch: want 200 (created=false), got created=%v err=%v", created, err)
	}
}

func TestContractSearch(t *testing.T) {
	c, _ := client(t)
	f := folder(t)
	n := newNote(t, c, f, "hello.py")
	res, err := c.Search(t.Context(), etapi.SearchParams{Search: `note.noteId != ""`, AncestorNoteID: f, IncludeArchivedNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range res {
		ids = append(ids, r.NoteID)
	}
	if !strings.Contains(strings.Join(ids, ","), f) || !strings.Contains(strings.Join(ids, ","), n) {
		t.Errorf("subtree search should include the ancestor itself and its child: %v", ids)
	}
	res, err = c.Search(t.Context(), etapi.SearchParams{Search: "note.title = hello.py", AncestorNoteID: f})
	if err != nil || len(res) != 0 {
		t.Errorf("unquoted value with '.': want silently empty, got %d results, err %v", len(res), err)
	}
}

func TestContractInheritedAttributes(t *testing.T) {
	c, _ := client(t)
	f := folder(t)
	parent := newNote(t, c, f, "P")
	child := newNote(t, c, parent, "C")
	ctx := t.Context()
	if _, err := c.CreateAttribute(ctx, etapi.AttributeCreate{NoteID: parent, Type: "label", Name: "inh", Value: "1", IsInheritable: true}); err != nil {
		t.Fatal(err)
	}
	n, err := c.GetNote(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range n.Attributes {
		found = found || (a.Name == "inh" && a.NoteID == parent)
	}
	if !found {
		t.Errorf("note attributes should include inherited ones owned by the parent: %+v", n.Attributes)
	}
	// Relations can't be patched (attr re-creates them).
	rel, err := c.CreateAttribute(ctx, etapi.AttributeCreate{NoteID: child, Type: "relation", Name: "r", Value: parent})
	if err != nil {
		t.Fatal(err)
	}
	v := child
	if _, err := c.PatchAttribute(ctx, rel.AttributeID, etapi.AttributePatch{Value: &v}); err == nil {
		t.Error("patching a relation's value is expected to be rejected")
	}
}

func TestContractContentQuirks(t *testing.T) {
	c, cfg := client(t)
	f := folder(t)
	n := newNote(t, c, f, "Q")
	ctx := t.Context()
	put := func(ct string) int {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPut, cfg.URL+"/notes/"+n+"/content", strings.NewReader(`{"a":1}`))
		req.Header.Set("Authorization", cfg.Token)
		req.Header.Set("Content-Type", ct)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if s := put("application/json"); s != 500 {
		t.Errorf("PUT content as JSON: want 500, got %d", s)
	}
	if s := put("text/plain"); s != 204 {
		t.Errorf("PUT content as text/plain: want 204, got %d", s)
	}

	body, _ := json.Marshal(map[string]string{"parentNoteId": f, "title": "no content", "type": "text"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL+"/create-note", bytes.NewReader(body))
	req.Header.Set("Authorization", cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Errorf("create-note without content: want 500, got %d", resp.StatusCode)
	}
}

func TestContractForcedGzip(t *testing.T) {
	c, cfg := client(t)
	f := folder(t)
	n := newNote(t, c, f, "G")
	big := "<p>" + strings.Repeat("gzip me ", 400) + "</p>"
	if err := c.PutContent(t.Context(), n, strings.NewReader(big), false); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, cfg.URL+"/notes/"+n+"/content", nil)
	req.Header.Set("Authorization", cfg.Token)
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := (&http.Client{Transport: &http.Transport{DisableCompression: true}}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("expected gzip even with Accept-Encoding: identity, got %q", resp.Header.Get("Content-Encoding"))
	}
}

func TestContractHistoryIntegers(t *testing.T) {
	c, cfg := client(t)
	f := folder(t)
	n := newNote(t, c, f, "H")
	if err := c.DeleteNote(t.Context(), n); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, cfg.URL+"/notes/history?ancestorNoteId="+f, nil)
	req.Header.Set("Authorization", cfg.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, r := range raw {
		if r["noteId"] == n {
			if _, isNum := r["current_isDeleted"].(float64); !isNum {
				t.Errorf("current_isDeleted is expected to be 0/1, got %T", r["current_isDeleted"])
			}
			return
		}
	}
	t.Errorf("deleted note %s not in history", n)
}

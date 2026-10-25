// Package etapitest is an in-memory fake of Trilium's ETAPI for command
// tests. It implements just enough behaviour, including the server quirks
// the CLI must handle: forced gzip above 1KB, deterministic branch IDs,
// search results that contain the ancestor itself, 500 on JSON content
// uploads and on create-note without content.
package etapitest

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/oyasmi/trilium-cli/internal/etapi"
)

// Token is the ETAPI token the fake accepts.
const Token = "test-token"

type note struct {
	etapi.Note
	content []byte
	deleted bool
}

type attachment struct {
	etapi.Attachment
	content []byte
}

type revision struct {
	etapi.Revision
	content []byte
}

// Server is a fake Trilium server.
type Server struct {
	*httptest.Server
	mu          sync.Mutex
	notes       map[string]*note
	branches    map[string]*etapi.Branch
	attrs       map[string]*etapi.Attribute
	attachments map[string]*attachment
	revisions   map[string]*revision
	history     []etapi.RecentChange
	undo        map[string][]etapi.Branch
	seq         int
	clock       int
	// Calls records "METHOD /path" for every request, for assertions.
	Calls []string
	// AfterWrite, if set, runs right after a note content PUT is applied
	// (used to simulate a concurrent writer overwriting it).
	AfterWrite func(s *Server, noteID string)
	// Fail, if set, makes matching requests ("METHOD /etapi/path" prefix
	// match) answer 500 without doing anything.
	Fail string
}

// New starts a fake server with root, the hidden subtree and an inbox.
func New() *Server {
	s := &Server{
		notes: map[string]*note{}, branches: map[string]*etapi.Branch{}, attrs: map[string]*etapi.Attribute{},
		attachments: map[string]*attachment{}, revisions: map[string]*revision{}, undo: map[string][]etapi.Branch{},
	}
	s.add("root", "", "root", "text", "text/html", "")
	s.add("_hidden", "root", "Hidden Notes", "doc", "", "")
	s.add("_options", "_hidden", "Options", "book", "", "")
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func (s *Server) now() string {
	s.clock++
	return fmt.Sprintf("2026-10-25 08:%02d:%02d.000Z", (s.clock/60)%60, s.clock%60)
}

func (s *Server) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s%04d", prefix, s.seq)
}

// Add creates a note directly (test setup) and returns its ID.
func (s *Server) Add(id, parent, title, typ, mime, content string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.add(id, parent, title, typ, mime, content)
}

func (s *Server) add(id, parent, title, typ, mime, content string) string {
	if id == "" {
		id = s.nextID("n")
	}
	ts := s.now()
	n := &note{Note: etapi.Note{NoteID: id, Title: title, Type: typ, Mime: mime, BlobID: blobID([]byte(content)),
		UTCDateCreated: ts, UTCDateModified: ts, ParentNoteIDs: []string{}, ChildNoteIDs: []string{}, ParentBranchIDs: []string{}, ChildBranchIDs: []string{}},
		content: []byte(content)}
	s.notes[id] = n
	if parent != "" {
		s.link(id, parent, "", nil)
	}
	s.history = append(s.history, etapi.RecentChange{NoteID: id, Title: title, CurrentTitle: title, UTCDate: ts})
	return id
}

// AddAttr adds an attribute directly (test setup).
func (s *Server) AddAttr(noteID, typ, name, value string, inheritable bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID("a")
	s.attrs[id] = &etapi.Attribute{AttributeID: id, NoteID: noteID, Type: typ, Name: name, Value: value, IsInheritable: inheritable, Position: 10 * (len(s.owned(noteID)) + 1)}
	return id
}

// AddAttachment adds an attachment directly (test setup).
func (s *Server) AddAttachment(owner, role, mime, title string, content []byte) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID("att")
	s.attachments[id] = &attachment{Attachment: etapi.Attachment{AttachmentID: id, OwnerID: owner, Role: role, Mime: mime, Title: title, ContentLength: len(content), Position: 10}, content: content}
	return id
}

// SetContent replaces a note's content directly (e.g. a concurrent writer).
func (s *Server) SetContent(id string, b []byte) {
	n := s.notes[id]
	n.content = b
	n.BlobID = blobID(b)
}

// Content returns a note's stored content.
func (s *Server) Content(id string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notes[id].content
}

// Exists reports whether a live note exists.
func (s *Server) Exists(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[id]
	return ok && !n.deleted
}

func blobID(b []byte) string {
	var h uint64 = 1469598103934665603
	for _, c := range b {
		h = (h ^ uint64(c)) * 1099511628211
	}
	return fmt.Sprintf("b%016x", h)
}

// link creates the branch parent→child; it returns false if it existed.
func (s *Server) link(child, parent, prefix string, pos *int) (*etapi.Branch, bool) {
	id := parent + "_" + child
	if b, ok := s.branches[id]; ok {
		b.Prefix = prefix
		if pos != nil {
			b.NotePosition = *pos
		}
		return b, false
	}
	p := s.notes[parent]
	position := 10 * (len(p.ChildNoteIDs) + 1)
	if pos != nil {
		position = *pos
	}
	b := &etapi.Branch{BranchID: id, NoteID: child, ParentNoteID: parent, Prefix: prefix, NotePosition: position, UTCDateModified: s.now()}
	s.branches[id] = b
	c := s.notes[child]
	c.ParentNoteIDs = append(c.ParentNoteIDs, parent)
	c.ParentBranchIDs = append(c.ParentBranchIDs, id)
	p.ChildNoteIDs = append(p.ChildNoteIDs, child)
	p.ChildBranchIDs = append(p.ChildBranchIDs, id)
	s.sortChildren(parent)
	return b, true
}

func (s *Server) sortChildren(parent string) {
	p := s.notes[parent]
	slices.SortStableFunc(p.ChildBranchIDs, func(a, b string) int { return s.branches[a].NotePosition - s.branches[b].NotePosition })
	p.ChildNoteIDs = p.ChildNoteIDs[:0]
	for _, bid := range p.ChildBranchIDs {
		p.ChildNoteIDs = append(p.ChildNoteIDs, s.branches[bid].NoteID)
	}
}

func (s *Server) unlink(branchID string) {
	var undo []etapi.Branch
	s.unlinkRec(branchID, &undo)
	if len(undo) > 0 {
		s.undo[undo[len(undo)-1].NoteID] = undo
	}
}

// unlinkRec removes a branch; a note losing its last branch is deleted with
// its subtree. Removed branches are recorded for undelete.
func (s *Server) unlinkRec(branchID string, undo *[]etapi.Branch) {
	b := s.branches[branchID]
	delete(s.branches, branchID)
	c, p := s.notes[b.NoteID], s.notes[b.ParentNoteID]
	c.ParentNoteIDs = remove(c.ParentNoteIDs, b.ParentNoteID)
	c.ParentBranchIDs = remove(c.ParentBranchIDs, branchID)
	p.ChildNoteIDs = remove(p.ChildNoteIDs, b.NoteID)
	p.ChildBranchIDs = remove(p.ChildBranchIDs, branchID)
	if len(c.ParentBranchIDs) == 0 {
		for _, cb := range slices.Clone(c.ChildBranchIDs) {
			s.unlinkRec(cb, undo)
		}
		c.deleted = true
		s.history = append(s.history, etapi.RecentChange{NoteID: c.NoteID, Title: c.Title, CurrentTitle: c.Title, CurrentIsDeleted: true, CanBeUndeleted: true, UTCDate: s.now()})
	}
	*undo = append(*undo, *b)
}

func remove(list []string, x string) []string {
	if i := slices.Index(list, x); i >= 0 {
		return slices.Delete(list, i, i+1)
	}
	return list
}

func (s *Server) deleteNote(id string) {
	var undo []etapi.Branch
	for _, bid := range slices.Clone(s.notes[id].ParentBranchIDs) {
		s.unlinkRec(bid, &undo)
	}
	s.undo[id] = undo
}

func (s *Server) undelete(id string) {
	undo := s.undo[id]
	delete(s.undo, id)
	for i := len(undo) - 1; i >= 0; i-- {
		b := undo[i]
		s.notes[b.NoteID].deleted = false
		pos := b.NotePosition
		s.link(b.NoteID, b.ParentNoteID, b.Prefix, &pos)
	}
}

func (s *Server) owned(id string) []etapi.Attribute {
	var out []etapi.Attribute
	for _, a := range s.attrs {
		if a.NoteID == id {
			out = append(out, *a)
		}
	}
	slices.SortFunc(out, func(a, b etapi.Attribute) int {
		if a.Position != b.Position {
			return a.Position - b.Position
		}
		return strings.Compare(a.AttributeID, b.AttributeID)
	})
	return out
}

// view renders a note with owned and inherited attributes, like ETAPI.
func (s *Server) view(n *note) etapi.Note {
	v := n.Note
	v.ParentNoteIDs = slices.Clone(n.ParentNoteIDs)
	v.ChildNoteIDs = slices.Clone(n.ChildNoteIDs)
	v.ParentBranchIDs = slices.Clone(n.ParentBranchIDs)
	v.ChildBranchIDs = slices.Clone(n.ChildBranchIDs)
	v.Attributes = s.owned(n.NoteID)
	seen := map[string]bool{n.NoteID: true}
	var walk func(id string)
	walk = func(id string) {
		for _, pid := range s.notes[id].ParentNoteIDs {
			if seen[pid] {
				continue
			}
			seen[pid] = true
			for _, a := range s.owned(pid) {
				if a.IsInheritable {
					v.Attributes = append(v.Attributes, a)
				}
			}
			walk(pid)
		}
	}
	walk(n.NoteID)
	if v.Attributes == nil {
		v.Attributes = []etapi.Attribute{}
	}
	return v
}

func (s *Server) live(id string) (*note, bool) {
	n, ok := s.notes[id]
	if !ok || n.deleted {
		return nil, false
	}
	return n, true
}

// isDescendant reports whether id is below (or equal to) ancestor and at
// which depth.
func (s *Server) depth(id, ancestor string) (int, bool) {
	if id == ancestor {
		return 0, true
	}
	best, found := 0, false
	var walk func(cur string, d int, seen map[string]bool)
	walk = func(cur string, d int, seen map[string]bool) {
		for _, p := range s.notes[cur].ParentNoteIDs {
			if seen[p] {
				continue
			}
			if p == ancestor {
				if !found || d+1 < best {
					best, found = d+1, true
				}
				continue
			}
			seen[p] = true
			walk(p, d+1, seen)
		}
	}
	walk(id, 0, map[string]bool{id: true})
	return best, found
}

// ---- HTTP plumbing ----

type httpError struct {
	status int
	code   string
	msg    string
}

func notFound(kind, id string) *httpError {
	return &httpError{404, strings.ToUpper(kind) + "_NOT_FOUND", fmt.Sprintf("%s '%s' not found.", kind, id)}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls = append(s.Calls, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != Token {
		writeJSON(w, 401, map[string]any{"status": 401, "code": "NOT_AUTHENTICATED", "message": "Not authenticated"})
		return
	}
	if s.Fail != "" && strings.HasPrefix(r.Method+" "+r.URL.Path, s.Fail) {
		writeJSON(w, 500, map[string]any{"status": 500, "code": "INJECTED", "message": "injected failure"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/etapi")
	status, body, ctype, herr := s.route(r, path)
	if herr != nil {
		writeJSON(w, herr.status, map[string]any{"status": herr.status, "code": herr.code, "message": herr.msg})
		return
	}
	if body == nil {
		w.WriteHeader(status)
		return
	}
	if ctype == "" {
		ctype = "application/json; charset=utf-8"
	}
	writeRaw(w, status, ctype, body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, _ := json.Marshal(v)
	writeRaw(w, status, "application/json; charset=utf-8", b)
}

// writeRaw gzips anything over 1KB regardless of Accept-Encoding, like
// Trilium's compression middleware.
func writeRaw(w http.ResponseWriter, status int, ctype string, b []byte) {
	w.Header().Set("Content-Type", ctype)
	if len(b) > 1024 {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(b)
		_ = zw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		b = buf.Bytes()
	}
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

var (
	reNote       = regexp.MustCompile(`^/notes/([^/]+)$`)
	reNoteSub    = regexp.MustCompile(`^/notes/([^/]+)/(content|attachments|revisions|revision|undelete|export|import)$`)
	reEntity     = regexp.MustCompile(`^/(branches|attributes|attachments|revisions)/([^/]+)$`)
	reEntityData = regexp.MustCompile(`^/(attachments|revisions)/([^/]+)/content$`)
	reCalendar   = regexp.MustCompile(`^/(inbox|calendar/days|calendar/weeks|calendar/months|calendar/years|calendar/week-first-day)/([^/]+)$`)
	reRefresh    = regexp.MustCompile(`^/refresh-note-ordering/([^/]+)$`)
	reBackup     = regexp.MustCompile(`^/backup/([^/]+)$`)
)

func js(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (s *Server) route(r *http.Request, path string) (int, []byte, string, *httpError) {
	switch {
	case path == "/create-note" && r.Method == http.MethodPost:
		return s.createNote(r)
	case path == "/notes" && r.Method == http.MethodGet:
		return s.search(r)
	case path == "/notes/history":
		return 200, js(s.history), "", nil
	case path == "/app-info":
		return 200, js(etapi.AppInfo{AppVersion: "0.106.1", DBVersion: 240}), "", nil
	case path == "/metrics":
		m := etapi.Metrics{}
		for _, n := range s.notes {
			if n.deleted {
				m.Database.DeletedNotes++
			} else {
				m.Database.ActiveNotes++
			}
		}
		return 200, js(m), "", nil
	case path == "/branches" && r.Method == http.MethodPost:
		return s.createBranch(r)
	case path == "/attributes" && r.Method == http.MethodPost:
		return s.createAttr(r)
	case path == "/attachments" && r.Method == http.MethodPost:
		return s.createAttachment(r)
	case reBackup.MatchString(path):
		return 204, nil, "", nil
	case reRefresh.MatchString(path):
		return 204, nil, "", nil
	case reCalendar.MatchString(path):
		m := reCalendar.FindStringSubmatch(path)
		return s.calendar(m[1], m[2])
	case reNote.MatchString(path):
		return s.noteEntity(r, reNote.FindStringSubmatch(path)[1])
	case reNoteSub.MatchString(path):
		m := reNoteSub.FindStringSubmatch(path)
		return s.noteSub(r, m[1], m[2])
	case reEntityData.MatchString(path):
		m := reEntityData.FindStringSubmatch(path)
		return s.entityData(r, m[1], m[2])
	case reEntity.MatchString(path):
		m := reEntity.FindStringSubmatch(path)
		return s.entity(r, m[1], m[2])
	}
	return 0, nil, "", &httpError{404, "NOT_FOUND", "no route " + path}
}

func decodeBody(r *http.Request, v any) *httpError {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return &httpError{400, "BAD_REQUEST", err.Error()}
	}
	return nil
}

func (s *Server) createNote(r *http.Request) (int, []byte, string, *httpError) {
	var raw map[string]json.RawMessage
	b, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(b, &raw); err != nil {
		return 0, nil, "", &httpError{400, "BAD_REQUEST", err.Error()}
	}
	if _, ok := raw["content"]; !ok {
		return 0, nil, "", &httpError{500, "", "Internal Server Error"}
	}
	var def etapi.CreateNoteDef
	_ = json.Unmarshal(b, &def)
	if _, ok := s.live(def.ParentNoteID); !ok {
		return 0, nil, "", notFound("Note", def.ParentNoteID)
	}
	if n, ok := s.live(def.NoteID); ok && def.NoteID != "" {
		// Like Trilium 0.106.1: an existing note is silently overwritten.
		n.Title, n.Type = def.Title, def.Type
		n.Mime = def.Mime
		s.SetContent(n.NoteID, []byte(def.Content))
		br, _ := s.link(n.NoteID, def.ParentNoteID, def.Prefix, def.NotePosition)
		return 201, js(etapi.NoteWithBranch{Note: s.view(n), Branch: *br}), "", nil
	}
	mime := def.Mime
	if mime == "" {
		switch def.Type {
		case "text":
			mime = "text/html"
		case "code":
			mime = "text/plain"
		case "mermaid":
			mime = "text/vnd.mermaid"
		}
	}
	id := s.add(def.NoteID, "", def.Title, def.Type, mime, def.Content)
	br, _ := s.link(id, def.ParentNoteID, def.Prefix, def.NotePosition)
	return 201, js(etapi.NoteWithBranch{Note: s.view(s.notes[id]), Branch: *br}), "", nil
}

var (
	reSearchAll    = regexp.MustCompile(`^note\.noteId != ""$`)
	reSearchID     = regexp.MustCompile(`^note\.noteId = "([^"]+)"$`)
	reSearchTitle  = regexp.MustCompile(`^note\.title ?= ?"([^"]*)"$`)
	reSearchLabel  = regexp.MustCompile(`^#([\p{L}\p{N}_:]+)(?:=(.*))?$`)
	reSearchQuoted = regexp.MustCompile(`=\s*[^"\s]*[.-]`)
)

// search supports the query forms the CLI and tests use: all notes, a note
// ID, an exact title, a label (optionally with value) and plain words.
func (s *Server) search(r *http.Request) (int, []byte, string, *httpError) {
	q := r.URL.Query()
	query := q.Get("search")
	ancestor := q.Get("ancestorNoteId")
	if ancestor == "" {
		ancestor = "root"
	}
	maxDepth := -1
	if d := q.Get("ancestorDepth"); strings.HasPrefix(d, "lt") {
		maxDepth, _ = strconv.Atoi(d[2:])
		maxDepth--
	} else if strings.HasPrefix(d, "eq") {
		maxDepth, _ = strconv.Atoi(d[2:])
	}
	match := func(n *note) bool {
		switch {
		case reSearchAll.MatchString(query):
			return true
		case reSearchID.MatchString(query):
			return n.NoteID == reSearchID.FindStringSubmatch(query)[1]
		case reSearchTitle.MatchString(query):
			return n.Title == reSearchTitle.FindStringSubmatch(query)[1]
		case reSearchQuoted.MatchString(query):
			return false // Trilium silently matches nothing for unquoted '.'/'-' values
		case reSearchLabel.MatchString(query):
			m := reSearchLabel.FindStringSubmatch(query)
			for _, a := range s.view(n).Attributes {
				if a.Type == "label" && a.Name == m[1] && (m[2] == "" && !strings.Contains(query, "=") || a.Value == m[2]) {
					return true
				}
			}
			return false
		}
		for w := range strings.FieldsSeq(strings.ToLower(query)) {
			if !strings.Contains(strings.ToLower(n.Title), w) && !strings.Contains(strings.ToLower(string(n.content)), w) {
				return false
			}
		}
		return true
	}
	var res []etapi.Note
	ids := make([]string, 0, len(s.notes))
	for id := range s.notes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		n := s.notes[id]
		if n.deleted {
			continue
		}
		d, ok := s.depth(id, ancestor)
		if !ok || (maxDepth >= 0 && d > maxDepth) {
			continue
		}
		// Like Trilium, the hidden subtree only shows up as _hidden itself.
		if strings.HasPrefix(id, "_") && id != "_hidden" && !strings.HasPrefix(ancestor, "_") {
			continue
		}
		if match(n) {
			res = append(res, s.view(n))
		}
	}
	if l, _ := strconv.Atoi(q.Get("limit")); l > 0 && len(res) > l {
		res = res[:l]
	}
	if res == nil {
		res = []etapi.Note{}
	}
	return 200, js(etapi.SearchResponse{Results: res}), "", nil
}

func (s *Server) noteEntity(r *http.Request, id string) (int, []byte, string, *httpError) {
	n, ok := s.live(id)
	if !ok {
		return 0, nil, "", notFound("Note", id)
	}
	switch r.Method {
	case http.MethodGet:
		return 200, js(s.view(n)), "", nil
	case http.MethodPatch:
		var p etapi.NotePatch
		if e := decodeBody(r, &p); e != nil {
			return 0, nil, "", e
		}
		if p.Title != nil {
			n.Title = *p.Title
		}
		if p.Type != nil {
			n.Type = *p.Type
		}
		if p.Mime != nil {
			n.Mime = *p.Mime
		}
		n.UTCDateModified = s.now()
		return 200, js(s.view(n)), "", nil
	case http.MethodDelete:
		s.deleteNote(id)
		return 204, nil, "", nil
	}
	return 0, nil, "", &httpError{405, "", "method not allowed"}
}

func (s *Server) noteSub(r *http.Request, id, sub string) (int, []byte, string, *httpError) {
	if sub == "undelete" {
		n, ok := s.notes[id]
		if !ok || !n.deleted {
			return 0, nil, "", &httpError{400, "NOTE_NOT_DELETED", fmt.Sprintf("Note '%s' is not deleted.", id)}
		}
		s.undelete(id)
		return 200, js(map[string]bool{"success": true}), "", nil
	}
	n, ok := s.live(id)
	if !ok {
		return 0, nil, "", notFound("Note", id)
	}
	switch sub {
	case "content":
		if r.Method == http.MethodGet {
			return 200, n.content, n.Mime, nil
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/plain") && ct != "application/octet-stream" {
			return 0, nil, "", &httpError{500, "", "Internal Server Error"}
		}
		b, _ := io.ReadAll(r.Body)
		s.SetContent(id, b)
		if s.AfterWrite != nil {
			s.AfterWrite(s, id)
		}
		n.UTCDateModified = s.now()
		return 204, nil, "", nil
	case "attachments":
		var out []etapi.Attachment
		for _, a := range s.attachments {
			if a.OwnerID == id {
				out = append(out, a.Attachment)
			}
		}
		slices.SortFunc(out, func(a, b etapi.Attachment) int { return strings.Compare(a.AttachmentID, b.AttachmentID) })
		if out == nil {
			out = []etapi.Attachment{}
		}
		return 200, js(out), "", nil
	case "revision":
		rid := s.nextID("r")
		ts := s.now()
		s.revisions[rid] = &revision{Revision: etapi.Revision{RevisionID: rid, NoteID: id, Type: n.Type, Mime: n.Mime, Title: n.Title,
			UTCDateCreated: ts, UTCDateLastEdited: n.UTCDateModified, ContentLength: len([]rune(string(n.content)))}, content: slices.Clone(n.content)}
		return 204, nil, "", nil
	case "revisions":
		var out []etapi.Revision
		for _, rv := range s.revisions {
			if rv.NoteID == id {
				out = append(out, rv.Revision)
			}
		}
		slices.SortFunc(out, func(a, b etapi.Revision) int { return strings.Compare(a.RevisionID, b.RevisionID) })
		if out == nil {
			out = []etapi.Revision{}
		}
		return 200, js(out), "", nil
	case "export":
		return 200, []byte("PK\x03\x04fake-zip-of-" + id + "-" + r.URL.Query().Get("format")), "application/zip", nil
	case "import":
		return s.importZip(r, id)
	}
	return 0, nil, "", &httpError{404, "NOT_FOUND", "no route"}
}

func (s *Server) importZip(r *http.Request, parent string) (int, []byte, string, *httpError) {
	b, _ := io.ReadAll(r.Body)
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return 0, nil, "", &httpError{400, "BAD_ZIP", err.Error()}
	}
	title, body := "imported", ""
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		if f.Name == "!!!meta.json" {
			var meta struct {
				Files []struct {
					Title  string `json:"title"`
					Format string `json:"format"`
				} `json:"files"`
			}
			_ = json.Unmarshal(data, &meta)
			if len(meta.Files) > 0 {
				title = meta.Files[0].Title + " (" + meta.Files[0].Format + ")"
			}
			continue
		}
		body = "<p>" + strings.TrimSpace(string(data)) + "</p>"
	}
	id := s.add("", "", title, "text", "text/html", body)
	br, _ := s.link(id, parent, "", nil)
	return 201, js(etapi.NoteWithBranch{Note: s.view(s.notes[id]), Branch: *br}), "", nil
}

func (s *Server) calendar(kind, value string) (int, []byte, string, *httpError) {
	if kind == "inbox" {
		for _, a := range s.attrs {
			if a.Type == "label" && a.Name == "inbox" {
				if n, ok := s.live(a.NoteID); ok {
					return 200, js(s.view(n)), "", nil
				}
			}
		}
		return 200, js(s.view(s.notes["root"])), "", nil
	}
	cal := "calendar"
	if _, ok := s.live(cal); !ok {
		s.add(cal, "root", "Calendar", "book", "", "")
	}
	id := "cal_" + strings.ReplaceAll(value, "-", "_")
	if _, ok := s.live(id); !ok {
		s.add(id, cal, value, "text", "text/html", "")
	}
	return 200, js(s.view(s.notes[id])), "", nil
}

func (s *Server) createBranch(r *http.Request) (int, []byte, string, *httpError) {
	var b etapi.BranchCreate
	if e := decodeBody(r, &b); e != nil {
		return 0, nil, "", e
	}
	if _, ok := s.live(b.NoteID); !ok {
		return 0, nil, "", notFound("Note", b.NoteID)
	}
	if _, ok := s.live(b.ParentNoteID); !ok {
		return 0, nil, "", notFound("Note", b.ParentNoteID)
	}
	prefix := ""
	if b.Prefix != nil {
		prefix = *b.Prefix
	}
	// Like Trilium, no cycle check here.
	br, created := s.link(b.NoteID, b.ParentNoteID, prefix, b.NotePosition)
	status := 200
	if created {
		status = 201
	}
	return status, js(br), "", nil
}

func (s *Server) createAttr(r *http.Request) (int, []byte, string, *httpError) {
	var a etapi.AttributeCreate
	if e := decodeBody(r, &a); e != nil {
		return 0, nil, "", e
	}
	if _, ok := s.live(a.NoteID); !ok {
		return 0, nil, "", notFound("Note", a.NoteID)
	}
	if a.Type == "relation" {
		if _, ok := s.live(a.Value); !ok {
			return 0, nil, "", notFound("Note", a.Value)
		}
	}
	id := s.nextID("a")
	attr := &etapi.Attribute{AttributeID: id, NoteID: a.NoteID, Type: a.Type, Name: strings.ReplaceAll(a.Name, " ", "_"), Value: a.Value,
		IsInheritable: a.IsInheritable, Position: 10 * (len(s.owned(a.NoteID)) + 1), UTCDateModified: s.now()}
	s.attrs[id] = attr
	return 201, js(attr), "", nil
}

func (s *Server) createAttachment(r *http.Request) (int, []byte, string, *httpError) {
	var a etapi.AttachmentCreate
	if e := decodeBody(r, &a); e != nil {
		return 0, nil, "", e
	}
	if _, ok := s.live(a.OwnerID); !ok {
		return 0, nil, "", notFound("Note", a.OwnerID)
	}
	id := s.nextID("att")
	att := &attachment{Attachment: etapi.Attachment{AttachmentID: id, OwnerID: a.OwnerID, Role: a.Role, Mime: a.Mime, Title: a.Title, Position: 10,
		ContentLength: len([]rune(a.Content))}, content: []byte(a.Content)}
	s.attachments[id] = att
	return 201, js(att.Attachment), "", nil
}

func (s *Server) entity(r *http.Request, kind, id string) (int, []byte, string, *httpError) {
	switch kind {
	case "branches":
		b, ok := s.branches[id]
		if !ok {
			return 0, nil, "", notFound("Branch", id)
		}
		switch r.Method {
		case http.MethodGet:
			return 200, js(b), "", nil
		case http.MethodPatch:
			var p etapi.BranchPatch
			if e := decodeBody(r, &p); e != nil {
				return 0, nil, "", e
			}
			if p.Prefix != nil {
				b.Prefix = *p.Prefix
			}
			if p.NotePosition != nil {
				b.NotePosition = *p.NotePosition
				s.sortChildren(b.ParentNoteID)
			}
			if p.IsExpanded != nil {
				b.IsExpanded = *p.IsExpanded
			}
			return 200, js(b), "", nil
		case http.MethodDelete:
			s.unlink(id)
			return 204, nil, "", nil
		}
	case "attributes":
		a, ok := s.attrs[id]
		if !ok {
			return 0, nil, "", notFound("Attribute", id)
		}
		switch r.Method {
		case http.MethodGet:
			return 200, js(a), "", nil
		case http.MethodPatch:
			var p etapi.AttributePatch
			if e := decodeBody(r, &p); e != nil {
				return 0, nil, "", e
			}
			if p.Value != nil {
				if a.Type == "relation" {
					return 0, nil, "", &httpError{400, "PROPERTY_NOT_ALLOWED", "Property 'value' is not allowed for relations."}
				}
				a.Value = *p.Value
			}
			return 200, js(a), "", nil
		case http.MethodDelete:
			delete(s.attrs, id)
			return 204, nil, "", nil
		}
	case "attachments":
		a, ok := s.attachments[id]
		if !ok {
			return 0, nil, "", notFound("Attachment", id)
		}
		switch r.Method {
		case http.MethodGet:
			return 200, js(a.Attachment), "", nil
		case http.MethodPatch:
			var p etapi.AttachmentPatch
			if e := decodeBody(r, &p); e != nil {
				return 0, nil, "", e
			}
			if p.Title != nil {
				a.Title = *p.Title
			}
			if p.Role != nil {
				a.Role = *p.Role
			}
			if p.Mime != nil {
				a.Mime = *p.Mime
			}
			if p.Position != nil {
				a.Position = *p.Position
			}
			return 200, js(a.Attachment), "", nil
		case http.MethodDelete:
			delete(s.attachments, id)
			return 204, nil, "", nil
		}
	case "revisions":
		rv, ok := s.revisions[id]
		if !ok {
			return 0, nil, "", notFound("Revision", id)
		}
		return 200, js(rv.Revision), "", nil
	}
	return 0, nil, "", &httpError{405, "", "method not allowed"}
}

func (s *Server) entityData(r *http.Request, kind, id string) (int, []byte, string, *httpError) {
	if kind == "revisions" {
		rv, ok := s.revisions[id]
		if !ok {
			return 0, nil, "", notFound("Revision", id)
		}
		return 200, rv.content, rv.Mime, nil
	}
	a, ok := s.attachments[id]
	if !ok {
		return 0, nil, "", notFound("Attachment", id)
	}
	if r.Method == http.MethodGet {
		return 200, a.content, a.Mime, nil
	}
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") && ct != "application/octet-stream" {
		return 0, nil, "", &httpError{500, "", "Internal Server Error"}
	}
	a.content, _ = io.ReadAll(r.Body)
	a.ContentLength = len([]rune(string(a.content)))
	return 204, nil, "", nil
}

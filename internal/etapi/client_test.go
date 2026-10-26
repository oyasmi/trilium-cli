package etapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recorded struct {
	method, path, query, contentType, auth, ua string
	body                                       string
}

// newServer returns a client wired to a handler that records the last request
// and answers with status and body.
func newServer(t *testing.T, status int, body string) (*Client, *recorded) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*rec = recorded{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Content-Type"), r.Header.Get("Authorization"), r.Header.Get("User-Agent"), string(b)}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL+"/etapi", "tok", WithUserAgent("trilium-cli/test"), WithRetryDelay(time.Millisecond)), rec
}

func TestMethods(t *testing.T) {
	note := `{"noteId":"n1","title":"T","type":"text","mime":"text/html","attributes":[]}`
	pos := 5
	title := "New"
	cases := []struct {
		name                   string
		status                 int
		resp                   string
		call                   func(c *Client, ctx context.Context) error
		method, path, query    string
		contentType, bodyMatch string
	}{
		{"CreateNote", 201, `{"note":` + note + `,"branch":{"branchId":"p_n1"}}`, func(c *Client, ctx context.Context) error {
			_, err := c.CreateNote(ctx, CreateNoteDef{ParentNoteID: "p", Title: "T", Type: "text"})
			return err
		}, "POST", "/etapi/create-note", "", "application/json", `"content":""`},
		{"GetNote", 200, note, func(c *Client, ctx context.Context) error {
			n, err := c.GetNote(ctx, "n1")
			if err == nil && n.Title != "T" {
				return errors.New("bad decode")
			}
			return err
		}, "GET", "/etapi/notes/n1", "", "", ""},
		{"PatchNote", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.PatchNote(ctx, "n1", NotePatch{Title: &title})
			return err
		}, "PATCH", "/etapi/notes/n1", "", "application/json", `{"title":"New"}`},
		{"DeleteNote", 204, "", func(c *Client, ctx context.Context) error { return c.DeleteNote(ctx, "n1") }, "DELETE", "/etapi/notes/n1", "", "", ""},
		{"GetContent", 200, "<p>x</p>", func(c *Client, ctx context.Context) error {
			b, err := c.GetContent(ctx, "n1")
			if err == nil && string(b) != "<p>x</p>" {
				return errors.New("bad content")
			}
			return err
		}, "GET", "/etapi/notes/n1/content", "", "", ""},
		{"PutContentText", 204, "", func(c *Client, ctx context.Context) error {
			return c.PutContent(ctx, "n1", strings.NewReader("<p>y</p>"), false)
		}, "PUT", "/etapi/notes/n1/content", "", "text/plain; charset=utf-8", "<p>y</p>"},
		{"PutContentBinary", 204, "", func(c *Client, ctx context.Context) error {
			return c.PutContent(ctx, "n1", bytes.NewReader([]byte{0, 1, 2}), true)
		}, "PUT", "/etapi/notes/n1/content", "", "application/octet-stream", "\x00\x01\x02"},
		{"Search", 200, `{"results":[` + note + `]}`, func(c *Client, ctx context.Context) error {
			r, err := c.Search(ctx, SearchParams{Search: "a b", FastSearch: true, IncludeArchivedNotes: true, AncestorNoteID: "x", AncestorDepth: "lt2", OrderBy: "title", OrderDirection: "desc", Limit: 21})
			if err == nil && len(r) != 1 {
				return errors.New("bad results")
			}
			return err
		}, "GET", "/etapi/notes", "ancestorDepth=lt2&ancestorNoteId=x&fastSearch=true&includeArchivedNotes=true&limit=21&orderBy=title&orderDirection=desc&search=a+b", "", ""},
		{"Export", 200, "PK", func(c *Client, ctx context.Context) error {
			_, err := c.Export(ctx, "n1", "markdown")
			return err
		}, "GET", "/etapi/notes/n1/export", "format=markdown", "", ""},
		{"Import", 201, `{"note":` + note + `}`, func(c *Client, ctx context.Context) error {
			_, err := c.Import(ctx, "p", strings.NewReader("PK"))
			return err
		}, "POST", "/etapi/notes/p/import", "", "application/octet-stream", "PK"},
		{"CreateRevision", 204, "", func(c *Client, ctx context.Context) error { return c.CreateRevision(ctx, "n1") }, "POST", "/etapi/notes/n1/revision", "", "", ""},
		{"ListRevisions", 200, `[{"revisionId":"r1"}]`, func(c *Client, ctx context.Context) error {
			_, err := c.ListRevisions(ctx, "n1")
			return err
		}, "GET", "/etapi/notes/n1/revisions", "", "", ""},
		{"ListAttachments", 200, `[]`, func(c *Client, ctx context.Context) error {
			_, err := c.ListAttachments(ctx, "n1")
			return err
		}, "GET", "/etapi/notes/n1/attachments", "", "", ""},
		{"Undelete", 200, `{"success":true}`, func(c *Client, ctx context.Context) error { return c.Undelete(ctx, "n1") }, "POST", "/etapi/notes/n1/undelete", "", "", ""},
		{"History", 200, `[]`, func(c *Client, ctx context.Context) error {
			_, err := c.History(ctx, "p")
			return err
		}, "GET", "/etapi/notes/history", "ancestorNoteId=p", "", ""},
		{"GetRevision", 200, `{"revisionId":"r1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.GetRevision(ctx, "r1")
			return err
		}, "GET", "/etapi/revisions/r1", "", "", ""},
		{"GetRevisionContent", 200, `x`, func(c *Client, ctx context.Context) error {
			_, err := c.GetRevisionContent(ctx, "r1")
			return err
		}, "GET", "/etapi/revisions/r1/content", "", "", ""},
		{"GetBranch", 200, `{"branchId":"p_n1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.GetBranch(ctx, "p_n1")
			return err
		}, "GET", "/etapi/branches/p_n1", "", "", ""},
		{"PatchBranch", 200, `{"branchId":"p_n1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.PatchBranch(ctx, "p_n1", BranchPatch{NotePosition: &pos})
			return err
		}, "PATCH", "/etapi/branches/p_n1", "", "application/json", `{"notePosition":5}`},
		{"DeleteBranch", 204, "", func(c *Client, ctx context.Context) error { return c.DeleteBranch(ctx, "p_n1") }, "DELETE", "/etapi/branches/p_n1", "", "", ""},
		{"RefreshNoteOrdering", 204, "", func(c *Client, ctx context.Context) error { return c.RefreshNoteOrdering(ctx, "p") }, "POST", "/etapi/refresh-note-ordering/p", "", "", ""},
		{"CreateAttachment", 201, `{"attachmentId":"a1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.CreateAttachment(ctx, AttachmentCreate{OwnerID: "n1", Role: "file", Mime: "text/plain", Title: "a"})
			return err
		}, "POST", "/etapi/attachments", "", "application/json", `"ownerId":"n1"`},
		{"GetAttachment", 200, `{"attachmentId":"a1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.GetAttachment(ctx, "a1")
			return err
		}, "GET", "/etapi/attachments/a1", "", "", ""},
		{"PatchAttachment", 200, `{"attachmentId":"a1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.PatchAttachment(ctx, "a1", AttachmentPatch{Title: &title})
			return err
		}, "PATCH", "/etapi/attachments/a1", "", "application/json", `{"title":"New"}`},
		{"DeleteAttachment", 204, "", func(c *Client, ctx context.Context) error { return c.DeleteAttachment(ctx, "a1") }, "DELETE", "/etapi/attachments/a1", "", "", ""},
		{"GetAttachmentContent", 200, "x", func(c *Client, ctx context.Context) error {
			_, err := c.GetAttachmentContent(ctx, "a1")
			return err
		}, "GET", "/etapi/attachments/a1/content", "", "", ""},
		{"PutAttachmentContent", 204, "", func(c *Client, ctx context.Context) error {
			return c.PutAttachmentContent(ctx, "a1", strings.NewReader("x"), true)
		}, "PUT", "/etapi/attachments/a1/content", "", "application/octet-stream", "x"},
		{"CreateAttribute", 201, `{"attributeId":"at1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.CreateAttribute(ctx, AttributeCreate{NoteID: "n1", Type: "label", Name: "x"})
			return err
		}, "POST", "/etapi/attributes", "", "application/json", `"name":"x"`},
		{"GetAttribute", 200, `{"attributeId":"at1"}`, func(c *Client, ctx context.Context) error {
			_, err := c.GetAttribute(ctx, "at1")
			return err
		}, "GET", "/etapi/attributes/at1", "", "", ""},
		{"PatchAttribute", 200, `{"attributeId":"at1"}`, func(c *Client, ctx context.Context) error {
			v := ""
			_, err := c.PatchAttribute(ctx, "at1", AttributePatch{Value: &v})
			return err
		}, "PATCH", "/etapi/attributes/at1", "", "application/json", `{"value":""}`},
		{"DeleteAttribute", 204, "", func(c *Client, ctx context.Context) error { return c.DeleteAttribute(ctx, "at1") }, "DELETE", "/etapi/attributes/at1", "", "", ""},
		{"Inbox", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.Inbox(ctx, "2026-10-25")
			return err
		}, "GET", "/etapi/inbox/2026-10-25", "", "", ""},
		{"Day", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.Day(ctx, "2026-10-25")
			return err
		}, "GET", "/etapi/calendar/days/2026-10-25", "", "", ""},
		{"WeekFirstDay", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.WeekFirstDay(ctx, "2026-10-25")
			return err
		}, "GET", "/etapi/calendar/week-first-day/2026-10-25", "", "", ""},
		{"Week", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.Week(ctx, "2026-W43")
			return err
		}, "GET", "/etapi/calendar/weeks/2026-W43", "", "", ""},
		{"Month", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.Month(ctx, "2026-10")
			return err
		}, "GET", "/etapi/calendar/months/2026-10", "", "", ""},
		{"Year", 200, note, func(c *Client, ctx context.Context) error {
			_, err := c.Year(ctx, "2026")
			return err
		}, "GET", "/etapi/calendar/years/2026", "", "", ""},
		{"AppInfo", 200, `{"appVersion":"0.106.1"}`, func(c *Client, ctx context.Context) error {
			a, err := c.AppInfo(ctx)
			if err == nil && a.AppVersion != "0.106.1" {
				return errors.New("bad decode")
			}
			return err
		}, "GET", "/etapi/app-info", "", "", ""},
		{"Metrics", 200, `{"database":{"activeNotes":4}}`, func(c *Client, ctx context.Context) error {
			m, err := c.Metrics(ctx)
			if err == nil && m.Database.ActiveNotes != 4 {
				return errors.New("bad decode")
			}
			return err
		}, "GET", "/etapi/metrics", "format=json", "", ""},
		{"Backup", 204, "", func(c *Client, ctx context.Context) error { return c.Backup(ctx, "now") }, "PUT", "/etapi/backup/now", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newServer(t, tc.status, tc.resp)
			if err := tc.call(c, t.Context()); err != nil {
				t.Fatalf("call: %v", err)
			}
			if rec.method != tc.method || rec.path != tc.path || rec.query != tc.query {
				t.Errorf("got %s %s?%s, want %s %s?%s", rec.method, rec.path, rec.query, tc.method, tc.path, tc.query)
			}
			if rec.contentType != tc.contentType {
				t.Errorf("content-type %q, want %q", rec.contentType, tc.contentType)
			}
			if !strings.Contains(rec.body, tc.bodyMatch) {
				t.Errorf("body %q does not contain %q", rec.body, tc.bodyMatch)
			}
			if rec.auth != "tok" || rec.ua != "trilium-cli/test" {
				t.Errorf("headers: auth=%q ua=%q", rec.auth, rec.ua)
			}
		})
	}
}

func TestCreateBranchStatus(t *testing.T) {
	for _, tc := range []struct {
		status  int
		created bool
	}{{201, true}, {200, false}} {
		c, _ := newServer(t, tc.status, `{"branchId":"p_n"}`)
		_, created, err := c.CreateBranch(t.Context(), BranchCreate{NoteID: "n", ParentNoteID: "p"})
		if err != nil || created != tc.created {
			t.Errorf("status %d: created=%v err=%v", tc.status, created, err)
		}
	}
}

// gzipServer always answers gzip, ignoring Accept-Encoding, like Trilium does.
func gzipServer(t *testing.T, payload string, forceIdentityTransport bool) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = io.WriteString(zw, payload)
		_ = zw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)
	h := &http.Client{}
	if forceIdentityTransport {
		h.Transport = &http.Transport{DisableCompression: true}
	}
	return New(srv.URL+"/etapi", "tok", WithHTTPClient(h))
}

func TestForcedGzip(t *testing.T) {
	payload := `{"appVersion":"` + strings.Repeat("x", 2000) + `"}`
	for _, identity := range []bool{false, true} {
		c := gzipServer(t, payload, identity)
		a, err := c.AppInfo(t.Context())
		if err != nil {
			t.Fatalf("identity=%v: %v", identity, err)
		}
		if len(a.AppVersion) != 2000 {
			t.Errorf("identity=%v: got %d bytes", identity, len(a.AppVersion))
		}
	}
}

func TestErrorDecoding(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   Error
	}{
		{404, `{"status":404,"code":"NOTE_NOT_FOUND","message":"Note 'x' not found."}`, Error{404, "NOTE_NOT_FOUND", "Note 'x' not found."}},
		{502, `<html><body>Bad Gateway</body></html>`, Error{502, "", "HTTP 502 from server"}},
		{500, ``, Error{500, "", "HTTP 500 from server"}},
		{400, `{"code":"X","message":"m"}`, Error{400, "X", "m"}},
	}
	for _, tc := range cases {
		c, _ := newServer(t, tc.status, tc.body)
		err := c.DeleteNote(t.Context(), "x")
		var e *Error
		if !errors.As(err, &e) {
			t.Fatalf("status %d: got %T %v", tc.status, err, err)
		}
		if *e != tc.want {
			t.Errorf("got %+v, want %+v", *e, tc.want)
		}
	}
	e := &Error{404, "NOTE_NOT_FOUND", "Note 'x' not found."}
	if !e.NotFound() || e.Error() != "Note 'x' not found. [NOTE_NOT_FOUND]" {
		t.Errorf("Error()=%q NotFound=%v", e.Error(), e.NotFound())
	}
	if (&Error{Message: "m"}).Error() != "m" {
		t.Error("message without code")
	}
}

func TestBadJSON(t *testing.T) {
	c, _ := newServer(t, 200, `not json`)
	if _, err := c.GetNote(t.Context(), "x"); err == nil {
		t.Fatal("want error")
	}
}

func flakyServer(t *testing.T, failures int32, status int) (*Client, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) <= failures {
			w.WriteHeader(status)
			return
		}
		_, _ = io.WriteString(w, `{"appVersion":"1"}`)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL+"/etapi", "tok", WithRetryDelay(time.Millisecond)), &n
}

func TestRetryGET(t *testing.T) {
	c, n := flakyServer(t, 1, 503)
	if _, err := c.AppInfo(t.Context()); err != nil {
		t.Fatalf("want success after retry, got %v", err)
	}
	if n.Load() != 2 {
		t.Errorf("calls=%d", n.Load())
	}
	c, n = flakyServer(t, 5, 504)
	if _, err := c.AppInfo(t.Context()); err == nil {
		t.Fatal("want failure after 3 attempts")
	}
	if n.Load() != 3 {
		t.Errorf("calls=%d, want 3", n.Load())
	}
	c, n = flakyServer(t, 1, 500)
	if _, err := c.AppInfo(t.Context()); err == nil || n.Load() != 1 {
		t.Errorf("500 must not be retried: calls=%d err=%v", n.Load(), err)
	}
}

func TestNoRetryPOST(t *testing.T) {
	c, n := flakyServer(t, 1, 503)
	if err := c.CreateRevision(t.Context(), "x"); err == nil {
		t.Fatal("want error")
	}
	if n.Load() != 1 {
		t.Errorf("POST retried: calls=%d", n.Load())
	}
}

func TestNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	u := srv.URL
	srv.Close()
	c := New(u+"/etapi", "tok", WithRetryDelay(time.Millisecond))
	_, err := c.AppInfo(t.Context())
	var ne *NetworkError
	if !errors.As(err, &ne) || ne.URL != u+"/etapi" || ne.Unwrap() == nil || ne.Error() == "" {
		t.Fatalf("got %T %v", err, err)
	}
	if c.BaseURL() != u+"/etapi" {
		t.Error("BaseURL")
	}
}

func TestContextCanceled(t *testing.T) {
	c, _ := flakyServer(t, 5, 503)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.AppInfo(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestBool(t *testing.T) {
	var rc []RecentChange
	err := json.Unmarshal([]byte(`[{"current_isDeleted":1,"current_isProtected":0,"canBeUndeleted":true},{"current_isDeleted":false,"canBeUndeleted":null}]`), &rc)
	if err != nil || !bool(rc[0].CurrentIsDeleted) || bool(rc[0].CurrentIsProtected) || !bool(rc[0].CanBeUndeleted) || bool(rc[1].CurrentIsDeleted) {
		t.Fatalf("%+v %v", rc, err)
	}
	var b Bool
	if err := json.Unmarshal([]byte(`"yes"`), &b); err == nil {
		t.Error("want error")
	}
}

func TestNetworkErrorMaybeApplied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		_ = conn.Close() // the request arrived, the answer never will
	}))
	c := New(srv.URL+"/etapi", "tok", WithRetryDelay(time.Millisecond))
	var ne *NetworkError

	err := c.PutContent(t.Context(), "abcd", strings.NewReader("x"), false)
	if !errors.As(err, &ne) || !ne.MaybeApplied {
		t.Errorf("write dropped after sending: got %v, want MaybeApplied", err)
	}
	_, err = c.GetContent(t.Context(), "abcd")
	if !errors.As(err, &ne) || ne.MaybeApplied {
		t.Errorf("read is never 'maybe applied': got %v", err)
	}

	srv.Close() // now nothing listens: the request can't have been sent
	err = c.PutContent(t.Context(), "abcd", strings.NewReader("x"), false)
	if !errors.As(err, &ne) || ne.MaybeApplied {
		t.Errorf("connection refused: got %v, want not MaybeApplied", err)
	}
}

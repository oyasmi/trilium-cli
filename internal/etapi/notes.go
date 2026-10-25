package etapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// CreateNote calls POST /create-note.
func (c *Client) CreateNote(ctx context.Context, def CreateNoteDef) (*NoteWithBranch, error) {
	var out NoteWithBranch
	if _, err := c.sendJSON(ctx, http.MethodPost, "/create-note", def, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetNote calls GET /notes/{id}.
func (c *Client) GetNote(ctx context.Context, id string) (*Note, error) {
	var out Note
	if err := c.getJSON(ctx, "/notes/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchNote calls PATCH /notes/{id}.
func (c *Client) PatchNote(ctx context.Context, id string, p NotePatch) (*Note, error) {
	var out Note
	if _, err := c.sendJSON(ctx, http.MethodPatch, "/notes/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteNote calls DELETE /notes/{id}.
func (c *Client) DeleteNote(ctx context.Context, id string) error {
	_, err := c.sendJSON(ctx, http.MethodDelete, "/notes/"+esc(id), nil, nil)
	return err
}

// GetContent calls GET /notes/{id}/content and returns the raw bytes.
func (c *Client) GetContent(ctx context.Context, id string) ([]byte, error) {
	_, body, err := c.do(ctx, request{method: http.MethodGet, path: "/notes/" + esc(id) + "/content"})
	return body, err
}

// PutContent calls PUT /notes/{id}/content. Binary content is sent as
// application/octet-stream, everything else as text/plain.
func (c *Client) PutContent(ctx context.Context, id string, body io.Reader, binary bool) error {
	_, _, err := c.do(ctx, request{method: http.MethodPut, path: "/notes/" + esc(id) + "/content", body: body, contentType: contentType(binary)})
	return err
}

// Search calls GET /notes.
func (c *Client) Search(ctx context.Context, p SearchParams) ([]Note, error) {
	q := url.Values{}
	q.Set("search", p.Search)
	if p.FastSearch {
		q.Set("fastSearch", "true")
	}
	if p.IncludeArchivedNotes {
		q.Set("includeArchivedNotes", "true")
	}
	if p.AncestorNoteID != "" {
		q.Set("ancestorNoteId", p.AncestorNoteID)
	}
	if p.AncestorDepth != "" {
		q.Set("ancestorDepth", p.AncestorDepth)
	}
	if p.OrderBy != "" {
		q.Set("orderBy", p.OrderBy)
	}
	if p.OrderDirection != "" {
		q.Set("orderDirection", p.OrderDirection)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	var out SearchResponse
	if err := c.getJSON(ctx, "/notes", q, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// Export calls GET /notes/{id}/export and returns the zip bytes.
func (c *Client) Export(ctx context.Context, id, format string) ([]byte, error) {
	q := url.Values{}
	if format != "" {
		q.Set("format", format)
	}
	_, body, err := c.do(ctx, request{method: http.MethodGet, path: "/notes/" + esc(id) + "/export", query: q})
	return body, err
}

// Import calls POST /notes/{id}/import with a zip file.
func (c *Client) Import(ctx context.Context, parentID string, zip io.Reader) (*NoteWithBranch, error) {
	_, body, err := c.do(ctx, request{method: http.MethodPost, path: "/notes/" + esc(parentID) + "/import", body: zip, contentType: "application/octet-stream"})
	if err != nil {
		return nil, err
	}
	var out NoteWithBranch
	if err := decode(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateRevision calls POST /notes/{id}/revision (204, no body).
func (c *Client) CreateRevision(ctx context.Context, id string) error {
	_, err := c.sendJSON(ctx, http.MethodPost, "/notes/"+esc(id)+"/revision", nil, nil)
	return err
}

// ListRevisions calls GET /notes/{id}/revisions.
func (c *Client) ListRevisions(ctx context.Context, id string) ([]Revision, error) {
	var out []Revision
	if err := c.getJSON(ctx, "/notes/"+esc(id)+"/revisions", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAttachments calls GET /notes/{id}/attachments.
func (c *Client) ListAttachments(ctx context.Context, id string) ([]Attachment, error) {
	var out []Attachment
	if err := c.getJSON(ctx, "/notes/"+esc(id)+"/attachments", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Undelete calls POST /notes/{id}/undelete.
func (c *Client) Undelete(ctx context.Context, id string) error {
	_, err := c.sendJSON(ctx, http.MethodPost, "/notes/"+esc(id)+"/undelete", nil, nil)
	return err
}

// History calls GET /notes/history.
func (c *Client) History(ctx context.Context, ancestorID string) ([]RecentChange, error) {
	q := url.Values{}
	if ancestorID != "" {
		q.Set("ancestorNoteId", ancestorID)
	}
	var out []RecentChange
	if err := c.getJSON(ctx, "/notes/history", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRevision calls GET /revisions/{id}.
func (c *Client) GetRevision(ctx context.Context, id string) (*Revision, error) {
	var out Revision
	if err := c.getJSON(ctx, "/revisions/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRevisionContent calls GET /revisions/{id}/content.
func (c *Client) GetRevisionContent(ctx context.Context, id string) ([]byte, error) {
	_, body, err := c.do(ctx, request{method: http.MethodGet, path: "/revisions/" + esc(id) + "/content"})
	return body, err
}

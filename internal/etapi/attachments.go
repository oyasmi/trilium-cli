package etapi

import (
	"context"
	"io"
	"net/http"
)

// CreateAttachment calls POST /attachments.
func (c *Client) CreateAttachment(ctx context.Context, a AttachmentCreate) (*Attachment, error) {
	var out Attachment
	if _, err := c.sendJSON(ctx, http.MethodPost, "/attachments", a, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAttachment calls GET /attachments/{id}.
func (c *Client) GetAttachment(ctx context.Context, id string) (*Attachment, error) {
	var out Attachment
	if err := c.getJSON(ctx, "/attachments/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchAttachment calls PATCH /attachments/{id}.
func (c *Client) PatchAttachment(ctx context.Context, id string, p AttachmentPatch) (*Attachment, error) {
	var out Attachment
	if _, err := c.sendJSON(ctx, http.MethodPatch, "/attachments/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAttachment calls DELETE /attachments/{id}.
func (c *Client) DeleteAttachment(ctx context.Context, id string) error {
	_, err := c.sendJSON(ctx, http.MethodDelete, "/attachments/"+esc(id), nil, nil)
	return err
}

// GetAttachmentContent calls GET /attachments/{id}/content.
func (c *Client) GetAttachmentContent(ctx context.Context, id string) ([]byte, error) {
	_, body, err := c.do(ctx, request{method: http.MethodGet, path: "/attachments/" + esc(id) + "/content"})
	return body, err
}

// PutAttachmentContent calls PUT /attachments/{id}/content.
func (c *Client) PutAttachmentContent(ctx context.Context, id string, body io.Reader, binary bool) error {
	_, _, err := c.do(ctx, request{method: http.MethodPut, path: "/attachments/" + esc(id) + "/content", body: body, contentType: contentType(binary)})
	return err
}

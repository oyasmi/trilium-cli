package etapi

import (
	"context"
	"net/http"
)

// CreateAttribute calls POST /attributes.
func (c *Client) CreateAttribute(ctx context.Context, a AttributeCreate) (*Attribute, error) {
	var out Attribute
	if _, err := c.sendJSON(ctx, http.MethodPost, "/attributes", a, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAttribute calls GET /attributes/{id}.
func (c *Client) GetAttribute(ctx context.Context, id string) (*Attribute, error) {
	var out Attribute
	if err := c.getJSON(ctx, "/attributes/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchAttribute calls PATCH /attributes/{id}.
func (c *Client) PatchAttribute(ctx context.Context, id string, p AttributePatch) (*Attribute, error) {
	var out Attribute
	if _, err := c.sendJSON(ctx, http.MethodPatch, "/attributes/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAttribute calls DELETE /attributes/{id}.
func (c *Client) DeleteAttribute(ctx context.Context, id string) error {
	_, err := c.sendJSON(ctx, http.MethodDelete, "/attributes/"+esc(id), nil, nil)
	return err
}

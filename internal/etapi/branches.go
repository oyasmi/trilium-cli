package etapi

import (
	"context"
	"net/http"
)

// CreateBranch calls POST /branches. created is false when the server
// answered 200, meaning the branch already existed and was updated.
func (c *Client) CreateBranch(ctx context.Context, b BranchCreate) (branch *Branch, created bool, err error) {
	var out Branch
	status, err := c.sendJSON(ctx, http.MethodPost, "/branches", b, &out)
	if err != nil {
		return nil, false, err
	}
	return &out, status == http.StatusCreated, nil
}

// GetBranch calls GET /branches/{id}.
func (c *Client) GetBranch(ctx context.Context, id string) (*Branch, error) {
	var out Branch
	if err := c.getJSON(ctx, "/branches/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PatchBranch calls PATCH /branches/{id}.
func (c *Client) PatchBranch(ctx context.Context, id string, p BranchPatch) (*Branch, error) {
	var out Branch
	if _, err := c.sendJSON(ctx, http.MethodPatch, "/branches/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBranch calls DELETE /branches/{id}.
func (c *Client) DeleteBranch(ctx context.Context, id string) error {
	_, err := c.sendJSON(ctx, http.MethodDelete, "/branches/"+esc(id), nil, nil)
	return err
}

// RefreshNoteOrdering calls POST /refresh-note-ordering/{parentId}.
func (c *Client) RefreshNoteOrdering(ctx context.Context, parentID string) error {
	_, err := c.sendJSON(ctx, http.MethodPost, "/refresh-note-ordering/"+esc(parentID), nil, nil)
	return err
}

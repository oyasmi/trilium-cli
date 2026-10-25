package etapi

import (
	"context"
	"net/http"
	"net/url"
)

// AppInfo calls GET /app-info.
func (c *Client) AppInfo(ctx context.Context) (*AppInfo, error) {
	var out AppInfo
	if err := c.getJSON(ctx, "/app-info", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Metrics calls GET /metrics?format=json.
func (c *Client) Metrics(ctx context.Context) (*Metrics, error) {
	var out Metrics
	if err := c.getJSON(ctx, "/metrics", url.Values{"format": {"json"}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Backup calls PUT /backup/{name}; the server writes backup-<name>.db.
func (c *Client) Backup(ctx context.Context, name string) error {
	_, err := c.sendJSON(ctx, http.MethodPut, "/backup/"+esc(name), nil, nil)
	return err
}

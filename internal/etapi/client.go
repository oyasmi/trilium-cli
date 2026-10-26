// Package etapi is a thin client for Trilium's ETAPI. Every method maps to
// exactly one endpoint and contains no business logic.
package etapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to one Trilium server.
type Client struct {
	base       string // ends with /etapi, no trailing slash
	token      string
	http       *http.Client
	userAgent  string
	retries    int
	retryDelay time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default *http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// WithRetryDelay sets the base delay between GET retries (doubles each time).
func WithRetryDelay(d time.Duration) Option { return func(c *Client) { c.retryDelay = d } }

// New returns a client for baseURL, which must already end with /etapi.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		base:       strings.TrimRight(baseURL, "/"),
		token:      token,
		http:       defaultHTTPClient(),
		userAgent:  "trilium-cli/dev",
		retries:    2,
		retryDelay: 400 * time.Millisecond,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// defaultHTTPClient bounds connecting and waiting for the response headers
// instead of the whole exchange, so that a large export, import or upload
// is not cut off while data is still flowing.
func defaultHTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 2 * time.Minute
	t.TLSHandshakeTimeout = 30 * time.Second
	return &http.Client{Transport: t}
}

// BaseURL returns the normalized ETAPI base URL.
func (c *Client) BaseURL() string { return c.base }

// Error is an error response from the server.
type Error struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s [%s]", e.Message, e.Code)
}

// NotFound reports whether the error code is one of the *_NOT_FOUND codes.
func (e *Error) NotFound() bool { return strings.HasSuffix(e.Code, "_NOT_FOUND") }

// NetworkError means the server could not be reached.
type NetworkError struct {
	URL string
	Err error
	// MaybeApplied is set for a write whose request may have reached the
	// server (the connection was made, then broke or timed out).
	MaybeApplied bool
}

func (e *NetworkError) Error() string { return fmt.Sprintf("could not reach %s: %v", e.URL, e.Err) }
func (e *NetworkError) Unwrap() error { return e.Err }

type request struct {
	method      string
	path        string
	query       url.Values
	body        io.Reader
	contentType string
}

// do performs a request and returns the status and the full (decoded) body.
// Non-2xx responses are turned into *Error.
func (c *Client) do(ctx context.Context, r request) (int, []byte, error) {
	u := c.base + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}
	attempts := 1
	if r.method == http.MethodGet {
		attempts += c.retries
	}
	var lastErr error
	for i := range attempts {
		if i > 0 {
			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			case <-time.After(c.retryDelay << (i - 1)):
			}
		}
		status, body, err := c.once(ctx, r, u)
		if err == nil {
			return status, body, nil
		}
		lastErr = err
		if !retryable(err) {
			break
		}
	}
	return 0, nil, lastErr
}

// isDialError reports whether the connection was never made, so the
// request cannot have reached the server.
func isDialError(err error) bool {
	var oe *net.OpError
	return errors.As(err, &oe) && oe.Op == "dial"
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var ne *NetworkError
	if errors.As(err, &ne) {
		return true
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Status == 502 || ae.Status == 503 || ae.Status == 504
	}
	return false
}

func (c *Client) once(ctx context.Context, r request, u string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, r.method, u, r.body)
	if err != nil {
		return 0, nil, err
	}
	// Accept-Encoding is deliberately left unset: the transport then asks for
	// gzip and decodes it, which matters because Trilium gzips anything over
	// ~1KB regardless of what the client asks for.
	req.Header.Set("Authorization", c.token)
	req.Header.Set("User-Agent", c.userAgent)
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, &NetworkError{URL: c.base, Err: err, MaybeApplied: r.method != http.MethodGet && !isDialError(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	var rd io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		// Only reached when the transport did not decode it for us.
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return 0, nil, &NetworkError{URL: c.base, Err: fmt.Errorf("bad gzip response: %w", err)}
		}
		defer func() { _ = gz.Close() }()
		rd = gz
	}
	body, err := io.ReadAll(rd)
	if err != nil {
		return 0, nil, &NetworkError{URL: c.base, Err: err, MaybeApplied: r.method != http.MethodGet}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, body, nil
	}
	return resp.StatusCode, nil, decodeError(resp.StatusCode, body)
}

func decodeError(status int, body []byte) error {
	var e Error
	if err := json.Unmarshal(body, &e); err == nil && (e.Code != "" || e.Message != "") {
		if e.Status == 0 {
			e.Status = status
		}
		return &e
	}
	return &Error{Status: status, Message: fmt.Sprintf("HTTP %d from server", status)}
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	_, body, err := c.do(ctx, request{method: http.MethodGet, path: path, query: q})
	if err != nil {
		return err
	}
	return decode(body, out)
}

func (c *Client) sendJSON(ctx context.Context, method, path string, in, out any) (int, error) {
	var rd io.Reader
	ct := ""
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		rd = bytes.NewReader(b)
		ct = "application/json"
	}
	status, body, err := c.do(ctx, request{method: method, path: path, body: rd, contentType: ct})
	if err != nil {
		return 0, err
	}
	if out != nil {
		return status, decode(body, out)
	}
	return status, nil
}

func decode(body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("unexpected response from server: %w", err)
	}
	return nil
}

func esc(id string) string { return url.PathEscape(id) }

func contentType(binary bool) string {
	if binary {
		return "application/octet-stream"
	}
	return "text/plain; charset=utf-8"
}

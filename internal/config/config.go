// Package config reads the CLI's settings from environment variables.
package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Config holds the settings needed to talk to a Trilium server.
type Config struct {
	URL    string // ETAPI base, always ending with /etapi
	Server string // server address as shown to the user (without /etapi)
	Token  string
	Agent  string
}

// Error is a configuration problem with a hint on how to fix it.
type Error struct {
	Msg  string
	Hint string
}

func (e *Error) Error() string { return e.Msg }

// Load reads TRILIUM_URL, TRILIUM_ETAPI_TOKEN and TRILIUM_AGENT through getenv.
func Load(getenv func(string) string) (*Config, error) {
	raw := strings.TrimSpace(getenv("TRILIUM_URL"))
	if raw == "" {
		return nil, &Error{
			Msg:  "TRILIUM_URL is not set.",
			Hint: "Set it to your Trilium server address, e.g.:\n      export TRILIUM_URL=https://notes.example.com",
		}
	}
	server, err := NormalizeURL(raw)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(getenv("TRILIUM_ETAPI_TOKEN"))
	if token == "" {
		return nil, &Error{
			Msg:  "TRILIUM_ETAPI_TOKEN is not set.",
			Hint: "In Trilium, open Options → ETAPI and create a token, then:\n      export TRILIUM_ETAPI_TOKEN=<your token>",
		}
	}
	return &Config{
		URL:    server + "/etapi",
		Server: server,
		Token:  token,
		Agent:  strings.TrimSpace(getenv("TRILIUM_AGENT")),
	}, nil
}

// NormalizeURL validates a server address and strips any trailing "/" and
// "/etapi", returning the bare server address.
func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", &Error{
			Msg:  fmt.Sprintf("TRILIUM_URL is not a valid http(s) address: %q", raw),
			Hint: "Use the address you open Trilium with, e.g.:\n      export TRILIUM_URL=https://notes.example.com",
		}
	}
	u.RawQuery, u.Fragment = "", ""
	s := strings.TrimRight(u.String(), "/")
	s = strings.TrimSuffix(s, "/etapi")
	return strings.TrimRight(s, "/"), nil
}

package config

import (
	"errors"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"https://notes.example.com":           "https://notes.example.com",
		"https://notes.example.com/":          "https://notes.example.com",
		"https://notes.example.com/etapi":     "https://notes.example.com",
		"https://notes.example.com/etapi/":    "https://notes.example.com",
		"http://localhost:8080":               "http://localhost:8080",
		"https://example.com/trilium/":        "https://example.com/trilium",
		"https://example.com/trilium/etapi":   "https://example.com/trilium",
		"https://notes.example.com/?x=1#frag": "https://notes.example.com",
		"  https://notes.example.com/etapi  ": "https://notes.example.com",
	}
	for in, want := range cases {
		cfg, err := Load(env(map[string]string{"TRILIUM_URL": in, "TRILIUM_ETAPI_TOKEN": "t"}))
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if cfg.Server != want || cfg.URL != want+"/etapi" {
			t.Errorf("%q: got %q / %q", in, cfg.Server, cfg.URL)
		}
	}
}

func TestMissing(t *testing.T) {
	_, err := Load(env(nil))
	var ce *Error
	if !errors.As(err, &ce) || ce.Error() != "TRILIUM_URL is not set." || ce.Hint == "" {
		t.Fatalf("got %v", err)
	}
	_, err = Load(env(map[string]string{"TRILIUM_URL": "https://x.org"}))
	if !errors.As(err, &ce) || ce.Msg != "TRILIUM_ETAPI_TOKEN is not set." {
		t.Fatalf("got %v", err)
	}
	if ce.Hint != "In Trilium, open Options → ETAPI and create a token, then:\n      export TRILIUM_ETAPI_TOKEN=<your token>" {
		t.Errorf("hint %q", ce.Hint)
	}
}

func TestInvalid(t *testing.T) {
	for _, in := range []string{"notes.example.com", "ftp://x.org", "https://", "://bad", "http://%zz"} {
		_, err := Load(env(map[string]string{"TRILIUM_URL": in, "TRILIUM_ETAPI_TOKEN": "t"}))
		var ce *Error
		if !errors.As(err, &ce) {
			t.Errorf("%q: want config error, got %v", in, err)
		}
	}
}

func TestAgent(t *testing.T) {
	cfg, err := Load(env(map[string]string{"TRILIUM_URL": "https://x.org", "TRILIUM_ETAPI_TOKEN": " t ", "TRILIUM_AGENT": "claude-1"}))
	if err != nil || cfg.Agent != "claude-1" || cfg.Token != "t" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

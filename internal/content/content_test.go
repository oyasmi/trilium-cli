package content

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oyasmi/trilium-cli/internal/testutil"
)

func TestRead(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "body.html")
	if err := os.WriteFile(f, []byte("<p>file</p>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdin := strings.NewReader("from stdin")

	b, src, err := Read(Input{Arg: "", HasArg: true}, stdin)
	if err != nil || src != Arg || string(b) != "" {
		t.Errorf("empty arg: %q %v %v", b, src, err)
	}
	b, src, err = Read(Input{File: f}, stdin)
	if err != nil || src != File || string(b) != "<p>file</p>\n" {
		t.Errorf("file: %q %v %v", b, src, err)
	}
	b, src, err = Read(Input{File: "-"}, stdin)
	if err != nil || src != Stdin || string(b) != "from stdin" {
		t.Errorf("stdin: %q %v %v", b, src, err)
	}
	// stdin is never read implicitly.
	b, src, err = Read(Input{}, errReader{})
	if err != nil || src != None || b != nil {
		t.Errorf("none: %q %v %v", b, src, err)
	}
	if _, _, err := Read(Input{Arg: "x", HasArg: true, File: f}, stdin); !errors.Is(err, ErrTooManySources) {
		t.Errorf("two sources: %v", err)
	}
	if _, _, err := Read(Input{File: filepath.Join(dir, "missing")}, stdin); err == nil {
		t.Error("missing file")
	}
	if _, _, err := Read(Input{File: "-"}, errReader{}); err == nil {
		t.Error("stdin error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("must not read") }

func TestInferFromFile(t *testing.T) {
	cases := []struct{ file, typ, mime string }{
		{"a.html", "text", "text/html"},
		{"a.HTM", "text", "text/html"},
		{"notes.md", "code", "text/x-markdown"},
		{"notes.markdown", "code", "text/x-markdown"},
		{"flow.mmd", "mermaid", "text/vnd.mermaid"},
		{"flow.mermaid", "mermaid", "text/vnd.mermaid"},
		{"a.txt", "code", "text/plain"},
		{"server.log", "code", "text/plain"},
		{"hello.py", "code", "text/x-python"},
		{"a.js", "code", "text/javascript"},
		{"a.mjs", "code", "text/javascript"},
		{"a.ts", "code", "application/typescript"},
		{"a.tsx", "code", "text/typescript-jsx"},
		{"a.jsx", "code", "text/jsx"},
		{"a.json", "code", "application/json"},
		{"a.yaml", "code", "text/x-yaml"},
		{"a.yml", "code", "text/x-yaml"},
		{"a.toml", "code", "text/x-toml"},
		{"a.xml", "code", "text/xml"},
		{"a.css", "code", "text/css"},
		{"a.sh", "code", "text/x-sh"},
		{"a.bash", "code", "text/x-sh"},
		{"main.go", "code", "text/x-go"},
		{"lib.rs", "code", "text/x-rustsrc"},
		{"A.java", "code", "text/x-java"},
		{"a.c", "code", "text/x-csrc"},
		{"a.h", "code", "text/x-csrc"},
		{"a.cpp", "code", "text/x-c++src"},
		{"a.cc", "code", "text/x-c++src"},
		{"a.hpp", "code", "text/x-c++src"},
		{"a.cs", "code", "text/x-csharp"},
		{"a.rb", "code", "text/x-ruby"},
		{"a.php", "code", "text/x-php"},
		{"a.sql", "code", "text/x-sql"},
		{"a.kt", "code", "text/x-kotlin"},
		{"a.swift", "code", "text/x-swift"},
		{"a.lua", "code", "text/x-lua"},
		{"a.r", "code", "text/x-rsrc"},
		{"a.diff", "code", "text/x-diff"},
		{"a.patch", "code", "text/x-diff"},
		{"a.tex", "code", "text/x-stex"},
		{"a.vue", "code", "text/x-vue"},
		{"a.scala", "code", "text/x-scala"},
		{"a.hs", "code", "text/x-haskell"},
		{"a.ex", "code", "text/x-elixir"},
		{"a.exs", "code", "text/x-elixir"},
		{"a.pl", "code", "text/x-perl"},
		{"a.ps1", "code", "application/x-powershell"},
		{"a.bat", "code", "application/x-bat"},
		{"a.proto", "code", "text/x-protobuf"},
		{"a.rst", "code", "text/x-rst"},
		{"a.dart", "code", "application/dart"},
		{"a.jl", "code", "text/x-julia"},
		{"a.groovy", "code", "text/x-groovy"},
		{"a.nix", "code", "text/x-nix"},
		{"a.erl", "code", "text/x-erlang"},
		{"a.clj", "code", "text/x-clojure"},
		{"a.ml", "code", "text/x-ocaml"},
		{"a.ini", "code", "text/x-toml"},
		{"a.properties", "code", "text/x-properties"},
		{"Dockerfile", "code", "text/x-dockerfile"},
		{"dir/x.PNG", "image", "image/png"},
		{"a.jpg", "image", "image/jpeg"},
		{"a.jpeg", "image", "image/jpeg"},
		{"a.gif", "image", "image/gif"},
		{"a.webp", "image", "image/webp"},
		{"a.svg", "image", "image/svg+xml"},
		{"a.bmp", "image", "image/bmp"},
		{"a.ico", "image", "image/vnd.microsoft.icon"},
		{"a.avif", "image", "image/avif"},
		{"report.pdf", "file", "application/pdf"},
		{"noext", "file", "application/octet-stream"},
	}
	for _, tc := range cases {
		typ, m := InferFromFile(tc.file, nil)
		if typ != tc.typ || m != tc.mime {
			t.Errorf("%s: got %s %s, want %s %s", tc.file, typ, m, tc.typ, tc.mime)
		}
	}
	if typ, m := InferFromFile("README", []byte("hello world")); typ != "file" || m != "text/plain" {
		t.Errorf("sniffed: %s %s", typ, m)
	}
	if MimeFromTitle("hello.py") != "text/x-python" || MimeFromTitle("Meeting notes") != "" || MimeFromTitle("python") != "" || MimeFromTitle("trailing.") != "" {
		t.Error("MimeFromTitle")
	}
}

func TestIsBinary(t *testing.T) {
	cases := []struct {
		typ, mime string
		data      []byte
		want      bool
	}{
		{"image", "image/png", nil, true},
		{"image", "image/svg+xml", []byte("<svg/>"), true},
		{"file", "application/pdf", nil, true},
		{"file", "text/csv", nil, false},
		{"file", "application/json", nil, false},
		{"file", "application/xml", nil, false},
		{"file", "application/javascript", nil, false},
		{"file", "text/plain", []byte{0xff, 0xfe}, true},
		{"text", "text/html", []byte("<p>ok</p>"), false},
		{"code", "text/plain", []byte{0xc3, 0x28}, true},
		{"code", "text/x-python", nil, false},
	}
	for _, tc := range cases {
		if got := IsBinary(tc.typ, tc.mime, tc.data); got != tc.want {
			t.Errorf("%s %s: got %v", tc.typ, tc.mime, got)
		}
	}
	if !BinaryType("image", "image/png") || BinaryType("text", "text/html") {
		t.Error("BinaryType")
	}
}

func TestCodeBlockClass(t *testing.T) {
	cases := map[string]string{
		"text/x-python":                      "language-text-x-python",
		"text/x-c++src":                      "language-text-x-c-src",
		"application/javascript;env=backend": "language-application-javascript-env-backend",
		"text/x-sqlite;schema=trilium":       "language-text-x-sqlite-schema-trilium",
		"Text/X_Go":                          "language-text-x-go",
	}
	for in, want := range cases {
		if got := CodeBlockClass(in); got != want {
			t.Errorf("%s: %s", in, got)
		}
	}
}

func TestHasHTMLTags(t *testing.T) {
	yes := []string{"<p>x</p>", "a<br>b", "<br/>", `<img src="x">`, "</div>", "<h2 class='a'>t</h2>"}
	no := []string{"plain text", "a < b > c", "1<2", "<>", "x <- y", "email <me@example.com>"}
	for _, s := range yes {
		if !HasHTMLTags(s) {
			t.Errorf("want tags: %q", s)
		}
	}
	for _, s := range no {
		if HasHTMLTags(s) {
			t.Errorf("want no tags: %q", s)
		}
	}
}

// sampleHTML covers every element of design §6.1, as Trilium writes them.
const sampleHTML = `<h2>Meeting &amp; notes</h2><p>Some <strong>bold</strong>, <em>italic</em>, <code>code</code> and <del>gone</del> text with an <a href="https://example.com">external link</a>.</p>
<p>See <a class="reference-link" href="#root/abc123/tText01">Other note</a> and the <a href="#root">root</a>.</p>
<h3>Lists</h3>
<ul><li>first<ul><li>nested one</li><li>nested two</li></ul></li><li>second</li></ul>
<ol><li>step one</li><li>step two<ol><li>sub step</li></ol></li><li><p>step three</p></li></ol>
<ul class="todo-list"><li><label class="todo-list__label"><input type="checkbox" disabled="disabled"><span class="todo-list__label__description">open task</span></label></li><li><label class="todo-list__label"><input type="checkbox" checked="checked" disabled="disabled"><span class="todo-list__label__description">done task</span></label></li></ul>
<pre><code class="language-text-x-python">def f(x):
    return x  *  2
</code></pre>
<figure class="table"><table><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody><tr><td>a</td><td><p>1</p></td></tr><tr><td>b &lt;c&gt;</td><td>2</td></tr></tbody></table></figure>
<blockquote><p>A quote.</p></blockquote>
<p><span class="math-tex">\(x^2\)</span></p>
<aside class="admonition note"><p>Heads up.</p></aside>
<hr>
<p>Line one<br>line two</p>
<p><img src="api/attachments/Kq9xYz1a/image/red.png"></p>



<h4>End</h4><script>alert(1)</script>`

func TestPlainTextGolden(t *testing.T) {
	testutil.Golden(t, "plaintext", PlainText(sampleHTML))
}

func TestPlainTextSmall(t *testing.T) {
	cases := map[string]string{
		"":                             "",
		"just text":                    "just text\n",
		"<p>a</p><p>b</p>":             "a\nb\n",
		"<p>  spaced   out  </p>":      "spaced out\n",
		"<li>loose item</li>":          "- loose item\n",
		"<p>a &nbsp; b</p>":            "a   b\n",
		"<ul><li></li><li>x</li></ul>": "-\n- x\n",
	}
	for in, want := range cases {
		if got := PlainText(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

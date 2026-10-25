package content

import (
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Note types the CLI infers from files.
const (
	TypeText    = "text"
	TypeCode    = "code"
	TypeMermaid = "mermaid"
	TypeImage   = "image"
	TypeFile    = "file"
)

// overrides take precedence over Trilium's getMimeTypeFromFileName(), which
// either knows nothing about these extensions or maps .md to text/x-gfm
// (not enabled in Trilium's code-note dropdown by default).
var overrides = map[string]string{
	"md":       "text/x-markdown",
	"markdown": "text/x-markdown",
	"txt":      "text/plain",
	"log":      "text/plain",
	"text":     "text/plain",
	"patch":    "text/x-diff",
	"proto":    "text/x-protobuf",
	"jl":       "text/x-julia",
	"ml":       "text/x-ocaml",
}

var imageMimes = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
	"svg":  "image/svg+xml",
	"bmp":  "image/bmp",
	"ico":  "image/vnd.microsoft.icon",
	"avif": "image/avif",
}

// extension returns the lower-case extension without the dot, or the whole
// lower-case name for well-known extension-less files such as Dockerfile.
func extension(name string) (string, bool) {
	base := strings.ToLower(filepath.Base(name))
	if i := strings.LastIndexByte(base, '.'); i >= 0 && i < len(base)-1 {
		return base[i+1:], true
	}
	if base == "dockerfile" {
		return base, true
	}
	return "", false
}

// CodeMime returns the code-note mime Trilium associates with a file name
// (e.g. "hello.py" → "text/x-python"), or "" when unknown.
func CodeMime(name string) string {
	ext, ok := extension(name)
	if !ok {
		return ""
	}
	if m, ok := overrides[ext]; ok {
		return m
	}
	lang := ext
	if l, ok := languagesByExtension[ext]; ok {
		lang = l
	}
	return markdownNames[lang]
}

// MimeFromTitle infers a code mime from a note title such as "hello.py".
func MimeFromTitle(title string) string { return CodeMime(title) }

// InferFromFile picks the note type and mime for a file, following the
// design's table: HTML → text, Mermaid → mermaid, known code → code,
// images → image, anything else → file.
func InferFromFile(path string, data []byte) (typ, mimeType string) {
	ext, _ := extension(path)
	switch ext {
	case "html", "htm":
		return TypeText, "text/html"
	case "mmd", "mermaid":
		return TypeMermaid, "text/vnd.mermaid"
	}
	if m, ok := imageMimes[ext]; ok {
		return TypeImage, m
	}
	if m := CodeMime(path); m != "" {
		return TypeCode, m
	}
	if ext != "" {
		if m := mime.TypeByExtension("." + ext); m != "" {
			return TypeFile, stripParams(m)
		}
	}
	if len(data) > 0 {
		return TypeFile, stripParams(http.DetectContentType(data))
	}
	return TypeFile, "application/octet-stream"
}

func stripParams(m string) string {
	if i := strings.IndexByte(m, ';'); i >= 0 {
		m = m[:i]
	}
	return strings.TrimSpace(m)
}

// TextMime reports whether a mime denotes text: text/*, JSON, XML or JavaScript.
func TextMime(m string) bool {
	m = strings.ToLower(m)
	return strings.HasPrefix(m, "text/") || strings.Contains(m, "json") ||
		strings.Contains(m, "xml") || strings.Contains(m, "javascript")
}

// IsBinary decides whether content must never be printed to a terminal.
// data may be nil when the content has not been downloaded.
func IsBinary(typ, mimeType string, data []byte) bool {
	switch {
	case typ == TypeImage:
		return true
	case typ == TypeFile && !TextMime(mimeType):
		return true
	}
	return data != nil && !utf8.Valid(data)
}

// BinaryType reports whether a note of this type and mime holds binary
// content, without looking at the content.
func BinaryType(typ, mimeType string) bool { return IsBinary(typ, mimeType, nil) }

var nonWord = regexp.MustCompile(`[\W_]+`)

// CodeBlockClass returns the class CKEditor uses for a code block of the
// given mime, mirroring Trilium's normalizeMimeTypeForCKEditor().
func CodeBlockClass(mimeType string) string {
	return "language-" + nonWord.ReplaceAllString(strings.ToLower(mimeType), "-")
}

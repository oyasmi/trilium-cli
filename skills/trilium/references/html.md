# Writing HTML for Trilium text notes

Text notes store HTML exactly as written; the CLI never converts it. Write the
same HTML that Trilium's editor (CKEditor) produces, shown below. Anything else
may be reformatted by the editor the next time a person edits the note.

## Contents

- Elements
- A complete example
- Code blocks
- Images and attachments
- Links to other notes
- When not to write HTML

## Elements

| Element | HTML |
|---|---|
| Headings | `<h2>` … `<h6>` (h1 is reserved for the note title; Markdown `#` becomes `<h2>`) |
| Paragraph | `<p>text</p>` |
| Emphasis | `<strong>`, `<em>`, `<code>`, `<del>` |
| Link | `<a href="https://example.com">text</a>` |
| Lists | `<ul><li>a<ul><li>nested</li></ul></li></ul>`, `<ol><li>…</li></ol>` |
| To-do list | see the example below; add `checked="checked"` to the input when done |
| Code block | `<pre><code class="language-text-x-python">…</code></pre>` |
| Table | `<figure class="table"><table><thead>…</thead><tbody>…</tbody></table></figure>` |
| Quote | `<blockquote><p>…</p></blockquote>` |
| Math | `<span class="math-tex">\(x^2\)</span>` |
| Callout | `<aside class="admonition note"><p>…</p></aside>` (note, tip, important, caution, warning) |
| Divider | `<hr>` |
| Image | `<img src="api/attachments/<attachmentId>/image/<file name>">` |
| Note link | `<a href="#root/<noteId>">text</a>` |

Escape `<`, `>` and `&` in text as `&lt;`, `&gt;` and `&amp;`.

## A complete example

```html
<h2>Release plan</h2>
<p>Owner: <strong>Alice</strong>. See <a href="https://example.com/spec">the spec</a>.</p>
<ul>
  <li>Backend<ul><li>API freeze</li></ul></li>
  <li>Frontend</li>
</ul>
<ol><li>Build</li><li>Test</li></ol>
<ul class="todo-list">
  <li><label class="todo-list__label"><input type="checkbox" disabled="disabled"><span class="todo-list__label__description">Write release notes</span></label></li>
  <li><label class="todo-list__label"><input type="checkbox" checked="checked" disabled="disabled"><span class="todo-list__label__description">Tag the release</span></label></li>
</ul>
<pre><code class="language-text-x-sh">make build &amp;&amp; make test</code></pre>
<figure class="table"><table><thead><tr><th>Step</th><th>Owner</th></tr></thead><tbody><tr><td>Build</td><td>Bob</td></tr></tbody></table></figure>
<blockquote><p>Ship small, ship often.</p></blockquote>
<aside class="admonition warning"><p>Freeze starts Friday.</p></aside>
<p><span class="math-tex">\(t_{total} = t_{build} + t_{test}\)</span></p>
<hr>
```

Long HTML is best written to a file and passed with `-f`:
`trilium create "Release plan" -f plan.html -p <parent>`.

## Code blocks

The class is `language-` plus the code mime type, lower-cased, with every run of
non-letters/digits replaced by `-`:

| Language | mime | class |
|---|---|---|
| Python | text/x-python | `language-text-x-python` |
| JavaScript | text/javascript | `language-text-javascript` |
| TypeScript | application/typescript | `language-application-typescript` |
| Shell | text/x-sh | `language-text-x-sh` |
| Go | text/x-go | `language-text-x-go` |
| C++ | text/x-c++src | `language-text-x-c-src` |
| JSON | application/json | `language-application-json` |
| SQL | text/x-sql | `language-text-x-sql` |
| YAML | text/x-yaml | `language-text-x-yaml` |
| unknown / auto-detect | — | `language-text-x-trilium-auto` |

## Images and attachments

1. Upload the image as an attachment of the note:
   `trilium att add <note> -f chart.png`
2. The output ends with an `embed:` line; put that HTML into the note:

```
added attachment Kq9xYz1a to tA1b2C3d: "chart.png" image/png 80 bytes
embed: <img src="api/attachments/Kq9xYz1a/image/chart.png">
```

`trilium append <note> '<p><img src="api/attachments/Kq9xYz1a/image/chart.png"></p>'`

Other files (PDF, CSV …) are attachments too; they are listed with `trilium att ls <note>`.

## Links to other notes

`<a href="#root/<noteId>">text</a>` links to another note (this is what
Trilium's own Markdown import produces). The editor's "reference link" style,
which shows the target note's title, is
`<a class="reference-link" href="#root/<noteId>">title</a>`.

## When not to write HTML

- **Plain text, logs, scripts, data**: create a code note instead, e.g.
  `trilium create notes.md -f notes.md` (a `.md` file becomes a code note with
  Markdown highlighting) or `trilium create "Log" -t code -f out.txt`.
- **You have Markdown and want rich text**: `trilium import <parent> doc.md`.
  Trilium's own importer converts it to HTML like the above.

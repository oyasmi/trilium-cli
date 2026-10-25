---
name: trilium
description: Read, search, create, edit, and organize notes in a Trilium Notes
  knowledge base with the `trilium` CLI. Use when the user mentions Trilium, or
  asks to find, save, update, or organize notes, journals, or knowledge kept in Trilium.
---

# Trilium

`trilium` talks to a Trilium Notes server. List rows are tab-separated with the
note ID first; write commands print one line saying what changed (no need to read back).

## Before you start

Run `trilium info` once. Expected:

```
server:  https://notes.example.com (Trilium 0.106.1)
auth:    ok
notes:   404 active, 147 deleted
agent:   (not set)
```

If it fails, show the user the `error:`/`hint:` lines: usually `TRILIUM_URL` or
`TRILIUM_ETAPI_TOKEN` is missing or wrong (tokens: Trilium → Options → ETAPI).

## Core concepts

- **note**: has an ID (e.g. `hIXMEyPSTX1y`), title, type (text, code, book,
  image, file, …) and content. `root` is the top of the tree.
- **branch**: one place where a note appears in the tree. A note can appear in
  several places (a *clone*); it is still one note.
- **attribute**: `#label=value` or `~relation=<noteId>` on a note; children can
  inherit `(inheritable)` ones. Use labels for status, tags, metadata.
- **attachment**: a file or image stored inside a note.
- Wherever a `<note>` is expected you can also use shortcuts: `@inbox`,
  `@today`, `@yesterday`, `@2026-10-25`, `@2026-W43`, `@2026-10`, `@2026`.
  Calendar shortcuts create the day/month/year note if it doesn't exist yet.

## Content is stored as-is

- **text notes store HTML.** Write HTML like `<p>…</p>`, never Markdown or
  plain lines (the CLI warns if text has no tags). Read
  [references/html.md](references/html.md) before writing anything beyond
  paragraphs and lists.
- **code notes store plain text** (scripts, logs, Markdown source, JSON).
- To get rich text from Markdown: `trilium import <parent> doc.md` (Trilium
  converts it). To keep Markdown as text: `trilium create doc.md -f doc.md`.
- The CLI never rewrites content: what you send is stored byte for byte.

## Everyday commands

```bash
trilium search budget --in <project>         # find notes (Trilium search syntax)
trilium ls <note> -d 2                       # children as a tree
trilium get <id1> <id2>                      # metadata + raw content, several at once
trilium create "Title" -f body.html -p <parent>   # default parent: @inbox
trilium append <note> "<p>New line</p>"      # add to the end (--top: start)
trilium edit <note> --old "<p>a</p>" --new "<p>b</p>"   # exact replace
trilium put <note> -f page.html              # replace the whole content
trilium rm <note>                            # delete (-r with children)
```

All commands: [references/commands.md](references/commands.md) or `trilium help <command>`.

## Common patterns

**1. Find, then read several at once.**

```
$ trilium search '#status=todo' --in <project>
<id1>	text	Budget review
<id2>	text	Launch checklist
$ trilium get <id1> <id2>
---
id: <id1>
title: Budget review
type: text (text/html)
parents: <project>
attrs: #status=todo
children: 0
attachments: 0
created: 2026-10-25 18:39
modified: 2026-10-25 18:39
---
<p>Q3 budget approved.</p>

---
id: <id2>   (one block per note, in argument order)
```

**2. Quick capture.**

```
$ trilium append @today -f note.html
appended <day-id> (@today) +54 bytes: 0 → 54 bytes
$ trilium create "Supplier call" -f note.html -p <project>
created <new-id> text "Supplier call" in <project>
```

**3. Change part of a long note.** Read the raw HTML with `get -c`, then
replace exact fragments; don't rewrite the whole note with `put`.

```
$ trilium get -c <id1>
<p>Q3 budget approved.</p>
$ trilium edit <id1> --old "<p>Q3 budget approved.</p>" --new "<p>Q3 budget approved (final).</p>"
edited <id1> 1 replacement: 26 → 34 bytes
```

**4. Long content goes through a file**: write it locally, then `-f path`
(or `-f -` with a heredoc). Quoting long HTML on the command line is error-prone.

**5. Track state with labels.**

```
$ trilium attr <id1> '#status=done'
set #status=done on <id1>
$ trilium search '#status=todo' --in <project>
```

**6. Organize.** `ls` lists what is *inside* a note (not the note itself);
with `-d N` deeper levels are indented by two spaces per level.

```
$ trilium ls <workspace> -d 2
<project>	book	Project Apollo	+3
  <id1>	text	Budget review
  <id2>	text	Launch checklist
  <id3>	text	Supplier call
$ trilium mv <id2> <archive>
moved <id2> "Launch checklist": <project> → <archive>
$ trilium branch add <id2> <project>
cloned <id2> into <project> (branch <project>_<id2>)
```

**7. Images.** Upload, then put the printed `embed:` HTML into the note.

```
$ trilium att add <id1> -f chart.png
added attachment <att-id> to <id1>: "chart.png" image/png 69 bytes
embed: <img src="api/attachments/<att-id>/image/chart.png">
$ trilium append <id1> '<p><img src="api/attachments/<att-id>/image/chart.png"></p>'
```

**8. Markdown to rich text.**
```
$ trilium import <project> plan.md
imported <new-id> "plan" into <project>
```

**9. Working alongside other agents.** With `TRILIUM_AGENT=<me>` set, notes
you create get `#agent=<me>`. Find yours with `trilium search '#agent=<me>'`;
see recent changes with `trilium history --in <project>`.

**10. Read cheaply.** For reading, summarizing or answering questions use
`trilium get <id…> --text`: a plain-text view (marked `content: plain-text
view …`). Use `get -c` when you need the real HTML, e.g. for `edit`.

**11. Act on every child of a folder.** `attr`, `get` etc. take note IDs; loop
over the first column of `ls` (depth 1, so no indentation):

```bash
for id in $(trilium ls <folder> | cut -f1); do trilium attr "$id" '#status=triaged'; done
```

## Gotchas

- `search words` is full-text: it matches titles **and** content (all words
  must appear). `--fast` searches titles and attributes only.
- **Quote search values containing `.` or `-`**, or nothing matches:
  `note.title="hello.py"`, `#date="2026-10-25"`. Quote the whole query for the
  shell too: `trilium search 'note.title="hello.py"'`.
- **`rm` deletes the note in every place it appears, with its whole subtree.**
  To remove one placement of a clone use `trilium branch rm <note> <parent>`.
  Mistakes can be undone with `trilium undelete <id>` (see `trilium history`).
- **Never use `--text` output as `edit --old`**: it is not the stored HTML.
- **Calendar shortcuts create notes** (`@today` creates today's note and, the
  first time, a "Calendar" root). Use them when you mean to write there.
- **Binary notes (images, files)**: don't `get -c`; save with
  `trilium get <id> -o file`. Attachments: `trilium att get <id> -o file`.
- `edit --old` must match exactly once; add context or use `--all`. Nothing is
  written unless every pair matches.
- Attribute names may only contain letters, digits, `_` and `:`.
- Exit codes: 0 ok (also "no matches"), 1 failed, 2 bad usage/config, 3 not found.

## References

- [references/commands.md](references/commands.md): every command, flag and
  output format. Read when a command or flag isn't covered above.
- [references/search.md](references/search.md): search syntax (labels,
  relations, note properties, tree conditions, and/or/not, regex, sorting).
  Read before writing any non-trivial query.
- [references/html.md](references/html.md): HTML for text notes (headings,
  lists, to-dos, code blocks, tables, images, links). Read before writing
  rich text.
- [references/workflows.md](references/workflows.md): longer recipes
  (reorganizing, batch tagging, journals, safe rewrites, export/import,
  recovering deleted notes).

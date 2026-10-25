# trilium command reference

## Contents

- Conventions: note references, content input, output, exit codes
- Everyday: search, ls, get, create, append, edit, put, rm
- Organize: update, attr, mv, branch, att, rev
- More: history, export, import, undelete, info, backup, version

## Conventions

**Note references** (`<note>`): a note ID (`abc123XYZ`), `root`, or a shortcut:
`@inbox`, `@today`, `@yesterday`, `@tomorrow`, `@2026-10-25` (day), `@2026-W43`
(week, needs `#enableWeekNote` on the calendar root), `@2026-10` (month),
`@2026` (year). Calendar shortcuts **create the note if it doesn't exist**
(and a "Calendar" root note on first use). `@inbox` is the note labelled
`#inbox`, else today's day note, else root. Dates use the local time zone.

**Content input** (create, append, put, att add, att put) — exactly one of:
a positional argument (short text), `-f FILE` (long text, all binary data),
`-f -` (stdin). stdin is never read without `-f -`. Bytes are sent unchanged.

**Output**: list rows are tab-separated with the ID first; write commands print
one result line; errors go to stderr as `error: …` plus an optional `hint: …`.
Empty results print `no matches` to stderr with exit code 0. Tabs, newlines
and other control characters in titles, prefixes and attribute values are
printed escaped (`\t`, `\n`, `\xNN`), so every row is one line; the stored
data is unchanged.
`--json` prints JSON Lines with ETAPI field names: entities as stored after
the change, content changes as `{"noteId","bytesBefore","bytesAfter"}`, saved
files (`-o FILE`) as `{"…Id","file","bytes"}`. Exceptions: `-o -` and `att get`
without `-o` print raw bytes. Errors become
`{"error":{"status","code","message"},"hint"}` on stderr.

**Errors after a change**: an error does not always mean nothing happened.
Messages say so when it matters: `… the result is unknown` / `your change may
be missing` (check with `get -c`), `the N operation(s) printed above were
applied` (`attr`), `created <id> but …` (two-step uploads), `… the note now
has both` (relation replacement).

**Exit codes**: 0 success (also empty results) · 1 API/network error or failed
operation (e.g. `edit` found no match) · 2 usage or configuration error ·
3 not found (`*_NOT_FOUND`).

**System notes** (`root` and IDs starting with `_`):

| Command | Refused | Needs `--force` |
|---|---|---|
| `rm`, `mv` (the moved note), `branch rm` | always | — |
| `put`, `append`, `edit`, `update`, `attr` | — | note is `_…` |
| `create` | — | `-p` or `--id` is `_…` |
| `mv` | — | new parent is `_…` |
| `branch add`, `branch set` | cloning `root` | note or parent is `_…` |
| `att add/put/update/rm` | — | owner note is `_…` |
| `import` | — | parent is `_…` |

Reading is never restricted.

## Everyday

### search `<query...>`
`-n N` (default 20, 0 = all) · `--in <note>` · `--depth eqN|ltN|gtN` ·
`--sort title|dateModified|dateCreated|#label` · `--desc` · `--fast` (skip
content) · `--archived` · `-l`. Syntax: `trilium help search` or search.md.
```
<id>\t<type>\t<title>[\t+<children>][\t(archived)]
-l: <id>\t<type>\t<mime>\t<modified>\t<title>[\t+N][\t(archived)]
stderr when cut off: 20 shown; more available (use -n)
```
Notes with IDs starting with `_` are hidden. With `--in`, the ancestor itself
can be part of the results (Trilium's subtree search includes it).

### ls `[note=root]`
`-d N` levels (indented two spaces per level) · `-n N` max lines (default 100) ·
`-l` · `--all` (show `_` system notes). Tree order, archived notes included and
marked. Same row format as search.

### get `<note...>`
`-c` content only (exact bytes) · `-m` metadata only · `--text` read-only
plain-text view (text notes) · `--all-attrs` add inherited attributes ·
`-o FILE` save raw content (one note; `-o -` = stdout).
```
---
id: tA1b2C3d
title: Meeting notes
type: text (text/html)
parents: e2GMUhHk0e9v
attrs: #status=draft ~seeAlso=xYz12345
inherited: #project=apollo (from e2GMUhHk0e9v)     (only with --all-attrs)
children: 0
attachments: 1
created: 2026-10-25 16:55
modified: 2026-10-25 16:57
content: plain-text view (read-only; not the stored HTML)   (only with --text)
---
<p>raw content…</p>
```
Several notes print several blocks, separated by an empty line. Binary notes
show `content: binary (image/png), save with: trilium get <id> -o <file>` and
`-c` fails. Protected notes show `protected: yes` and no content.

### create `<title> [content]`
`-p <note>` (default `@inbox`) · `-t TYPE` (default text; also code, mermaid,
book, search, file, image, canvas, …) · `--mime M` · `-f FILE` · `-a ATTR`
(repeatable) · `--id ID` (refused when that ID exists, so retries never
overwrite) · `--pos N` · `--force` (parent or `--id` is a system note).
With `-f`, type/mime/title come from the file (`.html` → text, `.md`/`.py`/…
→ code, `.mmd` → mermaid, images → image, else file); flags win. `-t code`
without `--mime` uses the title's extension. `-t search` stores the content as
`#searchString`. `TRILIUM_AGENT=<name>` adds `#agent=<name>`.
```
created tA1b2C3d text "Meeting notes" in e2GMUhHk0e9v (@inbox)
```

### append `<note> [content]`
`-f FILE` · `--top` (prepend) · `--force`. Text (HTML) notes are joined byte
for byte. Other notes get a `\n` between old and new content when needed.
Detects most concurrent changes and redoes the append; ETAPI has no
conditional write, so a lost update is still possible (one writer per note).
```
appended tA1b2C3d +36 bytes: 2431 → 2467 bytes
error: tA1b2C3d kept changing while writing (another writer?); your change may be missing
```

### edit `<note> --old S --new S [--old S --new S]...`
`--old-file F --new-file F` · `--all` · `--force`. Exact byte match on the raw
content (HTML for text notes). All pairs succeed or nothing is written. Each
`--old` must match once unless `--all`. `--new ""` deletes.
```
edited tA1b2C3d 2 replacements: 2431 → 2467 bytes
error: --old #1 not found in tA1b2C3d (nothing was changed)        (exit 1)
error: --old #1 matches 3 times in tA1b2C3d (nothing was changed)  (exit 1)
error: tA1b2C3d was written, but another writer changed it right after; the result is unknown
```
Pairs apply in order, so `--old A --new B --old B --new C` turns A into C.

### put `<note> [content]`
`-f FILE` · `--force`. Replaces the whole content; no automatic revision (run
`rev add` first). Image/file notes take new binary data with `-f`.
```
updated tA1b2C3d content: 2431 → 2467 bytes      (? → N for binary notes)
```

### rm `<note...>`
`-r` needed when a note has children. Deletes the note **everywhere it appears
and the notes below it** (a descendant cloned elsewhere survives there).
Restorable with `undelete`.
```
deleted tA1b2C3d "Title" (+3 children; undo: trilium undelete tA1b2C3d)
deleted tA1b2C3d "Title" (undo: …) (was in 2 places; to remove one placement use: trilium branch rm)
```

## Organize

### update `<note>`
`--title T` · `--type TYPE` · `--mime M` · `--force`. Sends only given fields.
```
updated tA1b2C3d title: "Old" → "New", mime: text/plain → text/x-python
```

### attr `<note> [ops...]`
No ops: list own attributes (`--all` adds inherited). `--force` for `_` notes.
```
<attributeId>\t#status=done
<attributeId>\t#project=apollo\t(from e2GMUhHk0e9v)      (inherited, with --all)
```
Ops (quote them): `#name=value` set (changes the first existing one), `#name`
set with empty value, `+#name=value` add another, `-#name` remove all,
`-#name=value` remove that one, `~name=<note>` set relation (`+~`, `-~` too),
`#name(inheritable)=value`. Values with spaces: `'#title="a b"'` (escapes
`\"` `\\` `\n` `\t` `\r`). Names: letters, digits, `_`, `:` only. All ops are
parsed, and relation targets checked to exist, before any change; ops then
run in order, so a later failure leaves earlier ones applied (the error says
how many). Replacing a relation or changing inheritability creates the new
attribute before deleting the old one. Output per op: `set #status=done on
<id>`, `added …`, `removed …`.

### mv `<note> <new-parent>`
`--from <old-parent>` (required for cloned notes) · `--pos N`. Creates the new
placement first, then removes the old one. Refuses to move a note into itself
or its own subtree.
```
moved tA1b2C3d "Title": e2GMUhHk0e9v → xYz12345
```

### branch
A branch is one placement of a note; cloning adds another. Address a branch as
`<note> <parent>` or by branch ID (usually `<parentId>_<noteId>`).
```
branch ls <note>                       <branchId>\t<parentId>\t<position>[\t<prefix>]
branch add <note> <parent> [--prefix P] [--pos N]
   cloned tA1b2C3d into xYz12345 (branch xYz12345_tA1b2C3d)
branch set <note> <parent> [--prefix P] [--pos N] [--expanded=true|false]
   updated branch xYz12345_tA1b2C3d: position 5
branch rm <note> <parent> [--force]
   removed tA1b2C3d from xYz12345 (branch xYz12345_tA1b2C3d)
```
`branch add` refuses loops. `branch rm` refuses the last placement (that would
delete the note) unless `--force`.

### att (attachments)
```
att ls <note>                  <attachmentId>\t<role>\t<mime>\t<size>\t<title>
att get <attachmentId> [-o FILE]           text is printed; binary needs -o
att add <note> -f FILE | content [--title T] [--role file|image] [--mime M] [--pos N]
   added attachment Kq9xYz1a to tA1b2C3d: "chart.png" image/png 80 bytes
   embed: <img src="api/attachments/Kq9xYz1a/image/chart.png">     (images only)
att put <attachmentId> -f FILE | content
att update <attachmentId> [--title T] [--role R] [--mime M] [--pos N]
att rm <attachmentId>
```
`size` is Trilium's contentLength: characters for text, bytes for binary.
Text content given as an argument needs `--title`.

### rev (revisions)
```
rev ls <note>                  <revisionId>\t<created>\t<size>\t<title>   (newest first)
rev get <revisionId> [--text] [-o FILE]       same layout as get
rev add <note>                 created revision R1a2b3c4 of tA1b2C3d
```

## More

### history `[--in <note>] [-n 20]`
```
<date>\t<noteId>\t<title>[\tdeleted][\t(can undelete)]
```

### export `<note> [-o FILE] [--format html|markdown]`
Zip of the note and its subtree; default file `./<title>.zip`.
`exported tA1b2C3d to ./Title.zip (8.9 KB)`

### import `<parent> <file.zip | file.md>`
A `.md` file becomes a rich text note, converted by Trilium's own importer.
`imported tNew1234 "Title" into xYz12345`

### undelete `<noteId>`
`restored tA1b2C3d "Title"` — find candidates with `history`.

### info
Checks the configuration; run it first.
```
server:  https://notes.example.com (Trilium 0.106.1)
auth:    ok
notes:   400 active, 40 deleted
agent:   claude-1
```

### backup `[name=now]` · version
`backup created on server: backup-now.db` · `trilium 1.0.0 (commit abc1234)`

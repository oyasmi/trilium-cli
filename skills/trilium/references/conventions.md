# trilium conventions and output formats

Flags and examples for each command: `trilium help <command>`. This file covers
what `--help` does not: shared conventions, what each command prints, and
behavior that is easy to get wrong.

## Contents

- Conventions: references, content input, output, errors, exit codes, system notes
- Reading: search, ls, get
- Writing: create, append, edit, put, rm
- Organizing: attr, mv, branch, att, rev
- Other: history, export, import, undelete, info

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

## Reading

**search / ls** rows: `<id>\t<type>\t<title>[\t+<children>][\t(archived)]`; with
`-l`: `<id>\t<type>\t<mime>\t<modified>\t<title>[\t+N][\t(archived)]`. `ls`
indents deeper levels by two spaces and stops at 100 lines (`-n 0`: all);
`search` stops at 20. A cut-off prints `20 shown; more available (use -n)` on
stderr. Notes with IDs starting `_` are hidden (`ls --all` shows them). With
`search --in`, the ancestor itself can appear in the results.

**get** prints a header, then the raw content (HTML for text notes):
```
---
id: tA1b2C3d
title: Meeting notes
type: text (text/html)
parents: e2GMUhHk0e9v
attrs: #status=draft ~seeAlso=xYz12345
children: 0
attachments: 1
created: 2026-10-25 16:55
modified: 2026-10-25 16:57
---
<p>raw content…</p>
```
`-c` prints only the exact stored bytes, `-m` only the header, `--text` a
read-only plain-text view (`content: plain-text view …` in the header),
`--all-attrs` adds inherited attributes. Several notes print several blocks.
Binary notes show `content: binary (image/png), save with: trilium get <id> -o
<file>` and `-c` fails. Protected notes show `protected: yes` and no content.

## Writing

```
created tA1b2C3d text "Meeting notes" in e2GMUhHk0e9v (@inbox)
appended tA1b2C3d +36 bytes: 2431 → 2467 bytes
edited tA1b2C3d 2 replacements: 2431 → 2467 bytes
updated tA1b2C3d content: 2431 → 2467 bytes
deleted tA1b2C3d "Title" (+3 children; undo: trilium undelete tA1b2C3d)
```

- **create**: with `-f`, type, mime and title come from the file (`.html` →
  text, `.md`/`.py`/… → code, `.mmd` → mermaid, images → image, else file);
  explicit flags win. `-t search` stores the content as `#searchString`.
  `--id` is refused when that ID exists, so a retry never overwrites.
  `TRILIUM_AGENT=<name>` adds `#agent=<name>`.
- **append**: text (HTML) notes are joined byte for byte, other notes get a
  `\n` between old and new when needed. It detects most concurrent changes and
  redoes the append; a lost update is still possible (one writer per note).
- **edit**: exact byte match on the raw content. All pairs succeed or nothing
  is written. Pairs apply in order, so `--old A --new B --old B --new C` turns
  A into C. `--new ""` deletes.
  ```
  error: --old #1 not found in tA1b2C3d (nothing was changed)        (exit 1)
  error: --old #1 matches 3 times in tA1b2C3d (nothing was changed)  (exit 1)
  error: tA1b2C3d was written, but another writer changed it right after; the result is unknown
  ```
- **put**: replaces everything, with no automatic revision (`rev add` first).
- **rm**: needs `-r` when the note has children. Deletes the note everywhere it
  appears, with its subtree (a descendant cloned elsewhere survives there).

## Organizing

**attr** `<note> [ops...]`: no ops lists own attributes (`--all`: inherited).
Ops, quoted for the shell:

| Op | Effect |
|---|---|
| `#name=value` | set (changes the first existing one, or adds) |
| `#name` | set with empty value |
| `+#name=value` | add another label of the same name |
| `-#name` / `-#name=value` | remove all / just that one |
| `~name=<note>` | set a relation (`+~`, `-~` too) |
| `#name(inheritable)=value` | children inherit it |

Values with spaces: `'#title="a b"'` (escapes `\"` `\\` `\n` `\t` `\r`). Names use
letters, digits, `_`, `:`. All ops are parsed, and relation targets checked,
before any change; ops then run in order, so a later failure leaves earlier
ones applied (the error says how many). Output per op: `set #status=done on
<id>`, `added …`, `removed …`.

**mv** `<note> <new-parent>`: `--from <old-parent>` is required for cloned
notes. Adds the new placement first, then removes the old one; refuses to move
a note into itself or its subtree.

**branch** (`ls`, `add`, `set`, `rm`): a branch is one placement of a note;
address it as `<note> <parent>` or by branch ID (usually `<parentId>_<noteId>`).
`add` refuses loops; `rm` refuses the last placement unless `--force`.

**att** (`ls`, `get`, `add`, `put`, `update`, `rm`): `att add` prints
`added attachment Kq9xYz1a to tA1b2C3d: "chart.png" image/png 80 bytes` and, for
images, `embed: <img src="api/attachments/Kq9xYz1a/image/chart.png">`. `att get`
prints text; binary needs `-o`. Text given as an argument needs `--title`. `ls`/`rev ls` sizes are Trilium's
`contentLength`: characters for text, bytes for binary.

**rev** (`ls`, `get`, `add`): revisions, newest first.

## Other

`history` rows: `<date>\t<noteId>\t<title>[\tdeleted][\t(can undelete)]`.
`export <note> [-o FILE] [--format html|markdown]` writes a zip of the subtree.
`import <parent> <file.zip|file.md>`: a `.md` becomes a rich text note, converted
by Trilium. `undelete <noteId>` restores a note and the subtree deleted with it.

`info` checks the configuration:
```
server:  https://notes.example.com (Trilium 0.106.1)
auth:    ok
notes:   400 active, 40 deleted
inbox:   tA1b2C3d "Inbox"
agent:   claude-1
```
`inbox:   none (…)` means no note has the label `#inbox`, so `@inbox` falls
back to today's day note.

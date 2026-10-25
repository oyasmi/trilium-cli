# Trilium workflows

Longer end-to-end recipes. `<id>` placeholders stand for real note IDs taken
from earlier output.

## Contents

- Reorganize a part of the tree
- Add an image to a note
- Tag a batch of notes
- Keep a work log in the journal
- Rewrite a section of a long note safely
- Export, import and Markdown
- Recover a deleted note
- Several agents on one knowledge base

## Reorganize a part of the tree

```bash
trilium ls <project> -d 2              # see the current structure
trilium create "Archive" -p <project> -t book
trilium mv <old-note> <archive>        # move (new placement first, then old removed)
trilium branch add <note> <other>      # also show a note in a second place (clone)
trilium branch set <note> <parent> --pos 5   # reorder among siblings
trilium ls <project> -d 2              # check the result
```

- A cloned note has several parents; `mv` then needs `--from <parent>`.
- `branch rm <note> <parent>` removes one placement; `rm` deletes the note
  everywhere, with its subtree.
- `book` notes are folders: they show their children instead of content.

## Add an image to a note

```bash
trilium att add <note> -f chart.png
# added attachment Kq9xYz1a to <note>: "chart.png" image/png 4211 bytes
# embed: <img src="api/attachments/Kq9xYz1a/image/chart.png">
trilium append <note> '<p><img src="api/attachments/Kq9xYz1a/image/chart.png"></p>'
```

To place it in the middle of the note, use `edit` with an `--old` anchor and a
`--new` that contains the anchor plus the `<img>`.

## Tag a batch of notes

```bash
trilium search 'note.title *=* invoice' --in <finance> -n 0
ids=$(trilium search 'note.title *=* invoice' --in <finance> -n 0 | cut -f1) || exit 1
for id in $ids; do
  trilium attr "$id" '#status=paid' '-#todo' || echo "failed: $id"
done
trilium search '#status=paid' --in <finance> -n 0   # verify
```

For the direct children of a folder use `trilium ls <folder> -n 0 | cut -f1`
instead (`ls` stops at 100 lines without `-n 0`; don't use `-d N` here: deeper
levels are indented). Titles with tabs or newlines are printed escaped
(`\t`, `\n`), so every row stays one line. For scripts, `--json` is the most
robust input.

`#name=value` changes an existing label or adds it, so repeating the loop is
safe. Use `+#tag=value` only when you really want a second label of that name.

## Keep a work log in the journal

```bash
trilium append @today '<p>10:30 Called the supplier; delivery moved to Friday.</p>'
trilium get @today --text            # read back cheaply
trilium ls @2026-10                  # day notes of a month
```

Calendar shortcuts create missing day/month/year notes (and the Calendar root
on first use). Day boundaries follow the local time zone.

## Rewrite a section of a long note safely

```bash
trilium get <id> -c > note.html                 # raw HTML, to find exact text
# write the exact old fragment and the new fragment to files
trilium edit <id> --old-file old.html --new-file new.html
trilium rev ls <id>                             # older versions, if any
```

- Never copy text for `--old` from `get --text`: it is not the stored HTML.
- Before a large rewrite with `put`, save a snapshot: `trilium rev add <id>`.
- Several small `--old/--new` pairs in one `edit` are applied all-or-nothing.

## Export, import and Markdown

```bash
trilium export <project>                         # ./Project.zip (HTML)
trilium export <project> --format markdown -o project-md.zip
trilium import <new-parent> ./Project.zip        # restore or copy a subtree
trilium import <parent> report.md                # Markdown → rich text note
```

`import x.md` is the way to turn Markdown into Trilium rich text; Trilium's
importer does the conversion. To keep Markdown as Markdown, create a code note:
`trilium create report.md -f report.md -p <parent>`.

## Recover a deleted note

```bash
trilium history -n 50            # deleted notes show "deleted (can undelete)"
trilium undelete <noteId>        # restores the note and the subtree deleted with it
```

## Several agents on one knowledge base

- Set `TRILIUM_AGENT=<name>` for each agent; every `create` adds `#agent=<name>`.
- Find your own notes: `trilium search '#agent="<name>"' --sort dateModified --desc`.
- See what changed recently: `trilium history --in <project>`.
- Don't let several agents write the same note. `append` and `edit` detect
  most concurrent changes (append retries, edit reports an unknown outcome),
  but Trilium has no conditional write, so a lost update is still possible.
  For a shared log, give each entry its own note (`create … -p <log-folder>`)
  and read them together with `ls` or `search`.
- `create --id <stable-id>` refuses an ID that already exists, so a retried
  create doesn't duplicate or overwrite. (Two clients racing for the same new
  ID at the same instant are not protected.)

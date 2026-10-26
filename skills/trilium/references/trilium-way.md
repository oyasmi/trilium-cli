# Using Trilium the way it was designed

Trilium is a **hierarchical, database-backed note app** with a rich-text editor,
a typed attribute system and a calendar. It is not a folder of Markdown files.
Notes written to fit these ideas are easier for the user to find and maintain
than notes that treat Trilium as a flat file store.

Sources are Trilium's User Guide (docs.triliumnotes.org); statements marked
*observed* were checked against a live server (Trilium 0.106.1) through ETAPI.

## Contents

- Strengths and weaknesses
- The tree, clones and archiving
- Choosing a note type
- Attributes: Trilium's metadata system
- Inbox, journal and the calendar
- Templates, collections and saved searches
- Ordering, revisions, protected notes
- Habits that make an agent's notes good

## Strengths and weaknesses

| Lean on | Be careful with |
|---|---|
| Deep tree plus **clones**: one note in several places | Not plain files: no grep, no git diff; back up with `trilium export` |
| **Attributes** + search language: structured, queryable metadata | Text notes are HTML: Markdown round-trips change structure |
| Day notes (journal), templates, saved searches, collections | Links are tree/relation based, not block-level or graph-first |
| Revisions, archiving, fast with very large note counts | Protected (encrypted) notes can't be read or written through ETAPI |
| Scripting, web clipper, ETAPI | Made for one user; no per-note permissions |
| Rich notes: tables, code, math, diagrams, images | Writes via ETAPI skip some editor behavior (see Links) |

## The tree, clones and archiving

- Organize by **placing a note under the topic it belongs to**, not by flat
  lists or title prefixes. A branch is one placement; keep the tree readable
  (a handful of top-level notes, topics below).
- A **clone** is the same note in a second place (`trilium branch add <note>
  <parent>`), edits show everywhere. Use it when a note truly belongs to two
  topics; never copy-paste a duplicate.
- **Archive instead of delete** for things that are finished but may matter:
  `trilium attr <note> '#archived'`. Archived notes drop out of normal search
  (`search --archived` finds them) and autocomplete, and stay recoverable with
  their links. `rm` is for mistakes and junk.

## Choosing a note type

**Default to a text note.** It is Trilium's main type and the one its features
are built around: a rich editor, tables, callouts, images, reference links,
automatic table of contents. The other types are for specific jobs.

| Content | Type | Create with |
|---|---|---|
| Knowledge, minutes, journal entries, anything a person reads and edits in the app | **text** (HTML) | `trilium create "T" -f body.html` |
| Markdown the user wants kept as source (README drafts, docs shared with other tools) | **Markdown** | `trilium create doc.md -f doc.md` |
| Scripts, logs, JSON, config, plain text | **code** | `trilium create x.py -f x.py` / `-t code --mime …` |
| A diagram described in text (flowchart, sequence, state, class) | **mermaid** | `trilium create "Flow" -t mermaid -f flow.mmd` |
| A folder-like container, task table, board | **book** (collection) | `trilium create "Tasks" -t book` |
| A live list from a query | **search** | `trilium create "Open" -t search '#status=todo' -p <where>` |
| Uploaded files and images | **file**, **image** | `trilium create x.pdf -f x.pdf` |

Notes on the types:

- **Text** is stored as HTML. Structure it with real headings (`<h2>`, `<h3>`):
  the app builds an outline / table of contents from them. Keep one topic per
  note and the note moderately sized: very large text and code notes open
  read-only in the app (threshold set in Options; `#autoReadOnlyDisabled` exempts
  a note, `#readOnly` locks one). Use simple tables for small comparisons; for
  many items with several fields, use child notes in a collection instead.
- **Markdown** is a *code note with a Markdown mime* (`text/markdown`,
  `text/x-markdown`, `text/x-gfm`), shown as source plus live preview, kept byte
  for byte (*observed*: `-t code --mime text/markdown` works). It lacks the
  editor's reference links and image handling. `trilium import <parent> doc.md`
  does the other thing: it **converts** Markdown into a text note.
- **Code** notes get syntax highlighting from their mime; `#tabWidth`,
  `#indentWithTabs`, `#wrapLines` tune them.
- **Mermaid** notes hold plain Mermaid source (*observed*: mime `text/vnd.mermaid`)
  and render as a diagram; prefer one over ASCII art or an image. A diagram
  can't be embedded live inside a text note by CLI; link to the mermaid note.
- **Canvas** (Excalidraw), **mind map**, **relation map**, **geo map** keep
  their data in app-specific formats: create or change them in the app, not by
  hand.
- A **saved search** is stored where you put it. In the app it defaults to the
  day note; with the CLI you choose the parent, or set `#searchHome` on a note
  to make it the usual place.
- The app can convert between text and Markdown notes (to-do states carry over),
  so the choice isn't permanent.

## Attributes: Trilium's metadata system

Attributes are the intended way to add structure; use them instead of encoding
meaning in titles ("[DONE] …") or in body text.

- **Labels** `#name=value` (searchable metadata or behavior) and **relations**
  `~name=<noteId>` (typed links between notes, e.g. `~seeAlso`). A note can hold
  several attributes with the same name.
- **Inheritable** attributes (`#name(inheritable)=…`) apply to all descendants,
  which makes them good for "everything under this folder". `#child:name` copies
  an attribute onto *new* direct children only.
- **Built-in names are interpreted.** An attribute named like a system one
  changes behavior. Choose your own names (`status`, `project`, `due`) and never
  reuse these for other meanings:

  | Purpose | Names |
  |---|---|
  | Placement | `inbox`, `clipperInbox`, `calendarRoot`, `searchHome`, `workspace` |
  | Visibility, order | `archived`, `sorted`, `sortDirection`, `sortFoldersFirst`, `top`, `bottom`, `subtreeHidden`, `bookmarked` |
  | Look | `iconClass`, `color`, `cssClass` |
  | Structure | `template`, `titleTemplate`, `viewType`, `pageSize` |
  | Safety | `readOnly`, `disableVersioning`, `versioningLimit`, `excludeFromExport` |
  | Calendar | `datePattern`, `monthPattern`, `yearPattern`, `weekPattern`, `enableWeekNote` |

- **Promoted attributes** show an attribute as a form field at the top of a note
  (and as a column in table collections). They are defined by a
  `#label:name="promoted,alias=Status,…"` (or `relation:`) attribute on a template
  or parent, e.g. `#label:status(inheritable)="promoted,alias=Status,single,text"`.
  Set the values with ordinary labels: `trilium attr <note> '#status=todo'`.
- Reuse the **same attribute names and values** across notes (`#status=todo`,
  `#status=done`) so searches and collections work; look at existing notes
  (`trilium attr <sibling>`) before inventing a new name.

## Inbox, journal and the calendar

- **Inbox** is the note labelled `#inbox`: quick capture lands there
  (`trilium create "…"` with no `-p`), and the user files it later. After
  capturing, leave the note in the inbox; move it (`mv`) to its topic only when
  asked or when you were told where it goes. The web clipper uses
  `#clipperInbox`, else `#inbox`.
- **Journal** is built from day notes under the note labelled `#calendarRoot`:
  Year › Month › Day (*observed*: `2026` › `10 - 十月` › `26 - 周一`; titles follow
  the root's `#datePattern`, `#monthPattern`, `#yearPattern`). Each day note has
  `#dateNote=YYYY-MM-DD`. `@today` and `@2026-10-25` find or create them.
- Put **time-bound things** in the day note: what happened today, meeting
  minutes, daily logs (`trilium append @today …`). Put **lasting knowledge** in
  topic notes. To see a topic's note on a given day, clone it into the day note
  (`branch add`).
- Week and quarter notes exist only if `#enableWeekNote` / `#enableQuarterNote`
  is set on the calendar root. Don't create your own "2026-10-26" notes: the day
  note already is that.
- The root is usually sorted (`#sorted`), so don't set positions by hand there.

## Templates, collections and saved searches

- **Template**: a note labelled `#template`. Notes get its attributes through a
  `~template` relation; its content is copied only into an empty note, and its
  children are copied too. `~child:template` on a parent applies a template to
  new children; `#titleTemplate` sets default titles. Use a template when many
  notes share a shape (people, books, projects). Whether ETAPI-created notes
  receive the copied content and children was **not** verified; check with
  `trilium get`/`ls` after creating from a template.
- **Collection** (`-t book`): shows its children as a grid, list, calendar,
  table or kanban board; table columns are the children's promoted attributes,
  boards group by a label (e.g. `#status`). View settings are stored by the
  app, so pick the view in the UI. Use collections for task lists, reading lists
  and anything with several fields per item: one child note per item, attributes
  for its fields.
- **Saved search** (`trilium create "Open tasks" -t search '#status=todo'`):
  a note that always shows the current result of a query. Prefer it to keeping
  hand-made lists in sync.

## Ordering, revisions, protected notes

- Children keep **manual order** unless the parent has `#sorted` (alone:
  alphabetical; or `title`, `dateCreated`, `dateModified`, or a child label name;
  `#sortDirection=DESC`). Sorting compares as strings ("10" before "2"). Pin
  with `#top` / `#bottom`.
- **Revisions** are snapshots taken on an interval (and on ETAPI changes).
  Take one before a big rewrite: `trilium rev add <note>`. Notes that change
  constantly (logs) can set `#disableVersioning` or `#versioningLimit`.
- **Protected notes** are encrypted (title, content, attachments); structure
  and attributes stay visible. The CLI can't read or change them: leave them
  alone and tell the user.

## Habits that make an agent's notes good

1. **Search before you create** (`trilium search`) to extend an existing note
   instead of duplicating it.
2. **One topic per note**, a descriptive title, placed under its topic. Don't
   dump long mixed content into one note, or hundreds of notes into one parent.
3. **State goes in labels, not titles**: `#status=done`, not "DONE - …". Dates
   of creation and modification are tracked by Trilium; don't put them in titles.
4. **Connect related notes** with a `~relation` (`~seeAlso=<id>`) or a clone.
   *Observed*: HTML links written through ETAPI (`<a href="#root/<id>">`) work
   for clicking but did **not** create the backlink relations the editor
   creates, so the target shows no backlink.
5. **Give containers an icon** and keep attribute names consistent
   ([icons.md](icons.md)).
6. **Time-bound goes to the journal, lasting knowledge to a topic.**
7. **Ask before restructuring** a tree the user built, and prefer additive
   changes: add, clone, archive. Don't mass-edit attributes or move subtrees
   unasked.

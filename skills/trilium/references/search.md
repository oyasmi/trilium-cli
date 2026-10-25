# Trilium search syntax

Used by `trilium search <query>` and by saved searches (`create -t search`).
Quote the whole query for your shell: `trilium search '#status=todo'`.

## Quoting rule (most common mistake)

Values containing `.` or `-` (or spaces) must be quoted, or the search
silently finds nothing:

    note.title = "hello.py"        not  note.title = hello.py
    #date = "2026-10-25"           not  #date = 2026-10-25

## Full text

    rings tolkien                  notes containing both words (title or content)
    "Lord of the Rings" tolkien    exact phrase plus a word
    towers --fast                  (CLI flag) titles and attributes only, faster

## Labels and relations

    #book                          has the label
    #!book                         does not have the label
    #year = 1954                   label value equals
    #year >= 1950 #year < 1960     numeric comparison
    #genre *=* fan                 contains        (=* starts with, *= ends with)
    #status != done                not equal
    #code %= '^[A-Z]{3}$'          regular expression
    #due <= TODAY+7                dates: NOW±seconds, TODAY±days, MONTH±months, YEAR±years
    ~author                        has the relation
    ~author.title *=* Tolkien      relation target's title contains

## Note properties (note.*)

    note.title *=* report          title contains
    note.type = code               types: text, code, file, image, search, book, mermaid …
    note.mime = "text/x-python"
    note.content *=* budget        content contains
    note.dateModified >= "2026-10-01"
    note.childrenCount > 10
    note.noteId = "abc123XYZ"
    Others: dateCreated, isArchived, isProtected, labelCount, relationCount,
    parentCount, contentSize, revisionCount

## Tree position

    note.parents.title = "Books"   a parent is titled "Books"
    note.ancestors.title = "Work"  somewhere below a note titled "Work"
    note.children.title = "Todo"   has a child titled "Todo"
    --in <note> [--depth eq1]      (CLI flags) only below a note; eq1 = direct children

## Combining

    #book or #article              either
    #article #!read                AND is the default between terms
    #article and not(#status = done)
    #year (#book or #article)      parentheses group; put the group last

## Ordering and limits

Prefer the CLI flags: `--sort dateModified --desc -n 10`.
The query language also has them, after an expression:
`#book orderBy #year desc, note.title limit 10`.

## Examples

    trilium search '#status=todo' --in <project>
    trilium search 'note.title *=* "q3-report"'
    trilium search '#agent="claude-1"' --sort dateModified --desc -n 5
    trilium search 'note.type = code note.mime = "text/x-python"'

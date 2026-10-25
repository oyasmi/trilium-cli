# Note icons

A note's icon is the inheritable-or-own label `#iconClass`, whose value is a
[Boxicons](https://boxicons.com/) v2 class string: the style prefix plus the
icon name, both parts required.

```bash
trilium attr <note> '#iconClass=bx bx-folder'     # set or change (same command)
trilium attr <note> '-#iconClass'                 # remove: back to the type's default icon
trilium create "Inbox" -p root -a '#iconClass=bx bx-archive-in'   # set at creation
trilium attr <folder> '#iconClass(inheritable)=bx bx-file'        # default for all descendants
```

Own labels beat inherited ones, so an inheritable icon on a folder can still be
overridden on single notes.

## A wrong name shows as "?"

The CLI does not validate the value: an icon name that does not exist is stored
fine and then renders as a question mark in the tree. `bx bx-inbox`, for
example, does **not** exist (use `bx bx-archive-in`). So:

- Pick names from the list below; don't invent them.
- Need something else? Ask the user to set the icon in Trilium (right-click the
  note → *Change note icon*), then read the value back with `trilium attr <note>`
  and reuse it.
- If the user reports a "?", replace the value with a name from this list.

## Prefixes

| Prefix | Style | Example |
|---|---|---|
| `bx bx-` | regular outline (default choice) | `bx bx-folder` |
| `bx bxs-` | solid | `bx bxs-folder` |
| `bx bxl-` | brand logos | `bx bxl-github`, `bx bxl-python` |

Not every icon exists in every style; when in doubt use `bx bx-`.

## Known-good names (`bx bx-…`)

| Use for | Names |
|---|---|
| Folders, collections | `folder`, `folder-open`, `collection`, `archive`, `archive-in`, `package`, `layer` |
| Documents, notes | `file`, `file-blank`, `note`, `notepad`, `book`, `book-open`, `bookmark`, `news` |
| Tasks, status | `task`, `list-check`, `check-circle`, `flag`, `star`, `pin`, `label`, `purchase-tag` |
| Time, planning | `calendar`, `calendar-check`, `calendar-event`, `time` |
| Capture, mail | `envelope`, `mail-send`, `edit`, `bulb` |
| Code, tech | `code`, `code-alt`, `terminal`, `bug`, `git-branch`, `server`, `data`, `cog`, `wrench` |
| People, work | `user`, `group`, `chat`, `briefcase`, `home` |
| Money, shopping | `wallet`, `receipt`, `cart` |
| Media | `image`, `camera`, `music`, `movie` |
| Misc | `world`, `map`, `link`, `lock`, `key`, `search`, `trash`, `heart`, `rocket`, `palette` |

## Choosing icons for new notes

Give structural notes an icon when you create them, so the tree is easy to scan.
Use one consistent icon per kind of thing, and match what is already in the
tree: look at the siblings' `trilium attr <sibling>` first.

- **Do** icon: folders/containers (projects, areas, an inbox), journals and
  calendars, templates, reference pages, notes the user will look for often.
- **Skip**: ordinary leaf notes (meeting notes, one-off captures) and many
  notes of the same kind; the default type icon is fine, and a tree full of
  custom icons stops helping.
- Prefer a meaningful regular icon (`bx bx-` prefix) over a decorative one.
- Never change icons of existing notes the user did not ask about; and never
  touch system notes (IDs starting with `_`) without `--force` and a reason.

**English** | [简体中文](README.zh-CN.md)

# trilium

*Work with your Trilium notes from the terminal — built for AI agents, pleasant for humans.*

`trilium` is a command-line tool for [Trilium Notes](https://github.com/TriliumNext/Trilium).
It talks to your Trilium server through its official API (ETAPI), so you can
search, read, write and organize your notes from a shell, a script, or an AI agent.

## What it does

- **Made for AI agents.** Short, stable output: one line per note, IDs first,
  one result line per change. A typical `get` prints about a quarter of what
  the raw API returns, and many tasks need a single call.
- **Exact.** Note content is read and written byte for byte. The tool never
  rewrites your HTML or code behind your back.
- **Covers the whole API.** Notes, the tree (moves and clones), labels and
  relations, attachments, revisions, journal notes, history, undelete,
  import and export.
- **Careful with your data.** Refuses to move a note into its own subtree, to
  delete a note's last placement by accident, to overwrite an existing note
  with `create --id`, or to change Trilium's system notes without `--force`.
  When an error follows a partial change, the message says so. Deleted notes
  can be restored.
- **One small binary.** No runtime, no config file: two environment variables
  and you're done.

## Install

**Let your AI agent do it.** Copy the text below into Claude Code, Codex or
another coding agent. It installs the CLI and the skill, or updates both if
they are already installed — run it again whenever you want the latest version.

```text
Install or update the `trilium` CLI (https://github.com/oyasmi/trilium-cli) and its agent skill on this machine. Work step by step, and tell me what you did at the end.

1. Check the current state: run `trilium --version` (note the version and, if it exists, where the binary lives: `command -v trilium`, or `where trilium` on Windows). Also look for an existing skill folder named `trilium` in your own skills directory.
2. Find the latest release: GET https://api.github.com/repos/oyasmi/trilium-cli/releases/latest and read `tag_name` (e.g. v0.1.0; the binary prints it without the "v": `trilium 0.1.0 (commit …)`). If the installed binary already reports that version, skip step 5 but still do the rest: the skill may be outdated or missing.
3. Pick the asset for this OS and CPU: `trilium_<version without v>_<os>_<arch>.tar.gz` with os = darwin | linux and arch = amd64 | arm64, or `…_windows_<arch>.zip` on Windows. Download it and `checksums.txt` from the same release into a new temporary directory, and verify the SHA-256 of the archive against checksums.txt. Stop if it doesn't match.
4. Unpack the archive. It contains the `trilium` binary (`trilium.exe` on Windows), and the skill in `skills/trilium/`.
5. Install the binary:
   - Updating: replace the existing binary at the same path.
   - New install: use `~/.local/bin` (create it if needed), or `%LOCALAPPDATA%\Programs\trilium` on Windows. If that directory isn't on PATH, tell me the exact line to add to my shell profile; don't edit the profile yourself.
   - On macOS, if the binary is blocked as downloaded from the internet, run `xattr -d com.apple.quarantine <path>`.
   - Ask me before using sudo or writing outside my home directory.
6. Install the skill: copy the whole `skills/trilium` folder into the skills directory of the agent you are (Claude Code: `~/.claude/skills/trilium`; Codex: `~/.codex/skills/trilium`; other agents: your equivalent, or ask me). If a `trilium` skill folder already exists, compare it with the new one; if they differ, replace the old folder entirely (no stale reference files may remain), otherwise leave it.
7. Verify: `trilium --version` must print the new version. If TRILIUM_URL and TRILIUM_ETAPI_TOKEN are set, also run `trilium info` and check it shows `auth:    ok`. If they aren't set, tell me how to set them: in Trilium, open Options → ETAPI, create a token, then `export TRILIUM_URL=<my Trilium address>` and `export TRILIUM_ETAPI_TOKEN=<token>`. Never print a token, and don't write it to any file unless I ask.
8. Remove the temporary directory and report: binary old → new version (or "already up to date"), skill installed / updated / unchanged, both paths, and anything I still need to do.
```

**Or download it yourself**: get the archive for your system from the
[releases page](https://github.com/oyasmi/trilium-cli/releases), unpack it, and put
`trilium` somewhere on your `PATH` (for example `/usr/local/bin`).

**Or build it with Go** (1.27 or newer):

```bash
go install github.com/oyasmi/trilium-cli/cmd/trilium@latest
```

## Set up (3 steps)

1. In Trilium, open **Options → ETAPI** and click **Create new ETAPI token**. Copy the token.
2. Tell `trilium` where your server is and which token to use:

   ```bash
   export TRILIUM_URL=https://notes.example.com     # the address you open Trilium with
   export TRILIUM_ETAPI_TOKEN=<your token>
   ```

   Add these lines to your `~/.bashrc` or `~/.zshrc` to keep them.
3. Check that it works:

   ```
   $ trilium info
   server:  https://notes.example.com (Trilium 0.106.1)
   auth:    ok
   notes:   424 active, 160 deleted
   inbox:   tA1b2C3d "Inbox"
   agent:   (not set)
   ```

## Quick start

```
$ trilium search budget --in N2lwDR1ByORY
2CgAMHEAjVXT	text	Budget review

$ trilium get 2CgAMHEAjVXT
---
id: 2CgAMHEAjVXT
title: Budget review
type: text (text/html)
parents: N2lwDR1ByORY
attrs: #status=todo
children: 0
attachments: 0
created: 2026-10-25 18:42
modified: 2026-10-25 18:42
---
<p>Q3 budget approved.</p>

$ trilium create "Supplier call" "<p>Delivery moved to Friday.</p>" -p N2lwDR1ByORY
created J7UZ1SubYZw6 text "Supplier call" in N2lwDR1ByORY

$ trilium append @today "<p>Called the supplier.</p>"
appended HN04PrpcQg3H (@today) +27 bytes: 58 → 85 bytes

$ trilium ls N2lwDR1ByORY
2CgAMHEAjVXT	text	Budget review
OaVTKVwlYbCb	text	Launch checklist
J7UZ1SubYZw6	text	Supplier call
```

Without `-p`, `create` puts new notes in your inbox (`@inbox`). If no note has the label `#inbox`, they go to today's journal note and `trilium` warns you; `trilium info` shows which note is your inbox.
Run `trilium help` for the list of commands and `trilium help <command>` for details.

## Everyday use

**Search** — Trilium's own search syntax; `trilium help search` has a cheat sheet.

```bash
trilium search meeting notes
trilium search '#status=todo' --in <project> --sort dateModified --desc
trilium search 'note.title="report.pdf"'      # quote values with "." or "-"
```

**Read** — several notes at once; `--text` for a quick plain-text view.

```bash
trilium get <id1> <id2>
trilium get <id> --text
trilium get <id> -c > page.html                # raw content only
```

**Write** — text notes store HTML. Long content is easiest from a file.

```bash
trilium create "Release plan" -f plan.html -p <parent>
trilium create -f script.py -p <parent>        # a .py file becomes a code note
trilium append <id> "<p>One more thing.</p>"
trilium put <id> -f page.html                  # replace everything
```

**Change part of a note** — exact search and replace, all-or-nothing.

```
$ trilium edit 2CgAMHEAjVXT --old "<p>Q3 budget approved.</p>" --new "<p>Q3 budget approved (final).</p>"
edited 2CgAMHEAjVXT 1 replacement: 26 → 34 bytes
```

**Organize** — labels, moves, clones.

```
$ trilium attr 2CgAMHEAjVXT '#status=done'
set #status=done on 2CgAMHEAjVXT
$ trilium mv OaVTKVwlYbCb 1DKciU1K11Yy
moved OaVTKVwlYbCb "Launch checklist": N2lwDR1ByORY → 1DKciU1K11Yy
$ trilium branch add OaVTKVwlYbCb N2lwDR1ByORY
cloned OaVTKVwlYbCb into N2lwDR1ByORY (branch N2lwDR1ByORY_OaVTKVwlYbCb)
```

**Attachments** — upload, then paste the `embed:` line into the note.

```
$ trilium att add 2CgAMHEAjVXT -f chart.png
added attachment ldExtmPZnwBR to 2CgAMHEAjVXT: "chart.png" image/png 69 bytes
embed: <img src="api/attachments/ldExtmPZnwBR/image/chart.png">
```

**Journal** — `@today`, `@yesterday`, `@2026-10-25`, `@2026-10` …

```bash
trilium append @today -f entry.html
trilium ls @2026-10                            # the days of a month
```

**Markdown** — Trilium converts it into a rich text note for you.

```
$ trilium import N2lwDR1ByORY plan.md
imported 2EfMSwP15Bnb "plan" into N2lwDR1ByORY
```

**Undo** — find and restore deleted notes.

```bash
trilium history -n 20
trilium undelete <id>
```

## Use with AI agents

The repository ships a ready-made skill in [`skills/trilium`](skills/trilium/)
that teaches an agent how to use the tool well: the core concepts, the HTML
that Trilium expects, common patterns and pitfalls.

- **Claude Code**: copy the folder to `~/.claude/skills/trilium` (all
  projects) or `.claude/skills/trilium` inside a project.

  ```bash
  cp -r skills/trilium ~/.claude/skills/
  ```

- **Codex**: copy the folder to `~/.codex/skills/trilium`.

  ```bash
  cp -r skills/trilium ~/.codex/skills/
  ```

- **Other agents**: add the contents of `skills/trilium/SKILL.md` to the
  system prompt, and make the files in `references/` available to read.
- **Several agents on one knowledge base**: give each one a name with
  `TRILIUM_AGENT=<name>`. Every note it creates gets the label
  `#agent=<name>`, so you can see who wrote what:
  `trilium search '#agent="claude-1"'`.
- **One writer per note.** Trilium's API has no conditional write, so two
  agents changing the same note at the same moment can still lose an update.
  Give each agent its own notes (for a shared log: one note per entry).

Add `--json` to any command to get JSON Lines (ETAPI field names) for scripts.

## Note shortcuts

Anywhere a note is expected you can write a note ID, `root`, or:

| Shortcut | Means |
|---|---|
| `@inbox` | your inbox note (the note labelled `#inbox`, otherwise today's journal note) |
| `@today` `@yesterday` `@tomorrow` | journal note of that day |
| `@2026-10-25` | journal note of a date |
| `@2026-W43` | week note (needs `#enableWeekNote` on the calendar root) |
| `@2026-10` / `@2026` | month / year note |

Journal shortcuts create the note if it doesn't exist yet, just like Trilium
does. Dates use your computer's time zone.

## Commands

| Group | Commands |
|---|---|
| Everyday | `search` `ls` `get` `create` `append` `edit` `put` `rm` |
| Organize | `update` (title/type) · `attr` (labels, relations) · `mv` · `branch` (clones) · `att` (attachments) · `rev` (revisions) |
| More | `history` · `export` · `import` · `undelete` · `info` · `backup` · `version` |

Exit codes: `0` success (also "no matches"), `1` failed, `2` wrong usage or
missing settings, `3` not found.

## Troubleshooting

- **`TRILIUM_URL is not set` / `TRILIUM_ETAPI_TOKEN is not set`** — export
  both variables (see *Set up*). The URL is the address you open Trilium with;
  a trailing `/` or `/etapi` is fine.
- **`could not reach TRILIUM_URL`** — check the address, your network, and
  that the server is running.
- **`Not authenticated [NOT_AUTHENTICATED]`** — the token is wrong or was
  deleted. Create a new one under Options → ETAPI.
- **Search finds nothing** — values containing `.` or `-` must be quoted:
  `note.title="hello.py"`, `#date="2026-10-25"`. Also quote the whole query for
  your shell: `trilium search '#status=todo'`.
- **My text shows up as one long line** — text notes store HTML. Wrap
  paragraphs in `<p>…</p>`, use a code note for plain text, or
  `trilium import <parent> file.md` to convert Markdown.
- **Protected notes** can't be read or changed through the API.

## Compatibility

Tested with Trilium (TriliumNext) **0.106.1**. Older servers may lack some
endpoints (undelete, history, revision lists, week notes, metrics); the other
commands keep working.

## Contributing

Issues and pull requests are welcome. The design is described in
[docs/design.md](docs/design.md) (Chinese).

```bash
make build      # ./trilium
make test       # go vet + unit and command tests
make lint       # golangci-lint
make e2e        # end-to-end tests against a real server (writes only inside a temporary sandbox note)
```

## License

[Apache License 2.0](LICENSE)

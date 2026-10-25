#!/usr/bin/env bash
# Measures how many bytes typical commands print, compared with the raw ETAPI
# responses they replace. Runs in a sandbox note under root that is deleted
# at the end. Needs TRILIUM_URL, TRILIUM_ETAPI_TOKEN, curl and a built ./trilium.
set -euo pipefail

T=${TRILIUM_BIN:-./trilium}
: "${TRILIUM_URL:?set TRILIUM_URL}" "${TRILIUM_ETAPI_TOKEN:?set TRILIUM_ETAPI_TOKEN}"
E="${TRILIUM_URL%/}"
E="${E%/etapi}/etapi"
export TRILIUM_AGENT=

etapi() { curl -s --compressed -H "Authorization: $TRILIUM_ETAPI_TOKEN" "$@"; }
bytes() { wc -c | tr -d ' '; }
id_of() { awk '{print $2}'; }

SB=$($T create "trilium-cli-token-audit" -p root -t book | id_of)
trap '$T rm "$SB" -r >/dev/null' EXIT

# A note with three attributes.
N=$($T create "Quarterly planning meeting" "<p>Agenda</p>" -p "$SB" -a '#status=draft' -a '#priority=high' -a "~seeAlso=$SB" | id_of)

# 50 nodes in two levels for ls -d 2 (5 folders x 9 children), and 10 search hits.
for i in 1 2 3 4 5; do
  F=$($T create "Project folder $i" -p "$SB" -t book | id_of)
  for j in 1 2 3 4 5 6 7 8 9; do
    $T create "Task $i.$j review the budget" -p "$F" -a '#audit' >/dev/null
  done
done

row() { printf '| %s | %s | %s | %s |\n' "$1" "$2" "$3" "$4"; }

echo "| Scenario | trilium (bytes) | raw ETAPI (bytes) | Target |"
echo "|---|---|---|---|"

cli=$($T get "$N" -m | bytes)
raw=$(etapi "$E/notes/$N" | bytes)
row "get -m, note with 3 attributes" "$cli" "$raw" "≤ 300"

out=$($T search '#audit' --in "$SB" -n 10 2>/dev/null)
lines=$(printf '%s\n' "$out" | wc -l | tr -d ' ')
cli=$(printf '%s\n' "$out" | bytes)
titles=$(printf '%s\n' "$out" | cut -f3 | tr -d '\n' | bytes)
raw=$(etapi -G --data-urlencode 'search=#audit' --data-urlencode "ancestorNoteId=$SB" --data-urlencode limit=10 "$E/notes" | bytes)
row "search, $lines results (per line, excluding title)" "$(( (cli - titles) / lines ))" "$(( raw / lines ))" "≤ 60"

line=$($T create "Weekly sync notes" "<p>x</p>" -p "$SB")
row "create result line (excluding title)" "$(( ${#line} - 17 ))" "-" "≤ 100"
C=$(echo "$line" | id_of)
line=$($T append "$C" "<p>more</p>")
row "append result line" "${#line}" "-" "≤ 100"
line=$($T edit "$C" --old "<p>x</p>" --new "<p>y</p>")
row "edit result line" "${#line}" "-" "≤ 100"

cli=$($T ls "$SB" -d 2 -n 0 | bytes)
nodes=$($T ls "$SB" -d 2 -n 0 | wc -l | tr -d ' ')
raw=$(( $(etapi "$E/notes/$SB" | bytes) + $(etapi -G --data-urlencode 'search=note.noteId != ""' --data-urlencode "ancestorNoteId=$SB" --data-urlencode ancestorDepth=lt3 "$E/notes" | bytes) ))
row "ls -d 2, $nodes nodes" "$cli" "$raw" "≤ 3072"

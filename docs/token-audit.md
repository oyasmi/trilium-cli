# Token audit

Output size of typical commands compared with the raw ETAPI responses they
replace. Produced by `make tokens` (`scripts/token-audit.sh`) against Trilium
0.106.1 on 2026-10-25.

| Scenario | trilium (bytes) | raw ETAPI (bytes) | Target |
|---|---|---|---|
| get -m, note with 3 attributes | 242 | 1022 | ≤ 300 |
| search, 10 results (per line, excluding title) | 19 | 640 | ≤ 60 |
| create result line (excluding title) | 44 | - | ≤ 100 |
| append result line | 45 | - | ≤ 100 |
| edit result line | 48 | - | ≤ 100 |
| ls -d 2, 52 nodes | 2386 | 35857 | ≤ 3072 |

All targets are met. `ls -d 2` needs two requests and prints about 7% of
what those requests return; `get -m` replaces two requests (note and
attachments) and prints about a quarter of the note JSON alone.

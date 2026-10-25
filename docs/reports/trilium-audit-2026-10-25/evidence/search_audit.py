import json
import os
from pathlib import Path
import subprocess
import uuid
env = dict(os.environ, TRILIUM_AGENT='')
records = []
def cli(*args):
    p = subprocess.run([str(Path.home()/'bin/trilium'), *args], env=env, capture_output=True, timeout=90)
    records.append(dict(args=list(args), exit=p.returncode, stdout=p.stdout.decode(), stderr=p.stderr.decode()))
    return p
def obj(*args):
    p = cli('--json', *args)
    assert p.returncode == 0
    return json.loads(p.stdout)
s = obj('create', 'Search audit '+uuid.uuid4().hex[:8], '-p', 'root', '-t', 'book')['noteId']
name = 'probe'+uuid.uuid4().hex[:8]
try:
    n = obj('create', 'Needle unique', '<p>Search check</p>', '-p', s, '-a', '#'+name+'=codex-audit')['noteId']
    for query in ['note.noteId="'+n+'"', '#'+name, '#'+name+'=codex-audit', '#'+name+'="codex-audit"', '#agent="codex-audit"']:
        cli('search', query, '--in', s)
finally:
    cli('rm', s, '-r')
    Path('/tmp/trilium-audit-20261025/search-results.json').write_text(json.dumps(records, ensure_ascii=False, indent=2))
    print(json.dumps(records[2:-1], ensure_ascii=False, indent=2))

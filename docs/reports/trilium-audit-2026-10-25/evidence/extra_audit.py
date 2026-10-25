import base64
import json
import os
from pathlib import Path
import subprocess
import uuid

base = Path('/tmp/trilium-audit-20261025')
env = dict(os.environ, TRILIUM_AGENT='codex-audit')
records = []
def cli(*args, data=None):
    p = subprocess.run([str(Path.home()/'bin/trilium'), *args], input=data, capture_output=True, env=env, timeout=90)
    records.append(dict(args=list(args), exit=p.returncode, stdout=p.stdout.decode('utf-8', 'replace'), stderr=p.stderr.decode('utf-8', 'replace')))
    return p
def obj(*args):
    p = cli('--json', *args)
    assert p.returncode == 0, records[-1]
    return json.loads(p.stdout)
s = obj('create', 'Extra audit '+uuid.uuid4().hex[:8], '-p', 'root', '-t', 'book')['noteId']
findings = {}
try:
    body = b'<p>stdin \xe4\xb8\xad\xe6\x96\x87</p>\n'
    n = json.loads(cli('--json', 'create', 'Stdin', '-p', s, '-f', '-', data=body).stdout)['noteId']
    findings['stdin_roundtrip'] = cli('get', n, '-c').stdout == body
    unquoted = cli('search', '#agent=codex-audit', '--in', s)
    quoted = cli('search', '#agent="codex-audit"', '--in', s)
    findings['hyphen_agent_search'] = dict(unquotedExit=unquoted.returncode, unquotedMatches=n in unquoted.stdout.decode(), quotedMatches=n in quoted.stdout.decode())
    cli('rev', 'add', n)
    revision = json.loads(cli('--json', 'rev', 'ls', n).stdout.splitlines()[0])['revisionId']
    out = cli('--json', 'rev', 'get', revision, '-o', str(base/'revision.html'))
    try:
        json.loads(out.stdout)
        is_json = True
    except ValueError:
        is_json = False
    findings['revision_save_json'] = dict(exit=out.returncode, json=is_json, stdout=out.stdout.decode())
    png = base/'pixel.png'
    png.write_bytes(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII='))
    added = obj('att', 'add', n, '-f', str(png))
    aid = added['attachmentId']
    fetched = json.loads(cli('--json', 'att', 'ls', n).stdout.splitlines()[0])
    findings['binary_attachment_json_length'] = dict(reported=added['contentLength'], stored=fetched['contentLength'], expected=png.stat().st_size)
    image = obj('create', '-f', str(png), '-p', s)['noteId']
    cli('get', image, '-o', str(base/'pixel-out.png'))
    findings['image_note_roundtrip'] = (base/'pixel-out.png').read_bytes() == png.read_bytes()
    md = base/'source.md'
    md.write_text('# Raw Markdown\n')
    code = obj('create', '-f', str(md), '-p', s)
    findings['markdown_source_inference'] = dict(type=code['type'], mime=code['mime'], exact=cli('get', code['noteId'], '-c').stdout == md.read_bytes())
finally:
    cli('rm', s, '-r')
    base.joinpath('extra-commands.json').write_text(json.dumps(records, ensure_ascii=False, indent=2))
    base.joinpath('extra-results.json').write_text(json.dumps(findings, ensure_ascii=False, indent=2))
    print(json.dumps(dict(commands=len(records), results=findings), ensure_ascii=False, indent=2))

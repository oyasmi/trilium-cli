import concurrent.futures as cf
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import uuid

BASE = Path('/tmp/trilium-audit-20261025')
BIN = str(Path.home() / 'bin/trilium')
ENV = dict(os.environ, TRILIUM_AGENT='codex-audit')
records = []
checks = []

def cli(*args, expect=0, data=None, env=None):
    p = subprocess.run([BIN, *args], input=data, capture_output=True, env=env or ENV, timeout=90)
    rec = dict(args=list(args), exit=p.returncode, stdout=p.stdout.decode('utf-8', 'replace'), stderr=p.stderr.decode('utf-8', 'replace'))
    # root metadata contains personal titles/attributes: do not persist it.
    if args != ('--json', 'get', 'root', '-m'):
        records.append(rec)
    if expect is not None and p.returncode != expect:
        raise AssertionError(rec)
    return p

def obj(*args):
    return json.loads(cli('--json', *args).stdout)

def create(title, parent, body=None, *extra):
    args = ['create', title]
    if body is not None:
        args.append(body)
    return obj(*args, '-p', parent, *extra)['noteId']

def check(name, passed, detail=''):
    checks.append(dict(name=name, passed=bool(passed), detail=detail))
    print(('PASS ' if passed else 'FINDING ') + name + (' | ' + detail if detail else ''), flush=True)

before = obj('get', 'root', '-m')['childNoteIds']
sandbox = None
try:
    sandbox = create('CLI audit ' + uuid.uuid4().hex[:8], 'root', None, '-t', 'book')
    (BASE / 'sandbox-id.txt').write_text(sandbox)
    a = create('A', sandbox, None, '-t', 'book')
    b = create('B', sandbox, None, '-t', 'book')
    n = create('中文与 HTML', a, '<h2>审查</h2><p>Alpha &amp; 中文</p>', '-a', '#status=draft')
    check('create agent label', '#agent=codex-audit' in cli('attr', n).stdout.decode())
    check('raw HTML exact bytes', cli('get', n, '-c').stdout == '<h2>审查</h2><p>Alpha &amp; 中文</p>'.encode())
    check('plain text view', 'Alpha & 中文' in cli('get', n, '--text').stdout.decode())
    cli('append', n, '<p>Beta</p>')
    cli('edit', n, '--old', 'Beta', '--new', 'Gamma')
    check('append and edit', cli('get', n, '-c').stdout.endswith(b'<p>Gamma</p>'))
    original = cli('get', n, '-c').stdout
    cli('edit', n, '--old', 'Gamma', '--new', 'X', '--old', 'ABSENT', '--new', 'Y', expect=1)
    check('edit prevalidation no partial writes', cli('get', n, '-c').stdout == original)
    check('scoped attribute search', n in cli('search', '#status=draft', '--in', sandbox).stdout.decode())
    cli('attr', n, '#status=done', '+#tag=x', '+#tag=y', '-#tag=x', '~audit=' + b)
    check('attribute operations', '#status=done' in cli('attr', n).stdout.decode())
    cli('update', n, '--title', '已更新')
    cli('branch', 'add', n, b)
    check('clone has two parents', len(obj('get', n, '-m')['parentNoteIds']) == 2)
    cli('mv', n, sandbox, expect=2)
    cli('branch', 'rm', n, a)
    cli('branch', 'rm', n, b, expect=2)
    cli('mv', n, a)
    cli('branch', 'add', a, n, expect=1)
    cli('rm', a, expect=2)
    check('structure safety guards', True, 'ambiguous move, last placement, cycle and recursive-delete guards reject')
    cli('rev', 'add', n)
    revs = [json.loads(s) for s in cli('--json', 'rev', 'ls', n).stdout.splitlines()]
    cli('put', n, '<p>Replacement</p>')
    check('revision snapshot', 'Gamma' in cli('rev', 'get', revs[0]['revisionId']).stdout.decode())
    check('history under sandbox', n in cli('history', '--in', sandbox).stdout.decode())
    for name, content in [('sample.json', b'{"message":"JSON"}\n'), ('sample.bin', bytes(range(256))*16)]:
        src = BASE / name
        src.write_bytes(content)
        note = obj('create', '-f', str(src), '-p', sandbox)['noteId']
        dst = BASE / ('out-' + name)
        cli('get', note, '-o', str(dst))
        check(name + ' note round trip', dst.read_bytes() == content)
        att = obj('att', 'add', n, '-f', str(src))['attachmentId']
        cli('att', 'get', att, '-o', str(dst))
        check(name + ' attachment round trip', dst.read_bytes() == content)
        if name.endswith('.bin'):
            cli('get', note, '-c', expect=1)
        cli('att', 'update', att, '--title', 'new-' + name)
        cli('att', 'put', att, '-f', str(src))
        cli('att', 'rm', att)
    md = BASE / 'sample.md'
    md.write_text('# 标题\n\nA **bold** word.\n')
    imp = obj('import', sandbox, str(md))['noteId']
    check('Markdown import converts to HTML', '<strong>bold</strong>' in cli('get', imp, '-c').stdout.decode())
    export = BASE / 'sandbox.zip'
    cli('export', a, '-o', str(export))
    cli('import', b, str(export))
    check('ZIP export/import', n not in cli('ls', b).stdout.decode() and '已更新' in cli('ls', b, '-d', '3').stdout.decode())
    deleted = create('Delete recovery', sandbox, '<p>recover</p>')
    cli('rm', deleted)
    cli('get', deleted, expect=3)
    cli('undelete', deleted)
    check('soft delete/undelete', cli('get', deleted, '-c').stdout == b'<p>recover</p>')
    stable = 'audit' + uuid.uuid4().hex[:12]
    create('Stable', sandbox, '<p>once</p>', '--id', stable)
    duplicate = cli('create', 'Stable second request', '<p>twice</p>', '-p', sandbox, '--id', stable, expect=None)
    stable_body = cli('get', stable, '-c').stdout
    check('create stable ID rejects existing ID', duplicate.returncode != 0 and stable_body == b'<p>once</p>', f'exit={duplicate.returncode}, content={stable_body!r}')
    cli('get', '@2026-13-45', expect=2)
    cli('rm', 'root', expect=2)
    check('JSON error', json.loads(cli('--json', 'get', 'doesNotExist1', expect=3).stderr)['error']['code'] == 'NOTE_NOT_FOUND')
    # Chained replacements, explicitly allowed by help: apply in order.
    chain = create('Chained edit', sandbox, 'A', '-t', 'code')
    p = cli('edit', chain, '--old', 'A', '--new', 'B', '--old', 'B', '--new', 'C', expect=None)
    final = cli('get', chain, '-c').stdout.decode()
    check('chained edit reports success', p.returncode == 0 and final == 'C', f'exit={p.returncode}, content={final!r}, stderr={p.stderr.decode().strip()}')
    # A normal stress run supplements deterministic concurrency testing later.
    log = create('Concurrent append', sandbox, '<p>start</p>')
    pieces = [f'<p>writer-{i}</p>' for i in range(8)]
    with cf.ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(lambda s: cli('append', log, s, expect=None), pieces))
    body = cli('get', log, '-c').stdout.decode()
    missing = [i for i, s in enumerate(pieces) if s not in body]
    duplicates = [i for i, s in enumerate(pieces) if body.count(s) > 1]
    check('8 writers append exactly once', all(p.returncode == 0 for p in results) and not missing and not duplicates, f'exits={[p.returncode for p in results]}, missing={missing}, duplicates={duplicates}')
    # Shared descendants should survive removal of one ancestor.
    shared_parent = create('Shared parent', sandbox, None, '-t', 'book')
    shared = create('Shared child', shared_parent, '<p>shared</p>')
    cli('branch', 'add', shared, b)
    cli('rm', shared_parent, '-r')
    p = cli('get', shared, '-m', expect=None)
    check('shared descendant survives ancestor delete', p.returncode == 0, f'exit={p.returncode}')
    # TSV contracts with legal titles containing delimiters.
    tabnote = create('Tab\tTitle\nSecond line', sandbox, '<p>tab</p>')
    p = cli('search', f'note.noteId="{tabnote}"', '--in', sandbox)
    check('one row per note with delimiter title', len(p.stdout.splitlines()) == 1 and len(p.stdout.rstrip(b'\n').split(b'\t')) == 3, repr(p.stdout.decode()))
    # Relation replacement: a target may be syntactically valid yet nonexistent.
    rel = create('Relation failure', sandbox, '<p>rel</p>', '-a', '~target=' + a)
    p = cli('attr', rel, '~target=doesNotExist1', expect=None)
    attrs = obj('get', rel, '-m')['attributes']
    targets = [x['value'] for x in attrs if x['name'] == 'target' and x['noteId'] == rel]
    check('failed relation update preserves old value', p.returncode != 0 and targets == [a], f'exit={p.returncode}, targets={targets}')
finally:
    if sandbox:
        cli('rm', sandbox, '-r')
    after = obj('get', 'root', '-m')['childNoteIds']
    check('root children restored exactly', before == after, f'before={len(before)}, after={len(after)}')
    (BASE / 'live-commands.json').write_text(json.dumps(records, ensure_ascii=False, indent=2))
    (BASE / 'live-checks.json').write_text(json.dumps(checks, ensure_ascii=False, indent=2))
    print(f'{len(records)} commands; {len(checks)} checks; {sum(not c["passed"] for c in checks)} findings', flush=True)

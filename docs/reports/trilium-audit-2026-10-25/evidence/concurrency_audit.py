import concurrent.futures as cf
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import threading

body = b'<p>seed</p>'
lock = threading.Lock()
barrier = threading.Barrier(2)
a_verified = threading.Event()
meta_counts = {}
content_counts = {}
events = []

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def respond(self, data, status=200, ct='application/json'):
        self.send_response(status)
        self.send_header('Content-Type', ct)
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        writer = self.headers['Authorization']
        if self.path == '/etapi/notes/auditnote':
            with lock:
                meta_counts[writer] = meta_counts.get(writer, 0) + 1
                count = meta_counts[writer]
                snapshot = body
                events.append(f'{writer}: metadata read {count}')
            if count == 2:
                barrier.wait(timeout=20)
            data = dict(noteId='auditnote', title='Audit', type='text', mime='text/html', blobId=hashlib.sha256(snapshot).hexdigest(), attributes=[], parentNoteIds=['root'], childNoteIds=[], parentBranchIds=['root_auditnote'], childBranchIds=[])
            self.respond(json.dumps(data).encode())
        elif self.path == '/etapi/notes/auditnote/content':
            with lock:
                content_counts[writer] = content_counts.get(writer, 0) + 1
                count = content_counts[writer]
                snapshot = body
                events.append(f'{writer}: content read {count}: {snapshot.decode()}')
            self.respond(snapshot, ct='text/html')
            if writer == 'A' and count == 2:
                a_verified.set()
        else:
            self.respond(b'{}', 404)

    def do_PUT(self):
        global body
        writer = self.headers['Authorization']
        incoming = self.rfile.read(int(self.headers['Content-Length']))
        if writer == 'B':
            assert a_verified.wait(timeout=20)
        with lock:
            body = incoming
            events.append(f'{writer}: wrote {body.decode()}')
        self.respond(b'', 204)

server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()

def run(writer):
    env = dict(os.environ, TRILIUM_URL=f'http://127.0.0.1:{server.server_port}', TRILIUM_ETAPI_TOKEN=writer)
    p = subprocess.run([str(Path.home()/'bin/trilium'), 'append', 'auditnote', f'<p>{writer}</p>'], env=env, capture_output=True, timeout=30)
    return dict(writer=writer, exit=p.returncode, stdout=p.stdout.decode(), stderr=p.stderr.decode())

try:
    with cf.ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(run, ['A', 'B']))
    result = dict(kind='deterministic local ETAPI model; installed binary', results=results, events=events, finalContent=body.decode(), lostA=b'<p>A</p>' not in body)
    Path('/tmp/trilium-audit-20261025/concurrency-result.json').write_text(json.dumps(result, indent=2))
    print(json.dumps(result, indent=2))
finally:
    server.shutdown()

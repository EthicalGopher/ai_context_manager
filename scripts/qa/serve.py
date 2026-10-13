"""Serves frontend/dist plus /media/<id> sample files for visual QA (see scripts/ui-qa.sh)."""
import http.server, os, sys
root, media_dir = sys.argv[1], sys.argv[2]
files = {'1': 'login-bug.png', '2': 'dashboard.png', '3': 'screen-recording.mp4'}
class H(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **k): super().__init__(*a, directory=root, **k)
    def do_GET(self):
        if self.path.startswith('/media/'):
            f = files.get(self.path.split('/')[2].split('#')[0])
            p = os.path.join(media_dir, f or '')
            if f and os.path.exists(p):
                data = open(p, 'rb').read()
                self.send_response(200); self.send_header('Content-Type', 'video/mp4' if f.endswith('mp4') else 'image/png'); self.send_header('Content-Length', str(len(data))); self.end_headers(); self.wfile.write(data); return
            self.send_error(404); return
        return super().do_GET()
    def log_message(self, *a): pass
http.server.ThreadingHTTPServer(('127.0.0.1', int(sys.argv[3])), H).serve_forever()

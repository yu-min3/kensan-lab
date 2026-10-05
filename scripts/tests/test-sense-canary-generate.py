import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from types import SimpleNamespace
import unittest

source = Path(__file__).resolve().parents[1] / 'sense-canary-generate.py'
spec = importlib.util.spec_from_file_location('generator', source)
module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
class HostClientTests(unittest.TestCase):
    def test_private_file_route_proof_and_new_output(self):
        files = [{'path':p,'base64Content':base64.b64encode(b'a').decode(),'executable':False} for p in ['apps/canary/app/main.py','kubernetes/apps/app-canary/values.yaml']]
        h = hashlib.sha256()
        for f in sorted(files,key=lambda f:f['path']): h.update((f['path']+'\0'+hashlib.sha256(b'a').hexdigest()+'\n').encode())
        response = {'templateSHA256':module.PIN,'filesSHA256':h.hexdigest(),'files':files}
        mode = ['ok']; calls = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self,*args): pass
            def do_POST(self):
                calls.append(self.path)
                self.server.test.assertEqual(self.path,'/api/sense-canary/v1/generate')
                self.server.test.assertEqual(self.headers['Authorization'],'Bearer '+('fixture-'*8))
                length = int(self.headers['Content-Length']); body=json.loads(self.rfile.read(length))
                self.server.test.assertEqual(set(body),{'description','theme','message'})
                if mode[0]=='redirect':
                    self.send_response(302); self.send_header('Location','http://127.0.0.1:1/evil'); self.end_headers(); return
                result = response if mode[0]=='ok' else dict(response,filesSHA256='0'*64)
                self.send_response(200); self.end_headers(); self.wfile.write(json.dumps(result).encode())
        server=HTTPServer(('127.0.0.1',0),Handler);server.test=self
        thread=threading.Thread(target=server.serve_forever);thread.start()
        try:
            with tempfile.TemporaryDirectory() as raw:
                root=Path(raw).resolve();token=root/'token';token.write_text('fixture-'*8);token.chmod(0o600)
                args=SimpleNamespace(base_url=f'http://127.0.0.1:{server.server_port}',token_file=str(token),output=str(root/'output'),description='Canary',theme='day',message='Hello')
                module.generate(args);self.assertTrue((root/'output/apps/canary/app/main.py').is_file())
                with self.assertRaises(FileExistsError):module.generate(args)
                for mode[0] in ['redirect','proof']:
                    args.output=str(root/mode[0])
                    with self.assertRaises(Exception):module.generate(args)
                    self.assertFalse(Path(args.output).exists())
                count=len(calls);token.chmod(0o644)
                with self.assertRaises(ValueError):module.generate(args)
                token.chmod(0o600);args.base_url='https://8.8.8.8'
                with self.assertRaises(ValueError):module.generate(args)
                self.assertEqual(len(calls),count)
        finally:server.shutdown();thread.join();server.server_close()
if __name__=='__main__':unittest.main()

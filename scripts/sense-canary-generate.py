#!/usr/bin/env python3
"""Host adapter: fixed machine API, no worker credentials, no publication."""
import argparse
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import stat
import sys
import urllib.error
import urllib.parse
import urllib.request
import base64

PIN = 'd90fbe17df37c7d1aecf28386c440b73bf0f83d5077cd92e9394e74d33db136e'
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        raise ValueError('redirect forbidden')

def generate(args):
    url = urllib.parse.urlsplit(args.base_url)
    # Host operator chooses an existing private TLS endpoint or a host-local tunnel.
    private = False
    try:
        ip = ipaddress.ip_address(url.hostname)
        private = ip.version == 4 and any(ip in ipaddress.ip_network(c) for c in ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'])
    except ValueError:
        pass
    local = url.hostname == '127.0.0.1' and url.scheme == 'http'
    if not (private and url.scheme == 'https' or local) or url.username or url.password or url.path or url.query or url.fragment:
        raise ValueError('fixed private API endpoint required')
    token_path = Path(args.token_file)
    metadata = token_path.lstat()
    if token_path.resolve() != token_path or not stat.S_ISREG(metadata.st_mode) or metadata.st_mode & 0o077 or metadata.st_size > 4096:
        raise ValueError('private host credential file required')
    token = token_path.read_text().strip()
    if len(token) < 32 or any(c.isspace() for c in token):
        raise ValueError('invalid service credential')
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    body = json.dumps({ 'description': args.description, 'theme': args.theme, 'message': args.message }).encode()
    req = urllib.request.Request(args.base_url + '/api/sense-canary/v1/generate', data=body, headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'}, method='POST')
    with opener.open(req, timeout=35) as response:
        data = response.read((6 << 20) + 1)
    if len(data) > 6 << 20:
        raise ValueError('response exceeds bound')
    result = json.loads(data)
    if result.get('templateSHA256') != PIN or not isinstance(result.get('files'), list) or not 1 <= len(result['files']) <= 100:
        raise ValueError('generation proof mismatch')
    files = []; seen = set(); total = 0
    for f in result['files']:
        path = f['path']; parts = path.split('/')
        if not path.startswith(('apps/canary/', 'kubernetes/apps/app-canary/', 'kubernetes/argocd/applications/apps/app-canary/')) or any(not p or p in ('.', '..', '.git', '.github', '.gitea', '.backstage') or p.startswith('.env') for p in parts) or any(not (c.isascii() and (c.isalnum() or c in '_./-')) for c in path) or path in seen:
            raise ValueError('generation path rejected')
        content = base64.b64decode(f['base64Content'], validate=True); total += len(content)
        if total > 4 << 20 or type(f.get('executable', False)) is not bool:
            raise ValueError('generation output rejected')
        seen.add(path); files.append((path, content, f.get('executable', False)))
    if not {'apps/canary/app/main.py', 'kubernetes/apps/app-canary/values.yaml'} <= seen:
        raise ValueError('incomplete generation')
    h = hashlib.sha256()
    for path, content, _ in sorted(files):
        h.update((path + '\0' + hashlib.sha256(content).hexdigest() + '\n').encode())
    if h.hexdigest() != result.get('filesSHA256'):
        raise ValueError('generation content proof mismatch')
    # Never overlay an existing checkout or follow output symlinks.
    root = Path(args.output).absolute()
    if root.parent.resolve() != root.parent:
        raise ValueError('output parent symlink forbidden')
    root.mkdir(mode=0o700)
    for path, content, executable in files:
        target = root / path; target.parent.mkdir(parents=True, exist_ok=True)
        with target.open('xb') as f:
            f.write(content)
        target.chmod(0o700 if executable else 0o600)
    (root / 'generation-proof.json').write_text(json.dumps({k: result[k] for k in ['templateSHA256', 'filesSHA256']}) + '\n')
    print('Fixed canary generation saved; publication requires the independent Release Gate.')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', required=True)
    parser.add_argument('--token-file', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--description', default='Private canary')
    parser.add_argument('--theme', choices=['day', 'night'], default='day')
    parser.add_argument('--message', default='Hello from the platform')
    try:
        generate(parser.parse_args())
    except Exception:
        # No upstream payload, credential, redirect target, or raw HTTP error in logs.
        print('Fixed canary generation failed; no publication was attempted.', file=sys.stderr)
        sys.exit(1)

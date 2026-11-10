#!/usr/bin/env python3
"""Package reviewed Grafana snapshots; no cluster or Helm operations."""
import copy
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
OUTPUT = HERE.parents[1] / 'kubernetes/observability/grafana/resources'
REMAP = {'200ac8fdbfbb74b39aff88118e4d1c2c':'cw-detail-node',
         '85a562078cdf77779eaa1add43ccec1e':'cw-detail-namespace',
         'a164a7f0339f99e89cea5cb47e9be617':'cw-detail-workload',
         '6581e46e4e5c7ba40a07646395ef7b23':'cw-detail-pod',
         'cluster-watch-practical':'cluster-watch'}

def production(d):
    raw = json.dumps(d, ensure_ascii=False)
    for old, new in REMAP.items():
        raw = raw.replace(old, new)
    d = json.loads(raw)
    d['id'] = None
    d['tags'] = ['platform', 'watch'] + (['sample'] if 'sample' in d['uid'] or 'function-' in d['uid'] else [])
    if d['uid'].startswith('cw-detail-'):
        d['title'] = 'Cluster Watch / ' + d['title']
    return d

def emit(name, dashboards, filename):
    content = ('apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ' + name +
               '\n  namespace: monitoring\n  labels:\n    grafana_dashboard: "1"\n    grafana_folder: "Platform"\ndata:\n')
    for d in sorted(dashboards, key=lambda v: v['uid']):
        content += '  ' + d['uid'] + '.json: |\n'
        content += ''.join('    ' + line + '\n' for line in json.dumps(d, ensure_ascii=False, indent=2 if len(dashboards) == 1 else None).splitlines())
    assert len(content.encode()) < 950000, 'ConfigMap too large'
    (OUTPUT / filename).write_text(content)

def build():
    docs = [production(json.loads(p.read_text())) for p in sorted((HERE / 'validated-dashboards').glob('*.json'))]
    home = copy.deepcopy(next(d for d in docs if d['uid'] == 'cluster-watch-app-sample'))
    home.update(uid='cluster-watch', title='Cluster Watch', tags=['platform', 'watch'])
    home['links'] = [
        {'type':'link', 'title':'通信機能の表示例 / SAMPLE', 'url':'/d/cw-function-normal/sample'},
        {'type':'link', 'title':'従来のCluster Watch', 'url':'/d/cluster-watch-classic/overview'}]
    docs.append(home)
    docs.append(production(json.loads((HERE / 'classic.json').read_text())))
    assert len({d['uid'] for d in docs}) == len(docs)
    assert len({d['title'] for d in docs}) == len(docs), 'Grafana requires unique titles'
    samples = []
    for d in docs:
        if 'sample' in d['tags']:
            samples.append(d)
        else:
            filename = 'cluster-watch-dashboard.yaml' if d['uid'] == 'cluster-watch' else d['uid'] + '-dashboard.yaml'
            name = 'grafana-cluster-watch-dashboard' if d['uid'] == 'cluster-watch' else 'grafana-' + d['uid'] + '-dashboard'
            emit(name, [d], filename)
    emit('grafana-cluster-watch-samples', samples, 'cluster-watch-samples-dashboard.yaml')
    print(f'{len(docs)} dashboards packaged; samples grouped in one ConfigMap')

if __name__ == '__main__':
    build()

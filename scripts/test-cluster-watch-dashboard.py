#!/usr/bin/env python3
"""Check actual dashboard queries with promtool (Docker image prom/prometheus:v3.1.0)."""
import json
from pathlib import Path
import subprocess
import tempfile

repo = Path(__file__).resolve().parents[1]
manifest = (repo / 'kubernetes/observability/grafana/resources/cluster-watch-dashboard.yaml').read_text()
dashboard = json.loads(manifest.split('  cluster-watch.json: |\n', 1)[1])
panels = {p['id']: p for p in dashboard['panels']}
health = {'series': 'cluster_watch:ksm_healthy:bool', 'values': '1+0x10'}
counter = {'series': 'kube_pod_container_status_restarts_total{namespace="example",pod="app",container="main"}', 'values': '0+0x10'}
rule_group = {'series': 'prometheus_rule_group_last_evaluation_timestamp_seconds{rule_group="rules;kubernetes-apps"}', 'values': '0+60x10'}
tests = []


def case(name, panel, series, labels, value, time='10m'):
    tests.append({'name': name, 'interval': '1m', 'input_series': series,
                  'promql_expr_test': [{'expr': panels[panel]['targets'][0]['expr'], 'eval_time': time,
                                       'exp_samples': [{'labels': labels, 'value': value}]}]})


for panel in (5, 7, 8):
    target = 'target' if panel == 8 else 'pod'
    extra = ',reason="対象なし"' if panel == 5 else ',alertname="発火なし"' if panel == 8 else ''
    base = [health] + ([counter] if panel == 7 else [rule_group] if panel == 8 else [])
    case(f'{panel}: healthy empty is explicit', panel, base,
         '{namespace="—",' + target + '="対象なし"' + extra + '}', 0)
    extra_unknown = ',reason="取得不能"' if panel == 5 else ',alertname="取得不能"' if panel == 8 else ''
    unknown = '{namespace="—",' + target + '="取得不能"' + extra_unknown + '}'
    case(f'{panel}: failed monitoring is not empty', panel,
         [{'series': 'cluster_watch:ksm_healthy:bool', 'values': '0+0x10'}], unknown, -1)
    case(f'{panel}: missing monitoring is not empty', panel, [], unknown, -1)
    case(f'{panel}: stale monitoring is not empty', panel,
         [{'series': 'cluster_watch:ksm_healthy:bool', 'values': '1 _ _ _'}], unknown, -1, '3m')

case('issues remain visible without an empty sentinel', 5,
     [health, {'series': 'cluster_watch:pod_issue:present{namespace="example",pod="app",reason="CrashLoopBackOff"}', 'values': '1+0x10'}],
     '{__name__="cluster_watch:pod_issue:present",namespace="example",pod="app",reason="CrashLoopBackOff"}', 1)
case('restart metric missing is not a healthy zero', 7, [health], '{namespace="—",pod="取得不能"}', -1)
case('rule observer missing is not no alerts', 8, [health], '{namespace="—",target="取得不能",alertname="取得不能"}', -1)

(repo / 'temp').mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix='cluster-watch-', dir=repo / 'temp') as folder:
    target = Path(folder)
    (target / 'tests.json').write_text(json.dumps({'rule_files': [], 'evaluation_interval': '1m', 'tests': tests}, ensure_ascii=False))
    subprocess.run(['docker', 'run', '--rm', '--network=none', '--entrypoint', 'promtool',
                    '-v', f'{target}:/tests:ro', 'prom/prometheus:v3.1.0', 'test', 'rules', '/tests/tests.json'], check=True)
print(f'{len(tests)} dashboard query scenarios passed')

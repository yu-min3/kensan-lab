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
inventory = [
 {'series':'kube_pod_info{namespace="example",pod="app",uid="a"}', 'values':'1+0x10'},
 {'series':'kube_pod_status_ready{namespace="example",pod="app",uid="a",condition="true"}', 'values':'1+0x10'},
 {'series':'kube_pod_status_phase{namespace="example",pod="app",uid="a",phase="Running"}', 'values':'1+0x10'},
]


def case(name, panel, series, labels, value, time='10m'):
    tests.append({'name': name, 'interval': '1m', 'input_series': series,
                  'promql_expr_test': [{'expr': panels[panel]['targets'][0]['expr'], 'eval_time': time,
                                       'exp_samples': [{'labels': labels, 'value': value}]}]})


for panel in (5, 7, 8):
    target = 'target' if panel == 8 else 'pod'
    extra = ',reason="正常稼働"' if panel == 5 else ',alertname="発火なし"' if panel == 8 else ''
    base = [health] + ([counter] if panel == 7 else [rule_group] if panel == 8 else inventory)
    case(f'{panel}: healthy empty is explicit', panel, base,
         '{namespace="—",' + target + '="' + ('異常なし' if panel == 5 else '対象なし') + '"' + extra + '}', 0)
    extra_unknown = ',reason="取得不能"' if panel == 5 else ',alertname="取得不能"' if panel == 8 else ''
    unknown = '{namespace="—",' + target + '="取得不能"' + extra_unknown + '}'
    case(f'{panel}: failed monitoring is not empty', panel,
         [{'series': 'cluster_watch:ksm_healthy:bool', 'values': '0+0x10'}], unknown, -1)
    case(f'{panel}: missing monitoring is not empty', panel, [], unknown, -1)
    case(f'{panel}: stale monitoring is not empty', panel,
         [{'series': 'cluster_watch:ksm_healthy:bool', 'values': '1 _ _ _'}], unknown, -1, '3m')

case('issues remain visible without an empty sentinel', 5,
     [health, *inventory, {'series': 'cluster_watch:pod_issue:present{namespace="example",pod="app",reason="CrashLoopBackOff"}', 'values': '1+0x10'}],
     '{__name__="cluster_watch:pod_issue:present",namespace="example",pod="app",reason="CrashLoopBackOff"}', 1)
case('restart metric missing is not a healthy zero', 7, [health], '{namespace="—",pod="取得不能"}', -1)
case('rule observer missing is not no alerts', 8, [health], '{namespace="—",target="取得不能",alertname="取得不能"}', -1)

for panel, expected in [(11,1),(12,0),(13,0),(14,0),(15,0),(16,0)]:
    case(f'{panel}: healthy Pod distribution',panel,[health,*inventory],'{}',expected)
    case(f'{panel}: missing Pod data is not zero',panel,[health],'{}',-1)
    case(f'{panel}: monitoring failure is not zero',panel,[],'{}',-1)
# Failed Jobs remain visible in the issue list, rather than being hidden by owner kind.
failed = [health,
 {'series':'kube_pod_info{namespace="example",pod="job",uid="j"}', 'values':'1+0x10'},
 {'series':'kube_pod_status_ready{namespace="example",pod="job",uid="j",condition="true"}', 'values':'0+0x10'},
 {'series':'kube_pod_status_phase{namespace="example",pod="job",uid="j",phase="Failed"}', 'values':'1+0x10'},
 {'series':'kube_pod_owner{namespace="example",pod="job",uid="j",owner_kind="Job"}', 'values':'1+0x10'},
]
case('failed Job is included in phase distribution',15,failed,'{}',1)
case('failed Job is included in issue details',5,failed,'{namespace="example",pod="job",uid="j",reason="Failed"}',1)
for panel in (17,18,19,20):
    case(f'{panel}: absent capacity/alert telemetry is not healthy',panel,[],'{}',-1)

# Rule groups legitimately evaluated every 180s must not flap at a fixed 120s age.
slow_rule = [health,
 {'series':'prometheus_rule_group_last_evaluation_timestamp_seconds{rule_group="slow"}', 'values':'0+42x10'},
 {'series':'prometheus_rule_group_interval_seconds{rule_group="slow"}', 'values':'180+0x10'},
]
case('slow rule group remains observable at 180 seconds old',20,slow_rule,'{}',0)
case('slow rule group still reports real alerts',20,[*slow_rule,{'series':'ALERTS{alertstate="firing",severity="warning",alertname="TargetDown"}','values':'1+0x10'}],'{}',1)
for phase, reason in [('Pending','Pending'),('Unknown','Unknown'),('Running','NotReady')]:
    data = [health,*[dict(item) for item in inventory]]
    data[2]['values']='0+0x10'
    data[3]['series']=data[3]['series'].replace('Running',phase)
    case(f'immediate {reason} is not labelled healthy',5,data,'{namespace="example",pod="app",uid="a",reason="'+reason+'"}',1)
node = [health,{'series':'kube_node_info{node="one"}','values':'1+0x10'}, {'series':'up{job="node-exporter",instance="one"}','values':'1+0x10'}]
case('capacity weighted CPU value remains numeric',17,[*node,{'series':'node_cpu_seconds_total{mode="idle",instance="one",cpu="0"}','values':'0+30x10'}],'{}',0.5)
case('capacity weighted memory value remains numeric',18,[*node,{'series':'node_memory_MemAvailable_bytes{instance="one"}','values':'40+0x10'},{'series':'node_memory_MemTotal_bytes{instance="one"}','values':'100+0x10'}],'{}',0.6)
pvc = [*node,
 {'series':'up{job="kubelet",metrics_path="/metrics",instance="one"}','values':'1+0x10'},
 {'series':'kubelet_volume_stats_used_bytes{namespace="example",persistentvolumeclaim="data"}','values':'70+0x10'},
 {'series':'kubelet_volume_stats_capacity_bytes{namespace="example",persistentvolumeclaim="data"}','values':'100+0x10'},
 {'series':'kube_persistentvolumeclaim_info{namespace="example",persistentvolumeclaim="data"}','values':'1+0x10'},
 {'series':'kube_pod_spec_volumes_persistentvolumeclaims_info{namespace="example",persistentvolumeclaim="data",pod="app",uid="a"}','values':'1+0x10'},inventory[2]]
case('worst PVC usage is numeric',19,pvc,'{}',0.7)
case('zero PVC capacity is not a valid percentage',19,[{**x,'values':'0+0x10'} if 'capacity_bytes' in x['series'] else x for x in pvc],'{}',-1)

(repo / 'temp').mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix='cluster-watch-', dir=repo / 'temp') as folder:
    target = Path(folder)
    (target / 'tests.json').write_text(json.dumps({'rule_files': [], 'evaluation_interval': '1m', 'tests': tests}, ensure_ascii=False))
    subprocess.run(['docker', 'run', '--rm', '--network=none', '--entrypoint', 'promtool',
                    '-v', f'{target}:/tests:ro', 'prom/prometheus:v3.1.0', 'test', 'rules', '/tests/tests.json'], check=True)
print(f'{len(tests)} dashboard query scenarios passed')

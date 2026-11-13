"""Regression checks for the operational app table, including PromQL failure paths."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml
from apps import EXPR, application_panel

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


def info(name='invoice-api', health='Healthy', sync='Synced', project='app-project', instance='controller:8082'):
    return (f'argocd_app_info{{job="argocd-application-controller",namespace="argocd",'
            f'instance="{instance}",name="{name}",project="{project}",health_status="{health}",sync_status="{sync}"}}')


def up(instance='controller:8082'):
    return f'up{{job="argocd-application-controller",namespace="argocd",instance="{instance}"}}'


def series(metric, values='1+0x10'):
    return {'series': metric, 'values': values}


def expected(name='invoice-api', health='Healthy', sync='Synced', value=600000):
    return {'labels': '{' + ','.join(f'{k}="{v}"' for k,v in sorted({
        'name':name, 'health_status':health, 'sync_status':sync, 'route_status':'未計測',
        'application_url':'https://argocd.platform.yu-min3.com/applications/argocd/'+name}.items())) + '}', 'value':value}


MISSING = {'labels': '{application_url="https://argocd.platform.yu-min3.com/applications",health_status="未取得",name="アプリ情報を取得できません",route_status="未計測",sync_status="未取得"}', 'value': -1}


def cases():
    tests = []
    def case(name, samples, result, when='10m'):
        tests.append({'name':name, 'interval':'1m', 'input_series':samples,
                      'promql_expr_test':[{'expr':EXPR,'eval_time':when,'exp_samples':result}]})
    case('only application project; live names and separate Sync/Health', [series(up()), series(info()),
         series(info('billing-worker','Degraded')), series(info('search-api','Healthy','OutOfSync')),
         series(info('control-plane','Healthy','Synced','platform-project'))],
         [expected(),expected('billing-worker','Degraded'),expected('search-api','Healthy','OutOfSync')])
    for health in ['Progressing','Suspended','Missing','Unknown']:
        case('preserve '+health,[series(up()),series(info(health=health))],[expected(health=health)])
    case('preserve unknown sync',[series(up()),series(info(sync='Unknown'))],[expected(sync='Unknown')])
    case('no collector', [], [MISSING])
    case('scrape down suppresses recent healthy data', [series(up(),'0+0x10'),series(info())], [MISSING])
    case('missing up suppresses healthy data', [series(info())], [MISSING])
    case('missing app info is not healthy or empty', [series(up())], [MISSING])
    case('stale app info is hidden at 120s', [series(up()),series(info(),'1 _ _')],[MISSING],'2m')
    case('fresh sample keeps its actual time', [series(up()),series(info(),'1 _')],[expected(value=0)],'1m')
    case('stale up is not healthy', [series(up(),'1 _ _'),series(info())],[MISSING],'2m')
    case('any failed controller makes collection incomplete', [series(up()),series(up('other:8082'),'0+0x10'),series(info())],[MISSING])
    case('identical HA samples collapse', [series(up()),series(up('other:8082')),series(info()),series(info(instance='other:8082'))],[expected()])
    case('status transition does not preserve old healthy label', [series(up()),series(info(),'1+0x8 stale'),series(info(health='Degraded'),'_ _ _ _ _ _ _ _ _ 1 1')],[expected(health='Degraded')])
    return tests


class AppPanelTest(unittest.TestCase):
    def test_real_panel_has_no_sample_or_invented_route_health(self):
        p=application_panel()
        encoded=json.dumps(p,ensure_ascii=False)
        self.assertEqual(p['datasource']['type'],'prometheus')
        for banned in ('csvContent','app-status-sample','/d/app-sample-','アプリA'):
            self.assertNotIn(banned,encoded)
        self.assertIn('未計測',encoded)
        self.assertIn('app-project',p['description'])
        self.assertEqual(p['targets'][0]['format'],'table')

    def test_packaged_home_is_live_and_scenarios_stay_explicit_samples(self):
        subprocess.run(['python3',str(HERE/'build.py')],check=True,capture_output=True)
        resources=ROOT/'kubernetes/observability/grafana/resources'
        cm=yaml.safe_load((resources/'cluster-watch-dashboard.yaml').read_text())
        home=json.loads(cm['data']['cluster-watch.json'])
        p=next(p for p in home['panels'] if p['id']==8)
        self.assertEqual(p['title'],'アプリの状態')
        self.assertIn('argocd_app_info',p['targets'][0]['expr'])
        self.assertNotIn('grafana-testdata-datasource',json.dumps(home))
        samples=yaml.safe_load((resources/'cluster-watch-samples-dashboard.yaml').read_text())
        example=json.loads(samples['data']['cw-function-normal.json'])
        self.assertIn('sample',example['tags'])
        self.assertIn('csvContent',json.dumps(example))

    def test_promql(self):
        with tempfile.TemporaryDirectory(dir=ROOT/'temp') as temp:
            path=Path(temp)/'cases.yaml'
            tests=cases()
            path.write_text(yaml.safe_dump({'rule_files':[], 'evaluation_interval':'1m','tests':tests},allow_unicode=True))
            subprocess.run(['docker','run','--rm','-v',f'{path.parent}:/work:ro',
                            '--entrypoint','promtool','prom/prometheus:v3.1.0','test','rules','/work/cases.yaml'],check=True)
            print(f'{len(tests)} app state / collection cases passed')


if __name__=='__main__':
    (ROOT/'temp').mkdir(exist_ok=True)
    unittest.main()

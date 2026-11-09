#!/usr/bin/env python3
"""Generate production Cluster Watch ConfigMaps from the validated native Grafana design."""
import copy
import json
from pathlib import Path
from common import (DS, NODES, KSM, MAPPINGS, fresh, guarded, node_ready, resource,
                   pods_ready, deployment_ready, daemonset_ready, panel, target)

HERE = Path(__file__).resolve().parent
UIDS = {'node':'200ac8fdbfbb74b39aff88118e4d1c2c', 'namespace':'85a562078cdf77779eaa1add43ccec1e',
        'workload':'a164a7f0339f99e89cea5cb47e9be617', 'pod':'6581e46e4e5c7ba40a07646395ef7b23'}
HOME = 'cluster-watch'
# Freeze the selected window when drilling down, including a relative "now" window.
CONTEXT = 'var-datasource=prometheus&var-cluster=&from=${__from}&to=${__to}'


def url(kind, params=''):
    return f'/d/{UIDS.get(kind, kind)}/details?{CONTEXT}' + ('&'+params if params else '')


def link(title, destination):
    return {'title':title, 'url':destination, 'targetBlank':False}


def defaults(p, **values):
    p['fieldConfig']['defaults'].update(values)
    return p


def text(title, content, x,y,w,h):
    p=panel('text',title,x,y,w,h)
    p['options']={'mode':'markdown','content':content}
    return p


def stat(title, expr, x,y,w,h=3, state=False, destination=None, unit='short'):
    p=panel('stat',title,x,y,w,h,[target(expr,title,'A')])
    defaults(p, unit=unit, decimals=0, mappings=copy.deepcopy(MAPPINGS) if state else [
        {'type':'value','options':{'-1':{'text':'? 観測不明','color':'gray'}}}],
        thresholds={'mode':'absolute','steps':[{'color':'gray','value':None},{'color':'green','value':0},{'color':'orange','value':1}]})
    p['options']={'colorMode':'value','graphMode':'none','textMode':'value','justifyMode':'center',
                  'reduceOptions':{'calcs':['last'],'values':False}}
    if state:p['options']['text']={'valueSize':22}
    if destination: defaults(p,links=[link('詳細を開く',destination)])
    return p


def table(title, expr, x,y,w,h, names=None, links=None, unit='short'):
    t=target(expr,'','A');t['format']='table'
    p=panel('table',title,x,y,w,h,[t])
    defaults(p,unit=unit,decimals=2 if unit=='short' else 0,
             color={'mode':'fixed','fixedColor':'text'},custom={'align':'auto','cellOptions':{'type':'auto'},'filterable':True})
    p['options']={'showHeader':True,'cellHeight':'sm','footer':{'show':False},'sortBy':[{'displayName':(names or {}).get('Value','Value'),'desc':True}]}
    p['transformations']=[{'id':'organize','options':{'excludeByName':{'Time':True,'__name__':True},'renameByName':names or {}}}]
    for field,destination in (links or {}).items():
        p['fieldConfig']['overrides'].append({'matcher':{'id':'byName','options':field},'properties':[
            {'id':'links','value':[link('対象の詳細へ',destination)]}]})
    return p


def dashboard(uid,title,panels):
    for i,p in enumerate(panels,1):p['id']=i
    return {'uid':uid,'id':None,'title':title,'schemaVersion':41,'version':1,'editable':False,
            'timezone':'Asia/Tokyo','refresh':'1m','time':{'from':'now-6h','to':'now'},'tags':['cluster-watch','local-prototype'],
            'panels':panels,'templating':{'list':[]},'annotations':{'list':[]},'graphTooltip':1,
            'links':[{'type':'link','title':'← Cluster Watch','url':url(HOME),'keepTime':False}] if uid!=HOME else []}


REMAP = {value:'cw-detail-'+key for key,value in UIDS.items()}
OUTPUT = HERE.parents[1]/'kubernetes/observability/grafana/resources'

def production(value):
    if isinstance(value,dict):return {k:production(v) for k,v in value.items()}
    if isinstance(value,list):return [production(v) for v in value]
    if isinstance(value,str):
        for old,new in REMAP.items():value=value.replace(old,new)
        return value.replace('cluster-watch-practical','cluster-watch')
    return value

def save(d):
    d=production(d)
    d['tags']=['platform','watch']
    if d['uid']=='cluster-watch':d['title']='Cluster Watch'
    if d['uid'].startswith('cw-detail-'):d['title']='Cluster Watch / '+d['title']
    name='grafana-cluster-watch-dashboard' if d['uid']=='cluster-watch' else 'grafana-'+d['uid']+'-dashboard'
    filename='cluster-watch-dashboard.yaml' if d['uid']=='cluster-watch' else d['uid']+'-dashboard.yaml'
    data=json.dumps(d,ensure_ascii=False,indent=2)
    content='apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: '+name+'\n  namespace: monitoring\n  labels:\n    grafana_dashboard: "1"\n    grafana_folder: "Platform"\ndata:\n  '+d['uid']+'.json: |\n'
    content+=''.join('    '+line+'\n' for line in data.splitlines())
    (OUTPUT/filename).write_text(content)


def vector_guard(expr, metrics):
    """Retain labels, but suppress current rows on incomplete/stale evidence."""
    checks=[KSM]
    for metric in metrics:
        checks += [f'count({metric}) > 0', fresh(metric)]
    return '('+expr+') and on() ('+') and on() ('.join(checks)+')'


KINDS=[('deployment','kube_deployment_spec_replicas','kube_deployment_status_replicas_available','deployment'),
       ('statefulset','kube_statefulset_replicas','kube_statefulset_status_replicas_ready','statefulset'),
       ('daemonset','kube_daemonset_status_desired_number_scheduled','kube_daemonset_status_number_ready','daemonset')]


def workload_deficits():
    parts=[]
    for kind,desired,actual,label in KINDS:
        # Missing per-workload replica telemetry remains -1, never silently disappears as healthy.
        valid=f'(({desired} - on(namespace,{label}) {actual}) and on(namespace,{label}) (time() - timestamp({desired}) < 120) and on(namespace,{label}) (time() - timestamp({actual}) < 120))'
        inventory=f'({desired} or on(namespace,{label}) {actual})'
        deficit=f'(clamp_min({valid},0) or on(namespace,{label}) ({inventory} * 0 - 1))'
        norm=f'label_replace(label_replace({deficit},"workload","$1","{label}","(.*)"),"kind","{kind}","",".*")'
        parts.append(f'max by(namespace,workload,kind) ({norm})')
    return '('+' or '.join(parts)+f') and on() ({KSM})'


DEFICITS=workload_deficits()
PENDING=vector_guard('max by(namespace,pod) (kube_pod_status_phase{phase="Pending"})',['kube_pod_status_phase{phase="Pending"}'])
RESTARTS=vector_guard('sum by(namespace,pod) (increase(kube_pod_container_status_restarts_total[1h]))',['kube_pod_container_status_restarts_total'])
CAPS=[
 ('control','API / etcd',pods_ready('kube-apiserver-master|etcd-master',2),'kube-system','apiserver',1,'API操作・etcd書込みの機能試験は未計測。etcd専用メトリクスの収集も未確認。'),
 ('scheduler','再配置の基盤',pods_ready('kube-controller-manager-master|kube-scheduler-master',2),'kube-system','kube-controller-manager|kube-scheduler',2,'Pod Readyのみ。新しいPodの配置・回復を実証する試験は未計測。'),
 ('dns','CoreDNS',deployment_ready('kube-system','coredns',1),'kube-system','coredns',2,'DNS名前解決と期待応答のprobeは未計測。'),
 ('network','Cilium',daemonset_ready('kube-system','cilium',1),'kube-system',None,None,'ノード間・Service経由の通信probeは未計測。'),
 ('mesh','Istio',deployment_ready('istio-system','istiod|gateway-platform-istio|gateway-prod-istio',3),'istio-system',None,None,'xDS同期・mesh通信の機能試験は未計測。'),
 ('storage','Storage',daemonset_ready('longhorn-system','longhorn-manager|longhorn-csi-plugin',2),'longhorn-system','longhorn-backend',3,'manager/CSIの稼働確認。Volumeの冗長性や復元の実証ではない。')]


def timeline(title, items,x,y,w,h):
    p=panel('state-timeline',title,x,y,w,h,[target(expr,name,chr(65+i),False) for i,(name,expr) in enumerate(items)])
    defaults(p,mappings=copy.deepcopy(MAPPINGS),color={'mode':'fixed','fixedColor':'green'},
             custom={'lineWidth':0,'fillOpacity':55,'spanNulls':False})
    p['options']={'mergeValues':True,'rowHeight':.7,'showValue':'never','alignValue':'left',
                  'legend':{'displayMode':'list','placement':'bottom','showLegend':False},'tooltip':{'mode':'single','sort':'none'}}
    return p


def build_overview():
    p=[text('', '## CLUSTER WATCH\n実測の概要 → 対象 → 根拠。ノードの状態・負荷、表の名前を押すと詳細へ。',0,0,14,3)]
    age=guarded('time() - min(timestamp(kube_node_status_condition{condition="Ready",status="true"}))',ksm=True)
    p += [stat('KSMデータの経過時間',age,14,0,5,unit='s'),
          stat('観測 · KSM',guarded('min(up{job="kube-state-metrics"})',[fresh('up{job="kube-state-metrics"}')],ksm=False),19,0,5,state=True,destination=url('cw-evidence-scheduler'))]
    defaults(p[1],thresholds={'mode':'absolute','steps':[{'color':'gray','value':None},{'color':'text','value':0},{'color':'orange','value':90}]})
    defaults(p[2],mappings=[{'type':'value','options':{'-1':{'text':'? 観測不明','color':'gray'},'0':{'text':'▲ 収集失敗','color':'orange'},'1':{'text':'取得中','color':'green'}}}])
    # Four aligned node columns; standard Grafana panels, no Canvas dependency.
    for i,node in enumerate(NODES):
        destination=url('node',f'var-node={node}')
        ready=stat(node+' · Ready →',node_ready(node),i*6,3,6,3,True,destination)
        ready['description']='ReadyはKSM。負荷はnode-exporter。クリックで同じ時間のNode詳細へ。'
        p.append(ready)
        gauge=panel('bargauge',node+' · 使用率',i*6,6,6,4,[target(resource(node),'CPU','A'),target(resource(node,True),'Memory','B')])
        defaults(gauge,unit='percent',min=0,max=100,decimals=0,links=[link('Node詳細へ',destination)],
                 mappings=[{'type':'value','options':{'-1':{'text':'? 観測不明','color':'gray'}}}])
        gauge['options']={'orientation':'horizontal','displayMode':'basic','showUnfilled':True,
                          'reduceOptions':{'calcs':['last'],'values':False},'valueMode':'color','namePlacement':'left'}
        p.append(gauge)
        trend=panel('timeseries',node+' · CPU / Memory推移',i*6,10,6,4,[
            target(f'({resource(node)}) >= 0','CPU','A',False),target(f'({resource(node,True)}) >= 0','Memory','B',False)])
        defaults(trend,unit='percent',min=0,max=100,color={'mode':'palette-classic'},links=[link('Node詳細へ',destination)],
                 custom={'drawStyle':'line','lineWidth':1,'fillOpacity':8,'spanNulls':False,'showPoints':'never','axisLabel':''})
        trend['options']={'legend':{'displayMode':'list','placement':'bottom'},'tooltip':{'mode':'multi'}}
        p.append(trend)
    for i,(slug,name,expr,*_) in enumerate(CAPS):
        card=stat(name+' · 稼働 →',expr,i*4,14,4,3,True,url('cw-evidence-'+slug))
        card['description']='機能成功とは別。クリックすると稼働・観測・未計測の根拠を表示。'
        p.append(card)
    # Summary counts stay unknown when telemetry is absent; zero is an observed zero.
    deficit_count=guarded(f'count(({DEFICITS}) > 0) or vector(0)',[f'count({DEFICITS}) > 0',f'min({DEFICITS}) >= 0'])
    pending_count=guarded(f'sum({PENDING})',[f'count({PENDING}) > 0'])
    restart_count=guarded(f'sum({RESTARTS})',[f'count({RESTARTS}) > 0'])
    p += [stat('Replica不足 · Workload数',deficit_count,0,17,8),stat('Pending · Pod数',pending_count,8,17,8),stat('再起動増加 · 直近1時間',restart_count,16,17,8)]
    # A display-only sentinel puts missing evidence first; never counted as a real deficit.
    rank=f'(({DEFICITS}) >= 0) or ((({DEFICITS}) < 0) * -1000000)'
    wt=table('Workloads · Replica不足 / 未取得を優先',rank,0,20,14,7,
       {'namespace':'Namespace','workload':'Workload','kind':'Kind','Value':'不足'},
       {'Workload':url('workload','var-namespace=${__data.fields.Namespace}&var-type=${__data.fields.Kind}&var-workload=${__data.fields.Workload}'),
        'Namespace':url('namespace','var-namespace=${__data.fields.Namespace}')})
    defaults(wt,decimals=0,mappings=[{'type':'value','options':{'1000000':{'text':'? 未取得','color':'gray'},'0':{'text':'充足','color':'green'}}}])
    wt['options']['sortBy']=[{'displayName':'不足','desc':True}]
    wt['transformations'][0]['options']['indexByName']={'namespace':0,'workload':1,'kind':2,'Value':3}
    for field,width in [('Namespace',140),('Workload',320),('Kind',105),('不足',85)]:
        wt['fieldConfig']['overrides'].append({'matcher':{'id':'byName','options':field},'properties':[{'id':'custom.width','value':width}]})
    wt['description']='Deployment/StatefulSet/DaemonSetの期待数と稼働数。列のフィルタで絞込み可能。KSM停止時は行を表示しない。未取得は表示用の優先値に変換して先頭へ。'
    p.append(wt)
    owners='namespace_workload_pod:kube_pod_owner:relabel{workload_type=~"deployment|statefulset|daemonset"}'
    phase=vector_guard(f'max by(namespace,pod,phase) (kube_pod_status_phase == 1) and on(namespace,pod) {owners}',['kube_pod_status_phase',owners])
    pt=table('常駐Pods · 現在の状態',phase,14,20,10,7,
             {'namespace':'Namespace','pod':'Pod','phase':'Phase'},
             {'Pod':url('pod','var-namespace=${__data.fields.Namespace}&var-pod=${__data.fields.Pod}')})
    pt['transformations'][0]['options']['excludeByName']['Value']=True
    pt['transformations'][0]['options']['indexByName']={'namespace':0,'pod':1,'phase':2}
    pt['options']['sortBy']=[{'displayName':'Phase','desc':False}]
    for field,width in [('Namespace',130),('Pod',280),('Phase',80)]:
        pt['fieldConfig']['overrides'].append({'matcher':{'id':'byName','options':field},'properties':[{'id':'custom.width','value':width}]})
    pt['description']='Deployment/StatefulSet/DaemonSet配下のPod。終了済みJobは含めません。RunningはReadyや再起動なしの保証ではありません。クリックで待機理由・再起動・メトリクスを確認。列フィルタで絞込み可能。'
    p.append(pt)
    p.append(timeline('基盤の稼働履歴 · 選択した時間範囲',[(name,expr) for _,name,expr,*_ in CAPS],0,27,16,7))
    p.append(text('回復の確認','### 機能probe・回復試験：未計測\n\n稼働カードの緑は **Ready / Replicaの確認**。\n\n回復実証には canaryの新UID → Ready → 期待応答が必要。\n\n[確認の根拠を見る]('+url('cw-evidence-scheduler')+')',16,27,8,7))
    changes='max by(node) (changes(kube_node_status_condition{condition="Ready",status="true"}[$__range]))'
    p.append(table('Node Readyの変化回数 · 選択範囲',vector_guard(changes,['kube_node_status_condition{condition="Ready",status="true"}']),0,34,10,5,{'node':'Node','Value':'変化回数'},{'Node':url('node','var-node=${__data.fields.Node}')}))
    p.append(text('この画面で分かる範囲','**ログ・Events：収集範囲を未確認**。Pod詳細ではメトリクスと現在の状態を確認できます。\n\n履歴の空白は観測不明です。更新エラー時は古い値が残ることがあるため、最上段の取得時間とパネルエラーを確認してください。\n\n単一control plane。StorageカードはVolume冗長性を保証しません。',10,34,14,5))
    d=dashboard(HOME,'Cluster Watch · Overview',p)
    d['links']=[{'type':'link','title':'従来のCluster Watch','url':url('cluster-watch-classic')}]
    save(d)


def build_evidence():
    for slug,name,expr,namespace,job,count,note in CAPS:
        p=[text('',f'## {name} — 確認の根拠\n稼働状態・機能確認・回復実証を分けて見る。',0,0,24,3),
           stat('稼働 · Ready / Replica',expr,0,3,8,4,True),
           text('機能確認','### ? 未計測\n'+note,8,3,8,4),
           text('回復実証','### ? 未計測\n故障注入・新UID生成・応答復帰の試験は未導入。',16,3,8,4)]
        p.append(timeline('稼働履歴',[(name,expr)],0,7,24,5))
        if job:
            s=f'up{{job=~"{job}"}}'
            scrape=guarded(f'min({s})',[f'count({s}) == {count}',fresh(s)],ksm=False)
            title='APIの収集 · etcdは未確認' if slug=='control' else 'メトリクス収集 · 稼働状態とは別'
            sp=stat(title,scrape,0,12,8,4,True)
            defaults(sp,mappings=[{'type':'value','options':{'-1':{'text':'? 対象未確認','color':'gray'},'0':{'text':'▲ 収集失敗','color':'orange'},'1':{'text':'取得中','color':'green'}}}])
            p.append(sp)
        else:p.append(text('専用メトリクス','### ? 収集範囲を未確認\nこの試作ではKSMの稼働情報を利用。',0,12,8,4))
        p.append(text('根拠の読み方','KSMのReady・期待Replicaと稼働数を比較。欠測・古いサンプルは不明。\n\n**scrape失敗は機能停止と同義ではありません。**',8,12,16,4))
        pods=f'max by(namespace,pod) (kube_pod_status_ready{{namespace="{namespace}",condition="true"}})'
        p.append(table(namespace+' · Pod Ready（クリックで詳細）',vector_guard(pods,[f'kube_pod_status_ready{{namespace="{namespace}",condition="true"}}']),0,16,24,8,
                 {'namespace':'Namespace','pod':'Pod','Value':'Ready'},{'Pod':url('pod','var-namespace=${__data.fields.Namespace}&var-pod=${__data.fields.Pod}')}))
        d=dashboard('cw-evidence-'+slug,'Cluster Watch · '+name+' Evidence',p)
        d['links'].append({'type':'link','title':namespace+'の詳細','url':url('namespace','var-namespace='+namespace)})
        save(d)


def import_details():
    """Adapt the four existing mixin dashboards without modifying production."""
    def visit(x):
        if isinstance(x,dict):
            if 'url' in x and isinstance(x['url'],str) and x['url'].startswith('/d/'):
                x['url'] += '&from=${__from}&to=${__to}' if '?' in x['url'] else '?from=${__from}&to=${__to}'
            for v in x.values():visit(v)
        elif isinstance(x,list):
            for v in x:visit(v)
    for source in (HERE/'detail-sources').glob('*.json'):
        d=json.loads(source.read_text());visit(d)
        d['id']=None;d['editable']=False;d['timezone']='Asia/Tokyo';d['refresh']='1m'
        d['links']=[{'type':'link','title':'← Cluster Watch','url':url(HOME)}]+d.get('links',[])
        for v in d['templating']['list']:
            if v['name'] in ('datasource','cluster'):
                value='prometheus' if v['name']=='datasource' else ''
                v.update(type='constant',query=value,current={'text':value,'value':value},hide=2,options=[{'text':value,'value':value,'selected':True}])
        if source.stem=='k8s-resources-node':
            # Original by(pod) tables lose namespace. Replace with namespace-safe usage lists.
            for i,p in enumerate(d['panels']):
                if p['type']!='table':continue
                mem='Memory' in p['title']
                metric='node_namespace_pod_container:container_memory_working_set_bytes' if mem else 'node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m'
                expr=f'sum by(namespace,pod) ({metric}{{node=~"$node",container!=""}})'
                pos=p['gridPos']
                q=table('Memory by Pod' if mem else 'CPU by Pod',expr,pos['x'],pos['y'],pos['w'],pos['h'],
                    {'namespace':'Namespace','pod':'Pod','Value':'Memory' if mem else 'CPU'},
                    {'Pod':url('pod','var-namespace=${__data.fields.Namespace}&var-pod=${__data.fields.Pod}')},unit='bytes' if mem else 'short')
                q['id']=p['id'];d['panels'][i]=q
        if source.stem=='k8s-resources-pod':
            # Actual state complements the inherited compute metrics; no fictitious log links.
            for p in d['panels']:p['gridPos']['y']+=10
            s='kube_pod_status_ready{namespace="$namespace",pod="$pod",condition="true"}'
            current=stat('Pod Ready',guarded(f'max({s})',[f'count({s}) == 1',fresh(s)]),0,0,6,3,True);current['id']=901
            rest=stat('再起動増加 · 直近1時間',guarded('sum(increase(kube_pod_container_status_restarts_total{namespace="$namespace",pod="$pod"}[1h]))'),6,0,6,3);rest['id']=902
            note=text('調査の範囲','ログ・Eventsはこの試作では未接続。以下はPrometheusにある状態・メトリクス。',12,0,12,3);note['id']=903
            reason=table('Containerの待機理由（該当なしなら行なし）','kube_pod_container_status_waiting_reason{namespace="$namespace",pod="$pod"} == 1',0,3,24,6);reason['id']=904
            d['panels']=[current,rest,note,reason]+d['panels']
        save(d)


if __name__=='__main__':
    save(json.loads((HERE/'classic.json').read_text()));import_details();build_overview();build_evidence()

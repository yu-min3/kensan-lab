#!/bin/sh
# Build 20 synthetic cases using real Iceberg, pin A/B, and verify fixed-release read and denials.
set -eu
umask 077
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../../.." && pwd)
cd "$root"
mkdir -p temp/llmops-dataset-demo
(cd apps/llmops && go build -o ../../temp/llmops-dataset-demo/worker ./cmd/llmops-worker && go build -o ../../temp/llmops-dataset-demo/seed ./tests/integration/dataset/seed)
(cd apps/konro/backend && go build -o ../../../temp/llmops-dataset-demo/konro-adapter ./cmd/konro-llmops-adapter)
temp/llmops-dataset-demo/seed temp/llmops-dev/storage/api-konro.json > temp/llmops-dataset-demo/seed.json
python3 - <<'PY'
from pathlib import Path
import hashlib,json,os,secrets,subprocess
root=Path.cwd();base=root/'temp/llmops-dataset-demo';data=root/'temp/llmops-dev';binary=base/'konro-adapter';seed=json.loads((base/'seed.json').read_text())
config={'app':'konro','role':'transform','temp_root':str(base/'work'),'adapters':{'konro':{'binary':str(binary),'sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'domain_schema_version':'konro.recipe-import.v1'}},'storage':{'endpoint':'http://127.0.0.1:19300','bucket':'llmops-dev','region':'us-east-1','credential_file':str(data/'storage/transform-konro.json')},'catalog':{'uri':'http://127.0.0.1:19381/api/catalog','name':'konro','namespace':'curated','table':'recipes','token_url':'https://id.localhost:19443/realms/llmops/protocol/openid-connect/token','client_id':'catalog-konro-transform','client_secret_file':str(data/'clients/catalog-konro-transform.secret'),'ca_file':str(data/'ca.pem'),'python_binary':str(root/'temp/llmops-probe/venv/bin/python')}}
path=base/'config.json'
def save():path.write_text(json.dumps(config));os.chmod(path,0o600)
def execute(args,payload=None,success=True):
 result=subprocess.run([str(base/'worker'),*args,'-config',str(path)],input=json.dumps(payload) if payload is not None else '',text=True,capture_output=True)
 assert (result.returncode==0)==success,result.stderr
 return json.loads(result.stdout) if success else None
save();curated=execute(['curate','-adapter','konro','-transform-version','dataset-synthetic-v1'],seed['inputs'])
entries=[]
for n,record in enumerate(sorted(curated['records'],key=lambda r:r['title'])):
 entries.append({'record_id':record['record_id'],'case':{'schema_version':'1','app_id':'konro','case_id':f'case-{n:02d}','input':seed['raw_refs'][record['capture_id']],'expected_constraints':{'required_ingredients':[f'ingredient {n+1}'],'required_steps':['Slice.','Simmer.']},'annotation_version':'synthetic-v1','annotation_status':'synthetic','split':'development' if n<15 else 'holdout','source_group':'fixture-case-'+f'{n:02d}'}})
config['catalog'].update(namespace='datasets',table='import_cases');config['dataset_source']={'namespace':'curated','table':'recipes'};save()
release='demo-'+secrets.token_hex(6)
spec={'release_id':release+'-a','task_id':'recipe-import','source_snapshot':curated['snapshot_id'],'code_version':'fixture-code-v1','adapter_version':'konro.recipe-import.v1@'+config['adapters']['konro']['sha256'],'entries':entries}
a=execute(['dataset','build'],spec);first=execute(['dataset','read','-release',a['release_id']]);assert len(first)==20
assert {x['case']['annotation_status'] for x in first}=={'synthetic'}
assert {x['case']['source_group'] for x in first if x['case']['split']=='development'}.isdisjoint({x['case']['source_group'] for x in first if x['case']['split']=='holdout'})
b=json.loads(json.dumps(spec));b.update(release_id=release+'-b',code_version='fixture-code-v2');b['entries'][0]['case']['expected_constraints']={'required_ingredients':['intentional alternate synthetic constraint']}
second=execute(['dataset','build'],b);assert a['tables'][1]['snapshot_id']!=second['tables'][1]['snapshot_id']
assert execute(['dataset','read','-release',a['release_id']])==first
assert execute(['dataset','build'],spec)==a
conflict=json.loads(json.dumps(spec));conflict['entries'][0]['case']['annotation_version']='changed';execute(['dataset','build'],conflict,False)
assert execute(['dataset','read','-release',a['release_id']])==first
execute(['dataset','read','-release','unpublished'],success=False)
# Eval uses separate secrets and readonly table/object policies.
config['role']='eval';config['storage']['credential_file']=str(data/'storage/eval-konro.json');config['catalog'].update(client_id='catalog-konro-eval',client_secret_file=str(data/'clients/catalog-konro-eval.secret'));save()
assert execute(['dataset','read','-release',a['release_id']])==first
execute(['dataset','build'],spec,False)
(base/'release-a.json').write_text(json.dumps(a));(base/'spec-a.json').write_text(json.dumps(spec))
print(json.dumps({'synthetic_cases':20,'source_group_split_leakage':False,'release_A_after_B_unchanged':True,'immutable_replay':True,'different_request_denied':True,'eval_read_success':True,'eval_build_denied':True,'snapshot_A':a['tables'][1]['snapshot_id'],'snapshot_B':second['tables'][1]['snapshot_id'],'retention_tags':[x['retention_tag'] for x in a['tables']]}))
PY
(cd apps/llmops && go build -o ../../temp/llmops-dataset-demo/check ./tests/integration/dataset/check)
temp/llmops-dataset-demo/check temp/llmops-dataset-demo/config.json temp/llmops-dataset-demo/spec-a.json
temp/llmops-probe/venv/bin/python apps/llmops/tests/integration/dataset/roles.py

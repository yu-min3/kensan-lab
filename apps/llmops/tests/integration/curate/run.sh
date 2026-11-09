#!/bin/sh
# Commit two public synthetic captures to real Iceberg using limited roles and verify lineage.
set -eu
umask 077
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../../.." && pwd)
cd "$root"
mkdir -p temp/llmops-curate-demo
(cd apps/llmops && go build -o ../../temp/llmops-curate-demo/worker ./cmd/llmops-worker && go build -o ../../temp/llmops-curate-demo/seed ./tests/integration/curate/seed)
(cd apps/konro/backend && go build -o ../../../temp/llmops-curate-demo/konro-adapter ./cmd/konro-llmops-adapter)
temp/llmops-curate-demo/seed temp/llmops-dev/storage/api-konro.json > temp/llmops-curate-demo/inputs.json
python3 - <<'PY'
from pathlib import Path
import hashlib,json,os,subprocess
root=Path.cwd();base=root/'temp/llmops-curate-demo';data=root/'temp/llmops-dev';binary=base/'konro-adapter'
config={'app':'konro','role':'transform','temp_root':str(base/'work'),'adapters':{'konro':{'binary':str(binary),'sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'domain_schema_version':'konro.recipe-import.v1'}},'storage':{'endpoint':'http://127.0.0.1:19300','bucket':'llmops-dev','region':'us-east-1','credential_file':str(data/'storage/transform-konro.json')},'catalog':{'uri':'http://127.0.0.1:19381/api/catalog','name':'konro','namespace':'curated','table':'recipes','token_url':'https://id.localhost:19443/realms/llmops/protocol/openid-connect/token','client_id':'catalog-konro-transform','client_secret_file':str(data/'clients/catalog-konro-transform.secret'),'ca_file':str(data/'ca.pem'),'python_binary':str(root/'temp/llmops-probe/venv/bin/python')}}
p=base/'config.json';p.write_text(json.dumps(config));os.chmod(p,0o600)
seed=json.loads((base/'inputs.json').read_text());inputs=json.dumps(seed['inputs']);command=[str(base/'worker'),'curate','-config',str(p),'-adapter','konro','-transform-version','konro-fixture-v2']
for _ in range(2):
 result=subprocess.run(command,input=inputs,text=True,capture_output=True)
 if result.returncode:raise RuntimeError(result.stderr.strip())
 result=json.loads(result.stdout);assert len(result['records'])==2 and result['snapshot_id']
 assert {r['title'] for r in result['records']}=={'Onion Soup','Tomato Soup'}
 assert {r['capture_id'] for r in result['records']}=={r['capture_id'] for r in json.loads(inputs)}
 assert all(r['raw_sha256']==seed['sha256_by_capture'][r['capture_id']] for r in result['records'])
print(json.dumps({'real_capture_to_iceberg':True,'records':2,'lineage_verified':True,'replay_without_duplicates':True,'snapshot_id':result['snapshot_id']}))
PY

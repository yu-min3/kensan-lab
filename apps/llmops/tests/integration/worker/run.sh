#!/bin/sh
# Build the shared worker and non-recipe adapter, then verify prepare/grade and invalid commands.
set -eu
umask 077
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../../.." && pwd)
cd "$root"
mkdir -p temp/llmops-worker-demo
(cd apps/llmops && go build -o ../../temp/llmops-worker-demo/worker ./cmd/llmops-worker && go build -o ../../temp/llmops-worker-demo/sample-adapter ./cmd/llmops-sample-adapter)
python3 - <<'PY'
from pathlib import Path
import hashlib,json,os,subprocess
base=Path('temp/llmops-worker-demo').resolve(); binary=base/'sample-adapter'
config={'app':'sample','role':'transform','temp_root':str(base/'work'),'timeout_millis':2000,'max_input_bytes':1048576,'max_output_bytes':1048576,'adapters':{'sample':{'binary':str(binary),'sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'domain_schema_version':'sample-text/1'}}}
path=base/'config.json';path.write_text(json.dumps(config));os.chmod(path,0o600)
def invoke(request):return subprocess.run([str(base/'worker'),'-config',str(path),'-adapter','sample'],input=json.dumps(request),text=True,capture_output=True)
request={'protocol_version':'1','request_id':'sample-1','operation':'prepare','domain_schema_version':'sample-text/1','payload':{'text':'hello lake'}}
r=invoke(request);assert r.returncode==0,r.stderr
assert json.loads(r.stdout)['result']['word_count']==2
request.update(operation='grade',payload={'text':'hello lake','expected_words':2});r=invoke(request);assert r.returncode==0 and json.loads(r.stdout)['result']['passed']
request['operation']='exec';assert invoke(request).returncode!=0
request['operation']='prepare';request['command']='touch forbidden';assert invoke(request).returncode!=0
print(json.dumps({'prepare_non_recipe':True,'grade_non_recipe':True,'exec_operation_rejected':True,'command_field_rejected':True}))
PY

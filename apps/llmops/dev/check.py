"""Assert local-service health and trusted HTTPS issuer from host and container."""
from pathlib import Path
import json,ssl,subprocess,time,urllib.request,urllib.error
root=Path(__file__).resolve().parents[3];data=root/'temp/llmops-dev'
issuer='https://id.localhost:19443/realms/llmops'
context=ssl.create_default_context(cafile=str(data/'ca.pem'))
url=issuer+'/.well-known/openid-configuration'
for _ in range(60):
 try:
  with urllib.request.urlopen(url,context=context,timeout=2) as response:discovery=json.load(response)
  assert discovery['issuer']==issuer
  break
 except (urllib.error.URLError,TimeoutError):time.sleep(1)
else:raise RuntimeError('Keycloak TLS readiness timed out')
try:urllib.request.urlopen(url,timeout=3)
except urllib.error.URLError:pass
else:raise AssertionError('private local CA unexpectedly trusted globally')
with urllib.request.urlopen(discovery['jwks_uri'],context=context,timeout=5) as response:
 assert json.load(response)['keys']
image='apache/polaris:1.8.0@sha256:347615e736cab2c9be0d8c464552ac930be4755302bb0aeb745216bbae99ba5b'
command=['docker','run','--rm','--network','llmops-dev_default','--mount',f'type=bind,src={data}/ca.pem,dst=/ca.pem,readonly','--entrypoint','curl',image,'--fail','--silent','--show-error','--connect-to','id.localhost:19443:keycloak:19443','--cacert','/ca.pem',url]
# curl special-cases *.localhost; connect-to selects Docker DNS without changing URL/SNI.
container=json.loads(subprocess.run(command,check=True,capture_output=True,text=True).stdout)
assert container['issuer']==issuer
for endpoint in ['http://127.0.0.1:19300/health','http://127.0.0.1:19382/q/health']:
 for _ in range(30):
  try:
   with urllib.request.urlopen(endpoint,timeout=2) as response:assert response.status==200
   break
  except urllib.error.URLError:time.sleep(1)
 else:raise RuntimeError('data service readiness timed out')
print(json.dumps({'host_trusted_https':True,'container_trusted_https':True,'same_issuer':issuer,'jwks_available':True,'untrusted_CA_rejected':True,'storage_and_catalog_healthy':True}))

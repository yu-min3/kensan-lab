import json
import subprocess
from pathlib import Path
import yaml

def render(*args):
    result=subprocess.run(['helm','template','review','chart','--namespace','app-llmops','-f','tests/values.json',*args],text=True,capture_output=True,check=True)
    return list(yaml.safe_load_all(result.stdout))
objects=render()
bykind={item['kind']:item for item in objects}
assert not {'Application','Secret','Namespace'} & bykind.keys()
pvc=bykind['PersistentVolumeClaim'];assert pvc['spec']['storageClassName']=='longhorn'
assert pvc['spec']['accessModes']==['ReadWriteOnce']
assert pvc['metadata']['annotations']['argocd.argoproj.io/sync-options']=='Prune=false'
d=bykind['Deployment'];assert d['spec']['replicas']==1 and d['spec']['strategy']['type']=='Recreate'
pod=d['spec']['template']['spec'];assert not pod['automountServiceAccountToken']
assert pod['securityContext']['runAsNonRoot']
for c in pod['containers']+pod['initContainers']:
    assert c['securityContext']['readOnlyRootFilesystem']
    assert not c['securityContext']['allowPrivilegeEscalation']
    assert c['securityContext']['capabilities']['drop']==['ALL']
    assert c['resources']['requests'] and c['resources']['limits']
volumes={v['name']:v for v in pod['volumes']};assert volumes['projected']['secret']['defaultMode']==0o440
assert volumes['private']['emptyDir']['medium']=='Memory'
assert 'projected' not in {v['name'] for v in pod['containers'][0]['volumeMounts']}
config=json.loads(bykind['ConfigMap']['data']['config.json']);assert config['tls_key_file']=='/run/private/tls.key'
assert config['state_directory']=='/data/jobs'
assert bykind['NetworkPolicy']['spec']['ingress']==[]
assert 'HTTPRoute' not in bykind and 'Job' not in bykind
# Enabling the route requires a separately supplied backend TLS policy and CA.
route=render('--set','route.enabled=true','--set','route.gatewayName=review-gateway','--set','route.gatewayNamespace=review-gateway','--set','route.hostname=llmops.example.invalid','--set','route.backendHostname=review-api.app-llmops.svc','--set','route.backendCAConfigMap=service-ca','--set','route.backendTLSPolicyAPIVersion=gateway.networking.k8s.io/v1alpha3')
assert any(v['kind']=='BackendTLSPolicy' for v in route)
worker=render('--set','worker.enabled=true','--set','worker.image.repository=example.invalid/llmops-worker','--set','worker.image.tag=review-only','--set','worker.existingSecret=example-transform-only','--set','worker.configMap=example-transform','--set','worker.inputConfigMap=example-input','--set-json','worker.args=["curate","-config","/etc/worker/config.json","-adapter","konro","-transform-version","v1"]')
job=next(v for v in worker if v['kind']=='Job');assert job['spec']['backoffLimit']==0
assert job['spec']['template']['spec']['volumes'][2]['secret']['secretName']=='example-transform-only'
# Deliberately invalid image/TLS inputs must fail rather than render insecurely.
for args in [['--set','image.tag=latest'],['--set','apiConfig.tls_key_file='],['--set','apiConfig.issuer=http://identity.example.invalid']]:
    result=subprocess.run(['helm','template','review','chart','-f','tests/values.json',*args],capture_output=True)
    assert result.returncode!=0,args
print('offline render checks passed: single writer, secret permissions, PVC protection, default isolation, opt-in route/worker, invalid config rejection')

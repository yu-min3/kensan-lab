#!/usr/bin/env python3
"""Create only Konro's service client; seal its secret without printing/persisting plaintext."""
import argparse,base64,json,subprocess,urllib.parse,urllib.request
from pathlib import Path

BASE='https://auth.yu-mins.com'
CLIENT='konro-agent-runtime'

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--kubeconfig',required=True);p.add_argument('--context',required=True)
 p.add_argument('--sealed-output',required=True);p.add_argument('--apply',action='store_true')
 a=p.parse_args();kube=['kubectl','--kubeconfig',a.kubeconfig,'--context',a.context]
 data=subprocess.run(kube+['-n','platform-auth-prod','get','secret','keycloak-secret','-o','json'],capture_output=True,check=True).stdout
 password=base64.b64decode(json.loads(data)['data']['KEYCLOAK_ADMIN_PASSWORD']).decode()
 opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
 def req(path,method='GET',body=None,token=None,form=False):
  payload=urllib.parse.urlencode(body).encode() if form else json.dumps(body).encode() if body is not None else None
  headers={'Authorization':'Bearer '+token} if token else {}
  if payload is not None:headers['Content-Type']='application/x-www-form-urlencoded' if form else 'application/json'
  r=urllib.request.Request(BASE+path,data=payload,headers=headers,method=method)
  with opener.open(r,timeout=20) as response:
   b=response.read(2<<20);return json.loads(b) if b else None
 token=req('/realms/master/protocol/openid-connect/token','POST',{'grant_type':'password','client_id':'admin-cli','username':'admin','password':password},form=True)['access_token']
 prefix='/admin/realms/kensan';clients=req(prefix+'/clients?clientId='+CLIENT,token=token)
 desired={'clientId':CLIENT,'protocol':'openid-connect','enabled':True,'publicClient':False,'serviceAccountsEnabled':True,'standardFlowEnabled':False,'implicitFlowEnabled':False,'directAccessGrantsEnabled':False,'fullScopeAllowed':False}
 if not clients:
  if not a.apply:print(json.dumps({'client':CLIENT,'exists':False,'apply_required':True}));return
  req(prefix+'/clients','POST',desired,token);clients=req(prefix+'/clients?clientId='+CLIENT,token=token)
 assert len(clients)==1
 client=clients[0];cid=client['id']
 for k,v in desired.items():assert client.get(k)==v,'existing service client differs'
 mappers=req(prefix+'/clients/'+cid+'/protocol-mappers/models',token=token)
 name='agent-runtime-audience';matching=[m for m in mappers if m['name']==name]
 config={'included.custom.audience':'agent-runtime','id.token.claim':'false','access.token.claim':'true'}
 if not matching:
  if not a.apply:print(json.dumps({'client':CLIENT,'audience_mapper_missing':True}));return
  req(prefix+'/clients/'+cid+'/protocol-mappers/models','POST',{'name':name,'protocol':'openid-connect','protocolMapper':'oidc-audience-mapper','config':config},token)
 else:
  assert len(matching)==1 and all(matching[0]['config'].get(k)==v for k,v in config.items())
 if a.apply:
  secret=req(prefix+'/clients/'+cid+'/client-secret',token=token)['value']
  raw={'apiVersion':'v1','kind':'Secret','metadata':{'name':'konro-agent-runtime','namespace':'app-konro'},'type':'Opaque','stringData':{'client-secret':secret}}
  sealed=subprocess.run(['kubeseal','--kubeconfig',a.kubeconfig,'--context',a.context,'--controller-name','sealed-secrets','--controller-namespace','kube-system','--format','yaml'],input=json.dumps(raw).encode(),capture_output=True,check=True).stdout
  path=Path(a.sealed_output);path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(sealed)
 print(json.dumps({'client':CLIENT,'audience':'agent-runtime','service_only':True,'sealed':a.apply,'credentials_printed':False}))
if __name__=='__main__':
 try:main()
 except Exception as e:
  print(json.dumps({'ok':False,'error_type':type(e).__name__,'credentials_printed':False}));raise SystemExit(1)

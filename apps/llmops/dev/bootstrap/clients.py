"""Idempotent local Keycloak service-client bootstrap; credentials never printed."""
from pathlib import Path
import argparse,base64,json,os,secrets
import requests
ROOT=Path(__file__).resolve().parents[4];DATA=ROOT/'temp/llmops-dev';ENV=dict(line.split('=',1) for line in (DATA/'dev.env').read_text().splitlines());BASE='https://id.localhost:19443';REALM=BASE+'/realms/llmops';CA=str(DATA/'ca.pem')
CLIENTS={
 'konro-app':('llmops-platform','konro','app'),
 'konro-transform':('llmops-platform','konro','transform'),
 'konro-eval':('llmops-platform','konro','eval'),
 'sample-app':('llmops-platform','sample','app'),
 'catalog-konro-transform':('polaris','konro','transform'),
 'catalog-konro-eval':('polaris','konro','eval'),
 'catalog-sample-app':('polaris','sample','app'),
 'runtime-konro-online':('agent-runtime','konro','online'),
 'runtime-konro-eval':('agent-runtime','konro','eval'),
 'runtime-sample-online':('agent-runtime','sample','online'),
 'runtime-expiry-probe':('agent-runtime','sample','expiry-test'),
 'runtime-denied-probe':('agent-runtime','sample','denied-test'),
}
def token(client,secret,realm=REALM,**extra):
 r=requests.post(realm+'/protocol/openid-connect/token',data={'grant_type':'client_credentials','client_id':client,'client_secret':secret,**extra},verify=CA,timeout=10)
 return r

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--rotate-client',choices=list(CLIENTS));args=parser.parse_args()
 admin=requests.post(BASE+'/realms/master/protocol/openid-connect/token',data={'grant_type':'password','client_id':'admin-cli','username':'local-admin','password':ENV['DEV_KEYCLOAK_ADMIN_SECRET']},verify=CA,timeout=10)
 if not admin.ok:raise RuntimeError('local administrator authentication failed')
 session=requests.Session();session.verify=CA;session.headers['Authorization']='Bearer '+admin.json()['access_token']
 endpoint=BASE+'/admin/realms/llmops/clients'
 directory=DATA/'clients';directory.mkdir(exist_ok=True);os.chmod(directory,0o700)
 for name,(aud,app,role) in CLIENTS.items():
  secretpath=directory/(name+'.secret')
  if not secretpath.exists():secretpath.write_text(secrets.token_hex(24));os.chmod(secretpath,0o600)
  secret=secretpath.read_text().strip()
  lookup=session.get(endpoint,params={'clientId':name},timeout=10);lookup.raise_for_status();matches=lookup.json()
  payload={'clientId':name,'enabled':True,'publicClient':False,'secret':secret,'serviceAccountsEnabled':True,'standardFlowEnabled':False,'directAccessGrantsEnabled':False,'protocol':'openid-connect','fullScopeAllowed':False}
  if name=='runtime-expiry-probe':payload['attributes']={'access.token.lifespan':'3'}
  if matches:
   uuid=matches[0]['id'];response=session.put(endpoint+'/'+uuid,json=payload,timeout=10)
  else:
   response=session.post(endpoint,json=payload,timeout=10);response.raise_for_status();uuid=session.get(endpoint,params={'clientId':name},timeout=10).json()[0]['id']
  response.raise_for_status()
  mapperurl=endpoint+'/'+uuid+'/protocol-mappers/models'
  existing={m['name']:m['id'] for m in session.get(mapperurl,timeout=10).json()}
  mappers=[{'name':'audience','protocol':'openid-connect','protocolMapper':'oidc-audience-mapper','config':{'included.custom.audience':aud,'access.token.claim':'true','id.token.claim':'false'}}]
  for claim,value in [('app_id',app),('llmops_role',role),('polaris_role',app+'_'+role)]:
   mappers.append({'name':claim,'protocol':'openid-connect','protocolMapper':'oidc-hardcoded-claim-mapper','config':{'claim.name':claim,'claim.value':value,'jsonType.label':'String','access.token.claim':'true','id.token.claim':'false'}})
  for mapper in mappers:
   if mapper['name'] in existing:result=session.put(mapperurl+'/'+existing[mapper['name']],json=mapper,timeout=10)
   else:result=session.post(mapperurl,json=mapper,timeout=10)
   result.raise_for_status()
  if args.rotate_client==name:
   old=secret;rotation=session.post(endpoint+'/'+uuid+'/client-secret',timeout=10);rotation.raise_for_status();secret=rotation.json()['value'];secretpath.write_text(secret);os.chmod(secretpath,0o600)
   assert token(name,old).status_code==401,'old credential still accepted'
  r=token(name,secret);assert r.ok,'service client authentication failed'
  claim=json.loads(base64.urlsafe_b64decode(r.json()['access_token'].split('.')[1]+'=='))
  assert claim['azp']==name and aud in ([claim['aud']] if isinstance(claim['aud'],str) else claim['aud']) and claim['iss']==REALM
 print(json.dumps({'clients':len(CLIENTS),'issuer':REALM,'audience_and_azp_verified':True,'credentials':'temp/llmops-dev/clients/*.secret','rotated_client':args.rotate_client}))
if __name__=='__main__':main()

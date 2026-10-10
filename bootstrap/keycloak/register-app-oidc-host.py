#!/usr/bin/env python3
"""Targeted operator: check owner metadata, optionally add one approved callback.

Reads the existing admin credential only in process memory. Does not print,
persist, or put credentials in argv. Does not run the realm-wide bootstrap.
Default is read-only; --apply appends the approved App callback and web origin only.
"""
import argparse,base64,json,re,subprocess,urllib.parse,urllib.request,uuid
BASE='https://auth.yu-mins.com'
CLIENT='istio-gateway-platform'

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--kubeconfig',required=True);p.add_argument('--context',required=True);p.add_argument('--host',required=True);p.add_argument('--apply',action='store_true');a=p.parse_args()
 assert re.fullmatch(r'[a-z0-9](?:[a-z0-9-]*[a-z0-9])?[.]app[.]yu-min3[.]com',a.host)
 HOST='https://'+a.host
 password=base64.b64decode(subprocess.run(['kubectl','--kubeconfig',a.kubeconfig,'--context',a.context,'-n','platform-auth-prod','get','secret','keycloak-secret','-o','jsonpath={.data.KEYCLOAK_ADMIN_PASSWORD}'],capture_output=True,check=True).stdout).decode()
 opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
 def request(path,method='GET',body=None,bearer=None,form=False):
  data=urllib.parse.urlencode(body).encode() if form else (json.dumps(body).encode() if body is not None else None)
  headers={}
  if bearer:headers['Authorization']='Bearer '+bearer
  if data is not None:headers['Content-Type']='application/x-www-form-urlencoded' if form else 'application/json'
  req=urllib.request.Request(BASE+path,data=data,method=method,headers=headers)
  with opener.open(req,timeout=20) as response:
   content=response.read(2<<20);return json.loads(content) if content else None
 token=request('/realms/master/protocol/openid-connect/token','POST',{'grant_type':'password','client_id':'admin-cli','username':'admin','password':password},form=True)['access_token']
 del password
 prefix='/admin/realms/kensan'
 users=request(prefix+'/users?username=yu&exact=true',bearer=token);assert len(users)==1 and users[0]['username']=='yu' and users[0]['enabled'];subject=users[0]['id'];uuid.UUID(subject)
 groups=request(prefix+'/users/'+subject+'/groups',bearer=token);assert any(g['name']=='platform-admin' for g in groups)
 clients=request(prefix+'/clients?clientId='+CLIENT,bearer=token);assert len(clients)==1 and clients[0]['clientId']==CLIENT;client_id=clients[0]['id']
 client=request(prefix+'/clients/'+client_id,bearer=token)
 redirect=HOST+'/oauth2/callback';redirects=client.get('redirectUris',[]);origins=client.get('webOrigins',[]);present=redirect in redirects and HOST in origins
 if a.apply and not present:
  # Update only the two fields; never send a masked or copied client secret.
  request(prefix+'/clients/'+client_id,'PUT',{'redirectUris':list(dict.fromkeys([*redirects,redirect])),'webOrigins':list(dict.fromkeys([*origins,HOST]))},bearer=token)
  after=request(prefix+'/clients/'+client_id,bearer=token);assert set(redirects).issubset(after['redirectUris']) and set(origins).issubset(after['webOrigins']);assert redirect in after['redirectUris'] and HOST in after['webOrigins'];present=True
 print(json.dumps({'owner_subject':subject,'required_group':'platform-admin','client_id':CLIENT,'callback_registered':present,'applied':a.apply,'credentials_printed':False}))
if __name__=='__main__':
 try:main()
 except Exception:print('{"ok":false,"reason":"keycloak_owner_or_callback_check_failed","credentials_printed":false}');raise SystemExit(1)

"""Map Keycloak service identities to isolated Polaris catalogs and built-in RBAC."""
from pathlib import Path
import json,requests,time
from clients import DATA,ENV,CA,token
BASE='http://127.0.0.1:19381'
for _ in range(30):
 try:
  r=requests.post(BASE+'/api/catalog/v1/oauth/tokens',data={'grant_type':'client_credentials','client_id':'root','client_secret':ENV['DEV_CATALOG_SECRET'],'scope':'PRINCIPAL_ROLE:ALL'},timeout=3)
  if r.ok:break
 except requests.RequestException:pass
 time.sleep(1)
else:raise RuntimeError('Polaris bootstrap token unavailable')
headers={'Authorization':'Bearer '+r.json()['access_token'],'Polaris-Realm':'POLARIS'}
def mg(method,path,payload=None):
 r=requests.request(method,BASE+'/api/management/v1'+path,headers=headers,json=payload,timeout=10)
 if r.status_code==409:return {}
 if not r.ok:raise RuntimeError(f'catalog bootstrap {method} {path}: HTTP {r.status_code}')
 return r.json() if r.content else {}
for app in ['konro','sample']:
 location=f's3://llmops-dev/apps/{app}/warehouse'
 mg('POST','/catalogs',{'catalog':{'name':app,'type':'INTERNAL','readOnly':False,'properties':{'default-base-location':location},'storageConfigInfo':{'storageType':'S3','endpoint':'http://127.0.0.1:19300','endpointInternal':'http://rustfs:9000','pathStyleAccess':True,'region':'us-east-1','allowedLocations':[location]}}})
for name,role,app,write in [('catalog-konro-transform','konro_transform','konro',True),('catalog-konro-eval','konro_eval','konro',False),('catalog-sample-app','sample_app','sample',True)]:
 mg('POST','/principals',{'principal':{'name':name}})
 mg('POST','/principal-roles',{'principalRole':{'name':role}})
 mg('PUT',f'/principals/{name}/principal-roles',{'principalRole':{'name':role}})
 mg('POST',f'/catalogs/{app}/catalog-roles',{'catalogRole':{'name':role}})
 mg('PUT',f'/principal-roles/{role}/catalog-roles/{app}',{'catalogRole':{'name':role}})
 privileges=['CATALOG_MANAGE_CONTENT'] if write else ['NAMESPACE_LIST','TABLE_LIST','NAMESPACE_READ_PROPERTIES','TABLE_READ_DATA','TABLE_READ_PROPERTIES']
 for privilege in privileges:mg('PUT',f'/catalogs/{app}/catalog-roles/{role}/grants',{'type':'catalog','privilege':privilege})
def bearer(name):
 r=token(name,(DATA/'clients'/f'{name}.secret').read_text().strip());assert r.ok
 return {'Authorization':'Bearer '+r.json()['access_token'],'X-Iceberg-Access-Delegation':'vended-credentials'}
for name,app in [('catalog-konro-transform','konro'),('catalog-sample-app','sample')]:
 h=bearer(name)
 for namespace in ['curated','datasets']:
  r=requests.post(BASE+f'/api/catalog/v1/{app}/namespaces',headers=h,json={'namespace':[namespace]},timeout=10)
  assert r.status_code in [200,201,409],f'OIDC namespace creation: {r.status_code}'
 # Same identity must not enumerate the other application's catalog.
 other='sample' if app=='konro' else 'konro'
 denied=requests.get(BASE+f'/api/catalog/v1/{other}/namespaces',headers=h,timeout=10)
 assert denied.status_code in [403,404],f'cross catalog namespace list accepted: {denied.status_code}'
h=bearer('catalog-konro-eval')
r=requests.get(BASE+'/api/catalog/v1/konro/namespaces',headers=h,timeout=10);assert r.ok
r=requests.post(BASE+'/api/catalog/v1/konro/namespaces',headers=h,json={'namespace':['forbidden']},timeout=10);assert r.status_code in [403,404]
# An otherwise valid API-audience token must not authenticate to Polaris.
r=requests.get(BASE+'/api/catalog/v1/konro/namespaces',headers=bearer('konro-app'),timeout=10);assert r.status_code==401
print(json.dumps({'OIDC_namespace_create':True,'own_namespace_list':True,'other_catalog_list_denied':True,'eval_namespace_write_denied':True,'wrong_audience_denied':True}))
# Real table operations use only OIDC identities and vended, warehouse-limited S3 sessions.
import duckdb
from urllib.parse import urlsplit
import boto3
from botocore.exceptions import ClientError

def database(name,app):
 t=bearer(name)['Authorization'][7:]
 c=duckdb.connect();c.execute('INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs')
 versions=dict(c.sql("SELECT extension_name,extension_version FROM duckdb_extensions() WHERE loaded").fetchall())
 assert versions['iceberg']=='4c0c0874c4' and versions['httpfs']=='77392a7207', 'extension revisions changed; revalidate first'
 c.execute('CREATE SECRET idp (TYPE iceberg, TOKEN $1)',[t])
 c.execute(f"ATTACH '{app}' AS lake (TYPE iceberg, SECRET idp, URI '{BASE}/api/catalog', ACCESS_DELEGATION_MODE 'vended_credentials')")
 return c
writer=database('catalog-konro-transform','konro')
writer.execute('CREATE TABLE IF NOT EXISTS lake.datasets._permission_probe(id INTEGER)')
writer.execute('INSERT INTO lake.datasets._permission_probe VALUES (1)')
reader=database('catalog-konro-eval','konro')
assert reader.sql('SELECT count(*) FROM lake.datasets._permission_probe').fetchall()[0][0]>=1
try:reader.execute('INSERT INTO lake.datasets._permission_probe VALUES (2)')
except duckdb.Error as e:assert '403' in str(e) or 'Forbidden' in str(e)
else:raise AssertionError('eval table write accepted')
r=requests.get(BASE+'/api/catalog/v1/konro/namespaces/datasets/tables/_permission_probe',headers=bearer('catalog-konro-eval'),timeout=10);assert r.ok
config=r.json()['config'];limited=boto3.client('s3',endpoint_url='http://127.0.0.1:19300',region_name='us-east-1',aws_access_key_id=config['s3.access-key-id'],aws_secret_access_key=config['s3.secret-access-key'],aws_session_token=config['s3.session-token'])
try:limited.get_object(Bucket='llmops-dev',Key='apps/sample/raw/probe.txt')
except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
else:raise AssertionError('vended session escaped application')
writer.execute('DROP TABLE lake.datasets._permission_probe')
print(json.dumps({'OIDC_table_create_and_read':True,'eval_table_write_denied':True,'vended_session_cross_app_denied':True,'probe_table_removed':True}))

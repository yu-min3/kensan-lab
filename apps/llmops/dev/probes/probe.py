"""Real RustFS/Polaris/DuckDB compatibility probe. Isolated data only."""
import argparse,json,time,secrets,base64,subprocess
from pathlib import Path
from urllib.parse import urlsplit
import boto3,duckdb,requests
from botocore.exceptions import ClientError

parser=argparse.ArgumentParser();parser.add_argument('--env-file',required=True);args=parser.parse_args()
env=dict(line.split('=',1) for line in Path(args.env_file).read_text().splitlines())
base='http://127.0.0.1:19281';s3url='http://127.0.0.1:19200'
for port,path in [(19200,'/health'),(19282,'/q/health')]:
 for _ in range(60):
  try:
   if requests.get(f'http://127.0.0.1:{port}{path}',timeout=3).ok:break
  except requests.RequestException:pass
  time.sleep(1)
 else:raise RuntimeError('probe service unavailable')

def token(client,secret):
 r=requests.post(base+'/api/catalog/v1/oauth/tokens',data={'grant_type':'client_credentials','client_id':client,'client_secret':secret,'scope':'PRINCIPAL_ROLE:ALL'},timeout=10)
 if not r.ok:raise RuntimeError('token request failed: '+str(r.status_code))
 return r.json()['access_token']
root=token('root',env['PROBE_CATALOG_SECRET']);headers={'Authorization':'Bearer '+root,'Polaris-Realm':'POLARIS'}
def mg(method,path,payload):
 r=requests.request(method,base+'/api/management/v1'+path,headers=headers,json=payload,timeout=15)
 if not r.ok:raise RuntimeError(f'management request failed {path}: {r.status_code}: {r.text[:700] if path.endswith('/grants') else ''}')
 return r.json() if r.content else {}
run=secrets.token_hex(4);bucket='probe-'+run;catalog='probe_'+run
s3=boto3.client('s3',endpoint_url=s3url,aws_access_key_id=env['PROBE_S3_KEY'],aws_secret_access_key=env['PROBE_S3_SECRET'],region_name='us-east-1');s3.create_bucket(Bucket=bucket)
mg('POST','/catalogs',{'catalog':{'name':catalog,'type':'INTERNAL','readOnly':False,'properties':{'default-base-location':f's3://{bucket}/warehouse'},'storageConfigInfo':{'storageType':'S3','endpoint':s3url,'endpointInternal':'http://rustfs:9000','pathStyleAccess':True,'region':'us-east-1','allowedLocations':[f's3://{bucket}/warehouse']}}})
mg('PUT',f'/catalogs/{catalog}/catalog-roles/catalog_admin/grants',{'type':'catalog','privilege':'CATALOG_MANAGE_CONTENT'})

def conn(client,secret):
 c=duckdb.connect();c.execute('INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs;')
 versions=dict(c.sql("SELECT extension_name,extension_version FROM duckdb_extensions() WHERE loaded").fetchall())
 for name,expected in {'iceberg':'4c0c0874c4','httpfs':'77392a7207','avro':'345c3f4e82'}.items():
  assert versions[name]==expected, 'extension revision changed; revalidate compatibility'
 c.execute('CREATE SECRET polaris (TYPE iceberg, CLIENT_ID $1, CLIENT_SECRET $2, OAUTH2_SERVER_URI $3)',[client,secret,base+'/api/catalog/v1/oauth/tokens'])
 c.execute(f"ATTACH '{catalog}' AS lake (TYPE iceberg, SECRET polaris, URI '{base}/api/catalog', ACCESS_DELEGATION_MODE 'vended_credentials')")
 return c
admin=conn('root',env['PROBE_CATALOG_SECRET'])
admin.execute('CREATE SCHEMA lake.konro');admin.execute('CREATE SCHEMA lake.sample')
admin.execute('CREATE TABLE lake.konro.recipes (id INTEGER, title VARCHAR)');admin.execute("INSERT INTO lake.konro.recipes VALUES (1,'onion')")
snap=admin.sql('SELECT snapshot_id FROM iceberg_snapshots(lake.konro.recipes)').fetchall()[0][0]
admin.execute("INSERT INTO lake.konro.recipes VALUES (2,'tomato')")
assert admin.sql('SELECT count(*) FROM lake.konro.recipes').fetchall()[0][0]==2
assert admin.sql(f'SELECT count(*) FROM lake.konro.recipes AT (VERSION => {snap})').fetchall()[0][0]==1
admin.execute('CREATE TABLE lake.sample.records (id INTEGER)');admin.execute('INSERT INTO lake.sample.records VALUES (10)')
principal='eval_'+run;role='eval_role_'+run
credentials=mg('POST','/principals',{'principal':{'name':principal}})['credentials']
mg('POST','/principal-roles',{'principalRole':{'name':role}})
mg('PUT',f'/principals/{principal}/principal-roles',{'principalRole':{'name':role}})
mg('POST',f'/catalogs/{catalog}/catalog-roles',{'catalogRole':{'name':role}})
mg('PUT',f'/principal-roles/{role}/catalog-roles/{catalog}',{'catalogRole':{'name':role}})
for privilege in ['NAMESPACE_LIST']:
 mg('PUT',f'/catalogs/{catalog}/catalog-roles/{role}/grants',{'type':'catalog','privilege':privilege})
for privilege in ['NAMESPACE_READ_PROPERTIES','TABLE_LIST']:
 mg('PUT',f'/catalogs/{catalog}/catalog-roles/{role}/grants',{'type':'namespace','namespace':['konro'],'privilege':privilege})
for privilege in ['TABLE_READ_DATA','TABLE_READ_PROPERTIES']:
 mg('PUT',f'/catalogs/{catalog}/catalog-roles/{role}/grants',{'type':'table','namespace':['konro'],'tableName':'recipes','privilege':privilege})
reader=conn(credentials['clientId'],credentials['clientSecret'])
assert reader.sql('SELECT count(*) FROM lake.konro.recipes').fetchall()[0][0]==2
for name,sql in [('readonly','INSERT INTO lake.konro.recipes VALUES (3,\'forbidden\')'),('other_app','SELECT * FROM lake.sample.records')]:
 try:reader.execute(sql)
 except duckdb.Error as error:
  assert '403' in str(error) or 'Forbidden' in str(error) or '404' in str(error), 'rejection was not authorization failure'
 else:raise AssertionError(name+' unexpectedly permitted')
# Inspect vended credentials only in memory, then attempt an S3 bypass outside the table.
t=token(credentials['clientId'],credentials['clientSecret'])
r=requests.get(base+f'/api/catalog/v1/{catalog}/namespaces/konro/tables/recipes',headers={'Authorization':'Bearer '+t,'X-Iceberg-Access-Delegation':'vended-credentials'},timeout=10);assert r.ok
config=r.json()['config']
location=urlsplit(r.json()['metadata']['location'])
assert location.netloc==bucket
table_key=location.path.lstrip('/')
limited=boto3.client('s3',endpoint_url=s3url,region_name='us-east-1',aws_access_key_id=config['s3.access-key-id'],aws_secret_access_key=config['s3.secret-access-key'],aws_session_token=config['s3.session-token'])
s3.put_object(Bucket=bucket,Key='other-app/private.txt',Body=b'not shared')
try:limited.get_object(Bucket=bucket,Key='other-app/private.txt')
except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
else:raise AssertionError('S3 bypass unexpectedly permitted')
try:limited.put_object(Bucket=bucket,Key=f'{table_key}/forbidden-{run}.txt',Body=b'forbidden')
except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
else:raise AssertionError('read-only vended credentials allowed write')
# Force real token expiry, reject the expired bearer, then reuse the OAuth secret.
claims=json.loads(base64.urlsafe_b64decode(t.split('.')[1]+'=='))
remaining=claims['exp']-time.time()
assert 0 < remaining <= 25, 'short-lived probe token configuration not applied'
time.sleep(max(0,remaining)+2)
expired=requests.get(base+f'/api/catalog/v1/{catalog}/namespaces/konro/tables/recipes',headers={'Authorization':'Bearer '+t},timeout=10)
assert expired.status_code==401, 'expired bearer accepted'
reader.execute('DETACH lake')
reader.execute(f"ATTACH '{catalog}' AS lake (TYPE iceberg, SECRET polaris, URI '{base}/api/catalog', ACCESS_DELEGATION_MODE 'vended_credentials')")
assert reader.sql('SELECT count(*) FROM lake.konro.recipes').fetchall()[0][0]==2
# Restart only this probe catalog; Postgres must preserve table and grants.
subprocess.run(['docker','compose','--env-file',args.env_file,'-f',str(Path(__file__).with_name('compose.yaml')),'restart','polaris'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
for _ in range(60):
 try:
  if requests.get('http://127.0.0.1:19282/q/health',timeout=2).ok:break
 except requests.RequestException:pass
 time.sleep(1)
else:raise RuntimeError('catalog restart failed')
reconnected=conn(credentials['clientId'],credentials['clientSecret'])
assert reconnected.sql('SELECT count(*) FROM lake.konro.recipes').fetchall()[0][0]==2
assert reconnected.sql(f'SELECT count(*) FROM lake.konro.recipes AT (VERSION => {snap})').fetchall()[0][0]==1
try:reconnected.execute("INSERT INTO lake.konro.recipes VALUES (3, 'forbidden')")
except duckdb.Error as error:assert '403' in str(error) or 'Forbidden' in str(error)
else:raise AssertionError('grant changed after restart')
print(json.dumps({'duckdb':duckdb.__version__,'create_insert_select':True,'snapshot_A_after_B':True,'reader_select':True,'reader_write_denied':True,'other_app_catalog_denied':True,'direct_S3_other_prefix_denied':True,'direct_S3_write_denied':True,'expired_bearer_denied':True,'oauth_secret_refresh_after_expiry':True,'catalog_restart_table_and_grants':True,'oidc_tested':False,'backup_restore_tested':False},indent=2))

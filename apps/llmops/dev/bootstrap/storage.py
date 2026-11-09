"""Provision limited RustFS users/policies using an isolated admin-only mc config."""
from pathlib import Path
import hashlib,json,os,secrets,subprocess,urllib.request,platform
import boto3
from botocore.exceptions import ClientError
ROOT=Path(__file__).resolve().parents[4];DATA=ROOT/'temp/llmops-dev';ENV=dict(line.split('=',1) for line in (DATA/'dev.env').read_text().splitlines());BUCKET='llmops-dev'
MC_RELEASE='RELEASE.2025-08-13T08-35-41Z'
TARGET=platform.system().lower()+'-'+{'arm64':'arm64','aarch64':'arm64','x86_64':'amd64'}.get(platform.machine(),platform.machine())
MC_SHA={'darwin-arm64':'a877fd0c183409da9f20f9d6e1811987298bbbca1aa03428eebdffba79fb9445','darwin-amd64':'2862c79cce11b09be9a8911a279b2e9465bebf74b9f01abca9c348a0d795f0cb','linux-amd64':'01f866e9c5f9b87c2b09116fa5d7c06695b106242d829a8bb32990c00312e891','linux-arm64':'14c8c9616cfce4636add161304353244e8de383b2e2752c0e9dad01d4c27c12c'}[TARGET]
mc=DATA/'mc'
if not mc.exists():
 data=urllib.request.urlopen(f'https://github.com/minio/mc/releases/download/{MC_RELEASE}/mc.{TARGET}.{MC_RELEASE}',timeout=60).read()
 assert hashlib.sha256(data).hexdigest()==MC_SHA
 mc.write_bytes(data);os.chmod(mc,0o700)
assert hashlib.sha256(mc.read_bytes()).hexdigest()==MC_SHA
os.chmod(mc,0o700)
config=DATA/'mc-config';config.mkdir(exist_ok=True);os.chmod(config,0o700)
p=config/'config.json';p.write_text(json.dumps({'version':'10','aliases':{'local':{'url':'http://127.0.0.1:19300','accessKey':ENV['DEV_S3_KEY'],'secretKey':ENV['DEV_S3_SECRET'],'api':'S3v4','path':'auto'}}}));os.chmod(p,0o600)
def admin(*args):
 # CLI requires secret positional arguments for user add; never print command/stdout.
 result=subprocess.run([str(mc),'--config-dir',str(config),*args],capture_output=True,text=True)
 if result.returncode:raise RuntimeError('RustFS administrator operation failed: '+args[0]+' '+args[1]+((': '+result.stderr) if args[1]=='policy' else ''))
def client(c):return boto3.client('s3',endpoint_url='http://127.0.0.1:19300',region_name='us-east-1',aws_access_key_id=c['access_key'],aws_secret_access_key=c['secret_key'])
root=client({'access_key':ENV['DEV_S3_KEY'],'secret_key':ENV['DEV_S3_SECRET']})
try:root.create_bucket(Bucket=BUCKET)
except ClientError as e:
 if e.response['Error']['Code'] not in ['BucketAlreadyOwnedByYou','BucketAlreadyExists']:raise
roles=json.loads((ROOT/'apps/llmops/config/access-model.json').read_text())['object_roles']
directory=DATA/'storage';directory.mkdir(exist_ok=True);os.chmod(directory,0o700)
for name,spec in roles.items():
 file=directory/(name+'.json')
 if file.exists():credential=json.loads(file.read_text())
 else:
  credential={'access_key':secrets.token_hex(12),'secret_key':secrets.token_hex(24)};file.write_text(json.dumps(credential));os.chmod(file,0o600)
 statements=[]
 for mode,prefixes in spec.items():
  actions=['s3:GetObject']+(['s3:PutObject'] if mode in ['read_write','read_append'] else [])+(['s3:DeleteObject'] if mode=='read_write' else [])
  statements.append({'Effect':'Allow','Action':actions,'Resource':[f'arn:aws:s3:::{BUCKET}/{prefix}*' for prefix in prefixes]})
  statements.append({'Effect':'Allow','Action':['s3:ListBucket'],'Resource':[f'arn:aws:s3:::{BUCKET}'],'Condition':{'StringLike':{'s3:prefix':[prefix+'*' for prefix in prefixes]}}})
 # Bucket region discovery is harmless and needed by some clients.
 statements.append({'Effect':'Allow','Action':['s3:GetBucketLocation'],'Resource':[f'arn:aws:s3:::{BUCKET}']})
 if name=='catalog-store':statements.append({'Effect':'Allow','Action':['sts:AssumeRole'],'Resource':['arn:aws:s3:::*']})
 policy=directory/(name+'-policy.json');policy.write_text(json.dumps({'Version':'2012-10-17','Statement':statements}));os.chmod(policy,0o600)
 admin('admin','user','add','local',credential['access_key'],credential['secret_key'])
 admin('admin','policy','create','local',name,str(policy))
 admin('admin','policy','attach','local',name,'--user',credential['access_key'])
# Install only the limited catalog warehouse credential into the server environment.
cat=json.loads((directory/'catalog-store.json').read_text());ENV.update(CATALOG_S3_KEY=cat['access_key'],CATALOG_S3_SECRET=cat['secret_key'])
(DATA/'dev.env').write_text(''.join(f'{k}={v}\n' for k,v in ENV.items()));os.chmod(DATA/'dev.env',0o600)
root.put_object(Bucket=BUCKET,Key='apps/konro/raw/probe.txt',Body=b'konro')
root.put_object(Bucket=BUCKET,Key='apps/sample/raw/probe.txt',Body=b'sample')
root.put_object(Bucket=BUCKET,Key='apps/konro/releases/probe.json',Body=b'{}')
for name in ['api-konro','eval-konro','transform-konro']:
 c=client(json.loads((directory/(name+'.json')).read_text()))
 assert c.get_object(Bucket=BUCKET,Key='apps/konro/raw/probe.txt')['Body'].read()==b'konro'
 try:c.get_object(Bucket=BUCKET,Key='apps/sample/raw/probe.txt')
 except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
 else:raise AssertionError('cross-app object accepted')
eval_client=client(json.loads((directory/'eval-konro.json').read_text()))
try:eval_client.put_object(Bucket=BUCKET,Key='apps/konro/releases/forbidden.json',Body=b'{}')
except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
else:raise AssertionError('eval changed dataset release')
sample_client=client(json.loads((directory/'api-sample.json').read_text()))
assert sample_client.get_object(Bucket=BUCKET,Key='apps/sample/raw/probe.txt')['Body'].read()==b'sample'
try:sample_client.get_object(Bucket=BUCKET,Key='apps/konro/raw/probe.txt')
except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
else:raise AssertionError('sample read konro Raw')
for actor,key in [('eval-konro','apps/konro/warehouse/forbidden.txt'),('transform-konro','apps/konro/raw/forbidden.txt')]:
 c=client(json.loads((directory/(actor+'.json')).read_text()))
 try:c.put_object(Bucket=BUCKET,Key=key,Body=b'forbidden')
 except ClientError as e:assert e.response['ResponseMetadata']['HTTPStatusCode']==403
 else:raise AssertionError('role wrote forbidden prefix')
print(json.dumps({'storage_roles' :len(roles),'app_raw_read_success':True,'other_app_get_denied':True,'eval_manifest_write_denied':True}))

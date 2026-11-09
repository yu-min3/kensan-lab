"""Check actual readonly enforcement on this dataset table and release object."""
from pathlib import Path
import json,sys,ssl,urllib.parse,urllib.request
import boto3,duckdb
from botocore.exceptions import ClientError
root=Path.cwd();data=root/'temp/llmops-dev';release=json.loads((root/'temp/llmops-dataset-demo/release-a.json').read_text())
secret=(data/'clients/catalog-konro-eval.secret').read_text().strip()
req=urllib.request.Request('https://id.localhost:19443/realms/llmops/protocol/openid-connect/token',data=urllib.parse.urlencode({'grant_type':'client_credentials','client_id':'catalog-konro-eval','client_secret':secret}).encode())
with urllib.request.urlopen(req,context=ssl.create_default_context(cafile=str(data/'ca.pem'))) as r:token=json.load(r)['access_token']
c=duckdb.connect();c.execute('LOAD iceberg; LOAD httpfs;');c.execute('CREATE SECRET readonly (TYPE iceberg,TOKEN $1)',[token]);c.execute("ATTACH 'konro' AS lake(TYPE iceberg,SECRET readonly,URI 'http://127.0.0.1:19381/api/catalog',ACCESS_DELEGATION_MODE 'vended_credentials')")
try:c.execute("INSERT INTO lake.datasets.import_cases VALUES('forbidden','x','x','{}','{}','1')")
except duckdb.Error as error:assert '403' in str(error) or 'Forbidden' in str(error)
else:raise AssertionError('readonly dataset table mutation permitted')
creds=json.loads((data/'storage/eval-konro.json').read_text());s3=boto3.client('s3',endpoint_url='http://127.0.0.1:19300',region_name='us-east-1',aws_access_key_id=creds['access_key'],aws_secret_access_key=creds['secret_key'])
try:s3.put_object(Bucket='llmops-dev',Key='apps/konro/releases/'+release['release_id']+'/manifest.json',Body=b'{}')
except ClientError as error:assert error.response['ResponseMetadata']['HTTPStatusCode']==403
else:raise AssertionError('readonly release manifest mutation permitted')
print(json.dumps({'real_eval_table_write_denied':True,'real_eval_manifest_write_denied':True}))

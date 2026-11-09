"""Trusted DuckDB bridge. Only fixed operations; input/credentials arrive via stdin."""
import json,re,sys,ssl,urllib.request
import duckdb

stage="startup"
def main():
 global stage
 data=json.load(sys.stdin);config=data['catalog']
 assert duckdb.__version__=='2.0.0'
 connection=duckdb.connect()
 connection.execute('INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs;')
 versions=dict(connection.sql("SELECT extension_name,extension_version FROM duckdb_extensions() WHERE loaded").fetchall())
 assert versions['iceberg']=='4c0c0874c4' and versions['httpfs']=='77392a7207'
 def ident(value):
  assert re.fullmatch('[a-z][a-z0-9_]{0,62}',value)
  return '"'+value+'"'
 table='lake.'+ident(config['namespace'])+'.'+ident(config['table'])
 tokenrequest=urllib.request.Request(config['token_url'],data=__import__('urllib.parse',fromlist=['urlencode']).urlencode({'grant_type':'client_credentials','client_id':config['client_id'],'client_secret':data['client_secret']}).encode(),headers={'Content-Type':'application/x-www-form-urlencoded'})
 context=ssl.create_default_context(cafile=config['ca_file'])
 with urllib.request.urlopen(tokenrequest,context=context,timeout=15) as response:token=json.load(response)['access_token']
 connection.execute('CREATE SECRET identity (TYPE iceberg, TOKEN $1)',[token])
 stage='attach'
 connection.execute("ATTACH $1 AS lake (TYPE iceberg, SECRET identity, URI $2, ACCESS_DELEGATION_MODE 'vended_credentials')",[config['name'],config['uri']])
 assert data['operation']=='curate'
 stage='ddl'
 connection.execute(data['ddl'].replace('{{table}}',table))
 stage='incoming'
 connection.execute('CREATE TEMP TABLE incoming AS SELECT * FROM '+table+' LIMIT 0')
 stage='bind'
 for row in data['rows']:
  content=json.dumps(row['content'])
  connection.execute("INSERT INTO incoming VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::JSON, from_json(?::JSON, '{\"title\":\"VARCHAR\",\"servings\":\"VARCHAR\",\"ingredients\":[\"VARCHAR\"],\"steps\":[\"VARCHAR\"]}'), ?::JSON, ?::VARCHAR[])",[row['record_id'],row['app_id'],row['capture_id'],row['raw_object_id'],row['raw_sha256'],row['source_uri'],row['source_group'],row['schema_version'],row['transform_version'],row['adapter_sha256'],row['prepared_text'],row['text_bytes'],row['text_truncated'],content,content,json.dumps(row['output_schema']),row['warnings']])
 # One Iceberg commit publishes the batch. Replay never appends duplicate record IDs.
 stage='merge'
 connection.execute('MERGE INTO '+table+' target USING incoming source ON target.record_id=source.record_id WHEN NOT MATCHED THEN INSERT BY NAME')
 stage='read'
 records=connection.execute('SELECT target.record_id,target.capture_id,target.raw_sha256,target.recipe.title,target.source_group,target.transform_version,target.adapter_sha256,target.text_truncated,target.recipe.ingredients,target.recipe.steps,target.source_uri,target.schema_version FROM '+table+' target JOIN incoming source ON target.record_id=source.record_id ORDER BY target.record_id').fetchall()
 expected={row['record_id']:row for row in data['rows']}
 for record in records:
  row=expected[record[0]]
  assert record[1]==row['capture_id'] and record[2]==row['raw_sha256']
  assert record[3]==row['content'].get('title') and record[4]==row['source_group']
  assert record[5]==row['transform_version'] and record[6]==row['adapter_sha256']
  assert record[7]==row['text_truncated'] and record[8]==row['content'].get('ingredients') and record[9]==row['content'].get('steps')
  assert record[10]==row['source_uri'] and record[11]==row['schema_version']
 stage='snapshots'
 snapshots=connection.sql('SELECT snapshot_id FROM iceberg_snapshots('+table+') ORDER BY sequence_number DESC').fetchall()
 print(json.dumps({'snapshot_id':str(snapshots[0][0]),'records':[{'record_id':r[0],'capture_id':r[1],'raw_sha256':r[2],'title':r[3]} for r in records]}))
try:main()
except Exception as error:
 # Do not print raw SQL, parameters, credentials, URL error bodies, or source text.
 print(json.dumps({'error':'catalog operation failed','type':type(error).__name__,'stage':stage}),file=sys.stderr)
 sys.exit(1)

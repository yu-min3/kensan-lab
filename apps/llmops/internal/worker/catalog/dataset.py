"""Fixed dataset build/read and retention checks; no arbitrary query endpoint."""
import json,re,sys,ssl,urllib.request,urllib.parse
import duckdb
stage='startup'
def main():
 global stage
 data=json.load(sys.stdin);cfg=data['catalog'];op=data['operation']
 assert duckdb.__version__=='2.0.0'
 c=duckdb.connect();c.execute('INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs;')
 versions=dict(c.sql('SELECT extension_name,extension_version FROM duckdb_extensions() WHERE loaded').fetchall())
 assert versions['iceberg']=='4c0c0874c4' and versions['httpfs']=='77392a7207'
 def identifier(name):
  assert re.fullmatch('[a-z][a-z0-9_]{0,62}',name)
  return '"'+name+'"'
 def table(ns,name):return 'lake.'+identifier(ns)+'.'+identifier(name)
 context=ssl.create_default_context(cafile=cfg['ca_file'])
 req=urllib.request.Request(cfg['token_url'],data=urllib.parse.urlencode({'grant_type':'client_credentials','client_id':cfg['client_id'],'client_secret':data['client_secret']}).encode())
 with urllib.request.urlopen(req,context=context,timeout=15) as response:token=json.load(response)['access_token']
 def rest(ns,name,payload=None):
  path=cfg['uri'].rstrip('/')+'/v1/'+urllib.parse.quote(cfg['name'],safe='')+'/namespaces/'+urllib.parse.quote(ns,safe='')+'/tables/'+urllib.parse.quote(name,safe='')
  req=urllib.request.Request(path,headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'},data=json.dumps(payload).encode() if payload is not None else None)
  with urllib.request.urlopen(req,timeout=15) as response:return json.load(response)
 def pin(ns,name,snapshot,ref):
  metadata=rest(ns,name)['metadata'];existing=metadata.get('refs',{}).get(ref)
  if existing:
   assert existing['type']=='tag' and existing['snapshot-id']==int(snapshot) and existing.get('max-ref-age-ms')==9223372036854775807
   return
  rest(ns,name,{'requirements':[{'type':'assert-table-uuid','uuid':metadata['table-uuid']},{'type':'assert-ref-snapshot-id','ref':ref,'snapshot-id':None}],'updates':[{'action':'set-snapshot-ref','ref-name':ref,'type':'tag','snapshot-id':int(snapshot),'max-ref-age-ms':9223372036854775807}]})
  assert rest(ns,name)['metadata']['refs'][ref]['snapshot-id']==int(snapshot)
 c.execute('CREATE SECRET idp (TYPE iceberg,TOKEN $1)',[token]);c.execute("ATTACH $1 AS lake(TYPE iceberg,SECRET idp,URI $2,ACCESS_DELEGATION_MODE 'vended_credentials')",[cfg['name'],cfg['uri']])
 target=table(cfg['namespace'],cfg['table'])
 if op=='build':
  stage='source'
  assert re.fullmatch('[0-9]{1,20}',data['source_snapshot'])
  source=table(data['source_namespace'],data['source_table']);snapshot=int(data['source_snapshot'])
  resolved=[]
  for entry in data['entries']:
   case=entry['case'];rows=c.execute('SELECT capture_id,raw_object_id,raw_sha256,source_group,prepared_text,content_json,output_schema,text_truncated FROM '+source+f' AT(VERSION => {snapshot}) WHERE record_id=?',[entry['record_id']]).fetchall();assert len(rows)==1
   r=rows[0];assert r[1]==case['input']['object_id'] and r[2]==case['input']['sha256'] and r[3]==case['source_group']
   prepared={'text':r[4],'content':json.loads(r[5]),'output_schema':json.loads(r[6]),'text_truncated':r[7]}
   resolved.append({'case':case,'record_id':entry['record_id'],'prepared':prepared})
  stage='commit'
  c.execute('CREATE TABLE IF NOT EXISTS '+target+'(release_id VARCHAR,case_id VARCHAR,source_record_id VARCHAR,case_json VARCHAR,prepared_input_json VARCHAR,source_snapshot_id VARCHAR)')
  ref='release_'+data['release_id']
  existing=rest(cfg['namespace'],cfg['table'])['metadata'].get('refs',{}).get(ref)
  if existing:ds=str(existing['snapshot-id'])
  else:
   c.execute('CREATE TEMP TABLE incoming AS SELECT * FROM '+target+' LIMIT 0')
   for row in resolved:c.execute('INSERT INTO incoming VALUES(?,?,?,?,?,?)',[data['release_id'],row['case']['case_id'],row['record_id'],json.dumps(row['case']),json.dumps(row['prepared']),data['source_snapshot']])
   c.execute('MERGE INTO '+target+' dst USING incoming src ON dst.release_id=src.release_id AND dst.case_id=src.case_id WHEN NOT MATCHED THEN INSERT BY NAME')
   ds=str(c.sql('SELECT snapshot_id FROM iceberg_snapshots('+target+') ORDER BY sequence_number DESC').fetchall()[0][0])
  stage='pin'
  pin(data['source_namespace'],data['source_table'],snapshot,ref)
  pin(cfg['namespace'],cfg['table'],ds,ref)
  stored=c.execute('SELECT case_json,prepared_input_json FROM '+target+' AT(VERSION => '+ds+') WHERE release_id=? ORDER BY case_id',[data['release_id']]).fetchall()
  assert [{'case':json.loads(r[0]),'prepared':json.loads(r[1])} for r in stored]==[{'case':r['case'],'prepared':r['prepared']} for r in sorted(resolved,key=lambda x:x['case']['case_id'])]
  print(json.dumps({'snapshot_id':ds,'retention_tag':ref,'count':len(stored)}));return
 if op=='guard':
  metadata=rest(cfg['namespace'],cfg['table'])['metadata']
  pinned=any(ref.startswith('release_') and str(value['snapshot-id'])==data['snapshot_id'] for ref,value in metadata.get('refs',{}).items())
  print(json.dumps({'pinned':pinned}));return
 assert op=='read'
 stage='pins'
 for ref in data['tables']:
  parts=ref['table'].split('.');assert len(parts)==3 and parts[0]==cfg['name']
  pinvalue=rest(parts[1],parts[2])['metadata'].get('refs',{}).get(ref['retention_tag']);assert pinvalue and pinvalue['type']=='tag' and str(pinvalue['snapshot-id'])==ref['snapshot_id'] and pinvalue.get('max-ref-age-ms')==9223372036854775807
 stage='read'
 snap=data['snapshot_id'];assert re.fullmatch('[0-9]{1,20}',snap)
 rows=c.execute('SELECT case_json,prepared_input_json,source_record_id FROM '+target+' AT(VERSION => '+snap+') WHERE release_id=? ORDER BY case_id LIMIT 101',[data['release_id']]).fetchall();assert 0<len(rows)<=100
 print(json.dumps({'cases':[{'case':json.loads(r[0]),'prepared':json.loads(r[1]),'record_id':r[2]} for r in rows]}))
try:main()
except Exception as error:
 print(json.dumps({'error':'dataset operation failed','type':type(error).__name__,'stage':stage}),file=sys.stderr);sys.exit(1)

import os,sys,json,hashlib,subprocess,glob,stat,urllib.request,datetime
os.umask(0o077)
BASE='/opt/kensan-dev'
STATE='/var/lib/kensan-dev-t022-recovery-005'
WORK=BASE+'/t022-recovery-worktrees-005'
RUNTIME=BASE+'/model-rootfs-v5'
RECORD=BASE+'/t022-recovery-evidence-005'
ARCHIVE=BASE+'/t022-recovery-archive-005'
BIN=BASE+'/recovery-fixture-56f9f69/sense-recovery-fixture'
SHA='598288f6536e44d9ff173f1970d2115bf96e82fcc1301958605dd3a08a90edd1'
SOURCE=BASE+'/t022-run-source-002'
SOURCE_SHA='d101a3b36f10910b01ccfec4b4e3a993f2cdf048'
AUTH=BASE+'/auth/claude'
GATE=BASE+'/live-t022/t022-recovery-release_gate-006/result.json'
def digest(path):
 with open(path,'rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def run(args,**kwargs):return subprocess.check_output(args,text=True,**kwargs).strip()
def save(name,body):
 with open(RECORD+'/'+name,'x') as f:json.dump(body,f,indent=2);f.write('\n');f.flush();os.fsync(f.fileno())
def pinchecks():
 assert digest(GATE)=='fabee8b2d87c6e4afe0a7f05187d64c8a66ece7c1f98f8a9769cf935a11c82c9'
 gate=json.load(open(GATE)); decision=json.loads(gate['output'])
 assert decision['decision']=='allow', 'independent Gate is not allow'
 assert digest(BASE+'/model-rootfs-v5/usr/local/bin/sense-dev-worker')=='f316bae4d4b092b88c4235fdcea62c5a0ee57395404b86524c41f6a596ced567'
 assert digest(BASE+'/sandbox-fixture/profile.apparmor')=='cd5c4aba4414c9563e07c6f27b190b961acf719e6f5140e8b96f64852099358e'
 assert digest('/etc/apparmor.d/bwrap-userns-restrict')=='11d39094f044f0cda0febb3ad517b830301da6b2ce929664af09ee9e4dd264f9'
 assert digest('/usr/bin/bwrap')=='e318903862396f96de3df57264e0158682b952fd3fb53ac23d876413e7b30f71'
 assert run(['sudo','-n','-u','kensan-dev','git','-C',SOURCE,'rev-parse','HEAD'])==SOURCE_SHA
 assert not run(['sudo','-n','-u','kensan-dev','git','-C',SOURCE,'status','--porcelain'])
 for svc in ['kensan-dev-controller','libvirtd']:assert run(['systemctl','is-active',svc])=='active'
 return digest(GATE)
def billing():
 credentials=json.load(open(AUTH+'/.credentials.json'));access=credentials['claudeAiOauth']['accessToken']
 req=urllib.request.Request('https://api.anthropic.com/api/oauth/usage',headers={'Authorization':'Bearer '+access,'anthropic-beta':'oauth-2025-04-20'})
 usage=json.load(urllib.request.urlopen(req,timeout=15))['extra_usage']
 evidence={k:usage.get(k) for k in ['is_enabled','monthly_limit','used_credits']}
 assert evidence['is_enabled'] is False and evidence['used_credits']==0,'billing boundary changed'
 return evidence
def snapshot():return json.load(open(STATE+'/state.json'))
def task_ids(st):
 assert len(st['tasks'])==2 and len(st['agents'])==2
 for task in st['tasks'].values():assert task['mission_id']=='t022-recovery-fixture-v1' and task['base_sha']==SOURCE_SHA
 return st
def session_log(session):
 import re
 assert re.fullmatch('[a-f0-9-]{36}',session)
 matches=glob.glob(AUTH+'/projects/**/'+session+'.jsonl',recursive=True)
 assert len(matches)==1,'own newly-created session log must be unique'
 path=matches[0]
 assert os.path.realpath(path)==path and stat.S_ISREG(os.lstat(path).st_mode)
 return path
def metadata(path):
 models=set();messages=0
 for line in open(path):
  obj=json.loads(line)
  if obj.get('type')=='assistant' and obj.get('message',{}).get('model'):
   models.add(obj['message']['model']);messages+=1
 assert models=={'claude-opus-5-5'} and messages>0
 return dict(models=sorted(models),assistant_messages=messages,sha256=digest(path),bytes=os.stat(path).st_size)
def invoke(phase,index):
 assert digest(BIN)==SHA
 name='sense-t022-recovery-'+phase+'-'+str(index)
 args=['systemd-run','--quiet','--wait','--pipe','--collect','--unit',name,'--uid=kensan-dev','--property=NoNewPrivileges=yes','--property=MemoryMax=1G','--property=TasksMax=192','--property=CPUQuota=100%','--property=RuntimeMaxSec=180','--property=LimitCORE=0','--property=ProtectSystem=strict','--property=ProtectHome=yes','--property=PrivateTmp=yes','--property=ReadWritePaths='+STATE+' '+WORK+' '+AUTH,BIN,'-phase',phase,'-state',STATE,'-source',SOURCE,'-worktrees',WORK,'-runtime',RUNTIME,'-auth',AUTH,'-bwrap','/usr/bin/bwrap','-base',SOURCE_SHA]
 proc=subprocess.run(args,text=True,capture_output=True,timeout=185)
 save(name+'.json',dict(exit_code=proc.returncode,stdout=proc.stdout,stderr=proc.stderr))
 assert proc.returncode==0,'fixture failed; no further automatic call'
 return proc.stdout
def artifact_body(st,ref):
 artifact=st['artifacts'][ref['id']]
 assert artifact['sha256']==ref['sha256'] and artifact['version']==ref['version']
 path=STATE+'/'+artifact['path'];assert digest(path)==artifact['sha256']
 return open(path,'rb').read()
def verify_previous(st):
 for turn in st['attempts'].values():
  ref=turn.get('output_ref');assert ref,'every previous turn must have evidence'
  body=artifact_body(st,ref)
  if turn['session_generation']==1:
   proof=json.loads(body)
   assert turn['status']=='failed' and proof['session_id']==turn['session_id'] and proof['child_exit']==0 and proof['relay_exit']=='SIGKILL' and proof['result_bytes']>0 and len(proof['result_sha256'])==64 and proof['type']=='fault_injected'
  else:
   assert turn['status']=='completed'
   data=json.loads(body);own='T022_PRIVATE_'+turn['team'].upper()+'_9B6E'
   other='T022_PRIVATE_'+('APP' if turn['team']=='platform' else 'PLATFORM')+'_9B6E'
   assert data.get('team')==turn['team'] and own in body.decode() and other not in body.decode(), 'recovered JSON team/memo mismatch: stop before another call'
mode=sys.argv[1]
gate_hash=pinchecks()
if mode=='prepare':
 data=sys.stdin.buffer.read();assert hashlib.sha256(data).hexdigest()==SHA
 for path in [STATE,WORK,RECORD,ARCHIVE]:assert not os.path.lexists(path),'fresh paths required: '+path
 os.mkdir(RECORD,0o700);os.mkdir(ARCHIVE,0o700)
 for path in [STATE,WORK]:os.mkdir(path,0o700);os.chown(path,996,988)
 assert digest(BIN)==SHA
 assert digest(RUNTIME+'/usr/local/bin/sense-dev-worker')=='f316bae4d4b092b88c4235fdcea62c5a0ee57395404b86524c41f6a596ced567'
 save('manifest.json',dict(gate_result_sha256=gate_hash,fixture_sha256=SHA,fixture_source='56f9f69',test_fix_source='e2e9d04',source_sha=SOURCE_SHA,state=STATE,worktrees=WORK,runtime=RUNTIME,models_max=4,billing=billing(),started_at=datetime.datetime.now(datetime.timezone.utc).isoformat()))
 invoke('init',0)
 print('prepared dedicated fixture; no model turn')
elif mode in ['kill-turn','recovery-turn']:
 st=task_ids(snapshot());verify_previous(st);before={k:dict(v) for k,v in st['attempts'].items()};index=len(before)+1
 if mode=='kill-turn':assert len(before) in [0,1]
 else:assert len(before) in [2,3]
 save('billing-before-'+str(index)+'.json',billing())
 invoke(mode,index)
 after=task_ids(snapshot());new=[v for k,v in after['attempts'].items() if k not in before]
 assert len(new)==1 and len(after['attempts'])==len(before)+1
 assert all(after['attempts'][k]==v for k,v in before.items()),'old attempts changed'
 verify_previous(after)
 turn=new[0];sid=turn['session_id'];log=session_log(sid);meta=metadata(log)
 save('turn-'+str(index)+'.json',dict(attempt=turn,cli=meta,session_id=sid,billing=billing()))
 print(json.dumps(dict(turn=index,session_id=sid,status=turn['status'],generation=turn['session_generation'],models=meta['models'])))
elif mode=='archive':
 st=task_ids(snapshot());verify_previous(st);assert len(st['attempts'])==2
 assert os.listdir(ARCHIVE)==[]
 records=[]
 for turn in st['attempts'].values():
  assert turn['status']=='failed' and turn['session_generation']==1 and turn.get('output_ref')
  sid=turn['session_id'];path=session_log(sid);meta=metadata(path)
  record=dict(session_id=sid,original=path,archived=ARCHIVE+'/'+sid+'.jsonl',**meta)
  records.append(record)
 assert len({r['session_id'] for r in records})==2
 save('archive-plan.json',records)
 for r in records:
  os.rename(r['original'],r['archived'])
  assert digest(r['archived'])==r['sha256'] and not glob.glob(AUTH+'/projects/**/'+r['session_id']+'.jsonl',recursive=True)
 save('archive-outcome.json',records)
 print('two fixture-only histories archived unchanged; no provider probe')
elif mode=='resume':
 records=json.load(open(RECORD+'/archive-outcome.json'));st=task_ids(snapshot())
 assert len(st['attempts'])==2 and {t['session_id'] for t in st['attempts'].values()}=={r['session_id'] for r in records}
 for r in records:assert digest(r['archived'])==r['sha256'] and not glob.glob(AUTH+'/projects/**/'+r['session_id']+'.jsonl',recursive=True)
 invoke('resume',0)
 print('generation2 prepared; no model turn')
elif mode=='audit':
 invoke('audit',0)
 st=task_ids(snapshot());verify_previous(st);assert len(st['attempts'])==4
 manifests=[]
 for artifact in st['artifacts'].values():
  if artifact['kind']=='input-manifest-feedback':
   path=STATE+'/'+artifact['path'];assert digest(path)==artifact['sha256']
   manifest=json.load(open(path));attempts=[a for a in st['attempts'].values() if a['agent_id']==manifest['agent_id'] and a['session_generation']==manifest['generation']]
   assert len(attempts)==1 and attempts[0]['input_manifest_hash']==manifest['input_sha256']
   assert manifest['team']==attempts[0]['team']
   own='T022_PRIVATE_'+manifest['team'].upper()+'_9B6E';other='T022_PRIVATE_'+('APP' if manifest['team']=='platform' else 'PLATFORM')+'_9B6E'
   memo=artifact_body(st,manifest['agent_memo']).decode();assert own in memo and other not in memo
   for ref,kind in [(manifest['team_profile'],'profile-'+manifest['team']),(manifest['team_knowledge'],'knowledge-'+manifest['team']),(manifest['common_knowledge'],'knowledge-common')]:
    artifact_body(st,ref);assert st['artifacts'][ref['id']]['kind']==kind
   manifests.append(dict(agent_id=manifest['agent_id'],team=manifest['team'],generation=manifest['generation'],input_sha256=manifest['input_sha256'],artifact_sha256=artifact['sha256'],profile=manifest['team_profile'],knowledge=manifest['team_knowledge'],memo=manifest['agent_memo']))
 assert len(manifests)==4
 for agent in st['agents']:
  pair=[m for m in manifests if m['agent_id']==agent];assert len(pair)==2 and {m['generation'] for m in pair}=={1,2}
  assert all(pair[0][k]==pair[1][k] for k in ['profile','knowledge','memo'])
 pinchecks();save('final-state.json',st)
 save('outcome.json',dict(result='PASS',scope='fixture result-loss and session-history-loss reconstruction only',attempts=st['attempts'],agents=st['agents'],input_manifests=manifests,billing=billing(),services={'controller':'active mock-only unchanged','libvirtd':'active'},remaining=['provider outage classification','real mobile access','Mac priority','canary delivery/acceptance','48h pilot']))
 print('PASS: 4distinctsessions / generations1-2 / preservedhistory / 4persistedmanifests / private memo nonmixing / extra usageOFF0')
else:raise AssertionError('unknown mode')

import os,subprocess,time,json
cmd=['sudo','-n','-u','kensan-dev','setpriv','--no-new-privs','/opt/kensan-dev/sandbox-fixture/bwrap','--unshare-all','--share-net','--die-with-parent','--new-session','--clearenv','--setenv','HOME','/agent-auth','--setenv','CODEX_HOME','/agent-auth','--setenv','PATH','/usr/local/bin:/usr/bin:/bin','--ro-bind','/opt/kensan-dev/model-rootfs-v5','/','--proc','/proc','--dev','/dev','--tmpfs','/tmp','--ro-bind','/opt/kensan-dev/sandbox-fixture/checkout','/workspace','--bind','/opt/kensan-dev/sandbox-fixture/dummy-auth','/agent-auth','--chdir','/workspace','--','/opt/codex/bin/codex-code-mode-host']
p=subprocess.Popen(cmd,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,close_fds=True)
result={}
try:
 time.sleep(.4)
 children={p.pid}
 for _ in range(4):
  for name in os.listdir('/proc'):
   if not name.isdigit():continue
   try:
    text=open('/proc/'+name+'/status').read()
    ppid=int(next(x for x in text.splitlines() if x.startswith('PPid:')).split()[1])
    if ppid in children:children.add(int(name))
   except (OSError,StopIteration):pass
 hostpid=None
 for pid in children:
  try:
   argv=open(f'/proc/{pid}/cmdline','rb').read().split(b'\x00')
   if argv and argv[0]==b'/opt/codex/bin/codex-code-mode-host':hostpid=pid
  except OSError:pass
 result['host_running']=hostpid is not None and p.poll() is None
 if hostpid:
  status=open(f'/proc/{hostpid}/status').read()
  result['status']={x.split(':',1)[0]:x.split(':',1)[1].strip() for x in status.splitlines() if x.split(':',1)[0] in ['CapEff','CapPrm','CapBnd','NoNewPrivs']}
  result['profile']=open(f'/proc/{hostpid}/attr/current').read().strip()
  try:
   env=open(f'/proc/{hostpid}/environ','rb').read().split(b'\x00')
   result['env_names']=sorted(x.split(b'=',1)[0].decode() for x in env if x)
  except OSError as e:result['env_unavailable']=type(e).__name__
  try:
   result['fds']={x:os.readlink(f'/proc/{hostpid}/fd/{x}') for x in os.listdir(f'/proc/{hostpid}/fd')}
   socket_ids={v[8:-1] for v in result['fds'].values() if v.startswith('socket:[')}
   result['unix_sockets']=[line.split() for line in open(f'/proc/{hostpid}/net/unix') if len(line.split())>=7 and line.split()[6] in socket_ids]
  except OSError as e:result['fds_unavailable']=type(e).__name__
finally:
 p.stdin.close()
 try:p.wait(timeout=5)
 except subprocess.TimeoutExpired:p.terminate();p.wait(timeout=5)
 result['exit_after_stdin_eof']=p.returncode
 print(json.dumps(result,ensure_ascii=False,indent=2))

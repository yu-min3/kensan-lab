import socket,subprocess,json,hashlib
listener=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
listener.bind('\0sense-fixture-sentinel');listener.listen(1)
cmd=['sudo','-n','-u','kensan-dev','setpriv','--no-new-privs','/opt/kensan-dev/sandbox-fixture/bwrap','--unshare-all','--share-net','--die-with-parent','--new-session','--clearenv','--setenv','HOME','/agent-auth','--setenv','CODEX_HOME','/agent-auth','--setenv','PATH','/usr/local/bin:/usr/bin:/bin','--ro-bind','/opt/kensan-dev/sandbox-fixture/code-mode-probe-rootfs','/','--proc','/proc','--dev','/dev','--tmpfs','/tmp','--ro-bind','/opt/kensan-dev/sandbox-fixture/checkout','/workspace','--bind','/opt/kensan-dev/sandbox-fixture/dummy-auth','/agent-auth','--chdir','/workspace','--','/opt/codex/bin/codex-code-mode-host']
try:
 r=subprocess.run(cmd,capture_output=True,text=True,timeout=20)
 record=dict(exit_code=r.returncode,profile_probe=json.loads(r.stdout),stderr=r.stderr,external_unix_sentinel_listening=True,surrogate=True)
 s=json.dumps(record,ensure_ascii=False,indent=2)+'\n'
 print(s);print('sha256='+hashlib.sha256(s.encode()).hexdigest())
 with open('/opt/kensan-dev/sandbox-fixture/code-mode-boundary-results.json','w') as f:f.write(s)
 if r.returncode:raise SystemExit(r.returncode)
finally:listener.close()

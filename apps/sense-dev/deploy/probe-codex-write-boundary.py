import subprocess, json, hashlib, os, sys
base='/opt/kensan-dev/sandbox-fixture'
root, checkout, record, probe_binary = sys.argv[1:5]
assert os.geteuid() == 0
assert all(p.startswith(base+'/') and not os.path.lexists(p) for p in (root, checkout, record))
assert probe_binary.startswith('/') and os.path.isfile(probe_binary)
subprocess.run(['cp','-a',base+'/code-mode-probe-rootfs',root],check=True)
subprocess.run(['install','-m','0755',probe_binary,root+'/opt/codex/bin/codex-code-mode-host'],check=True)
subprocess.run(['cp','-a','/opt/kensan-dev/live-t022/checkout',checkout],check=True)
os.symlink('/agent-auth/provider-secret.txt',checkout+'/auth-alias')
cmd=['systemd-run','--quiet','--wait','--pipe','--collect','--uid=kensan-dev','--property=NoNewPrivileges=yes','--property=TasksMax=64','--property=MemoryMax=1G','--property=RuntimeMaxSec=60','--property=CPUQuota=100%','/opt/kensan-dev/sandbox-fixture/bwrap','--unshare-all','--share-net','--die-with-parent','--new-session','--clearenv','--setenv','HOME','/agent-auth','--setenv','CODEX_HOME','/agent-auth','--setenv','PATH','/usr/local/bin:/usr/bin:/bin','--ro-bind',root,'/','--proc','/proc','--dev','/dev','--tmpfs','/tmp','--ro-bind',checkout,'/workspace','--bind',checkout+'/app/main.py','/workspace/app/main.py','--bind',checkout+'/tests/test_main.py','/workspace/tests/test_main.py','--bind',base+'/dummy-auth','/agent-auth','--chdir','/workspace','--','/opt/codex/bin/codex-code-mode-host','-write-fixture']
p=subprocess.run(cmd,capture_output=True,text=True,timeout=65)
result=dict(exit_code=p.returncode,profile_probe=json.loads(p.stdout),stderr=p.stderr,surrogate=True,write_scope=['app/main.py','tests/test_main.py'])
s=json.dumps(result,indent=2)+'\n'
with open(record,'x') as f:f.write(s);f.flush();os.fsync(f.fileno())
print(s);print('sha256='+hashlib.sha256(s.encode()).hexdigest())
assert p.returncode==0

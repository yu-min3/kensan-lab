# Credential-free, offline fixture only. No model inference.
import subprocess,json,hashlib,os,sys
base=['sudo','-n','-u','kensan-dev','setpriv','--no-new-privs','/opt/kensan-dev/sandbox-fixture/bwrap','--unshare-all','--die-with-parent','--new-session','--clearenv','--setenv','HOME','/workspace','--setenv','CODEX_HOME','/tmp','--setenv','PATH','/usr/local/bin:/usr/bin:/bin','--ro-bind','/opt/kensan-dev/model-rootfs-v3','/','--proc','/proc','--dev','/dev','--tmpfs','/tmp','--ro-bind','/opt/kensan-dev/preflight-worktree','/workspace','--chdir','/workspace','--']
probe='''
test ! -e /var/lib/kensan-dev || exit 20
test ! -e /opt/kensan-dev/auth || exit 21
test -z "$(ls -A /agent-auth)" || exit 22
if touch /workspace/escape 2>/dev/null; then exit 23; fi
if touch /tmp/scratch 2>/dev/null; then exit 24; fi
while read -r key value rest; do
 case "$key" in
  CapEff:|CapPrm:|CapBnd:) test "$value" = 0000000000000000 || exit 25 ;;
  NoNewPrivs:) test "$value" = 1 || exit 26 ;;
 esac
done < /proc/self/status
cat /proc/self/attr/current
ls -l /proc/self/ns/net
printf "boundary_ok\\n"
'''
cases=[('direct_bwrap_tool_cannot_create_namespace',base+['/opt/codex/bin/codex','sandbox','--','/opt/codex/codex-resources/bwrap','--unshare-user','--ro-bind','/','/','--','/usr/bin/sh','-c',':'],False),('sandbox_boundary',base+['/opt/codex/bin/codex','sandbox','--','/usr/bin/sh','-c',probe],True),('tool_cannot_create_namespace',base+['/opt/codex/bin/codex','sandbox','--','/usr/bin/sh','-c','exec /opt/codex/codex-resources/bwrap --unshare-user --ro-bind / / -- /usr/bin/sh -c :'],False),('untrusted_outer_cannot_regain_setup',base+['/usr/bin/sh','-c','exec /opt/codex/bin/codex sandbox -- /usr/bin/sh -c :'],False)]
results=[]
for name,cmd,success in cases:
 r=subprocess.run(cmd,capture_output=True,text=True,timeout=30)
 results.append(dict(name=name,exit_code=r.returncode,passed=(r.returncode==0)==success,stdout=r.stdout,stderr=r.stderr))
host_net=subprocess.check_output(['readlink','/proc/self/ns/net'],text=True).strip()
record=dict(host_network_namespace=host_net,results=results)
s=json.dumps(record,ensure_ascii=False,indent=2)+'\n'
with open('/home/yu-min/sense-cli-staging/codex-fixture-results.json','w') as f:f.write(s)
print(s)
print('sha256='+hashlib.sha256(s.encode()).hexdigest())
sys.exit(0 if all(x['passed'] for x in results) and host_net not in results[1]['stdout'] else 1)

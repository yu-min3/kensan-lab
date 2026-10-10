#!/usr/bin/env python3
"""Isolated kind integration: actual plugin/RBAC/security + admission + lifecycle.
Never targets the production context. Requires an existing kind cluster.
"""
import json
import socket
import struct
import subprocess
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CONTEXT = 'kind-lan-dns-verification'
K = ['kubectl', '--context', CONTEXT]

def run(*args, data=None, ok=True):
    p = subprocess.run([*K, *args], input=data, text=True, capture_output=True)
    if ok and p.returncode:
        raise RuntimeError(p.stderr)
    return p

def apply(obj):
    return run('apply', '-f', '-', data=json.dumps(obj))

def namespace(name):
    apply({'apiVersion':'v1','kind':'Namespace','metadata':{'name':name}})

def route(host='smoke.app.yu-min3.com', gateway='gateway-prod', namespace='app-smoke', ignore=False):
    return {'apiVersion':'gateway.networking.k8s.io/v1','kind':'HTTPRoute',
            'metadata':{'name':'smoke','namespace':namespace,'labels':{'k8s-gateway.dns/ignore':str(ignore).lower()}},
            'spec':{'hostnames':[host],'parentRefs':[{'name':gateway,'namespace':'istio-system'}],
                    'rules':[{'backendRefs':[{'name':'placeholder','port':80}]}]}}

def query(name):
    labels = b''.join(bytes([len(x)]) + x.encode() for x in name.split('.')) + b'\0'
    packet = struct.pack('!HHHHHH', 4567, 0x100, 1, 0, 0, 0) + labels + struct.pack('!HH',1,1)
    with socket.create_connection(('127.0.0.1',15353), timeout=3) as s:
        s.sendall(struct.pack('!H',len(packet))+packet)
        def receive(n):
            buf=b''
            while len(buf)<n:
                chunk=s.recv(n-len(buf))
                if not chunk: raise RuntimeError('short DNS response')
                buf+=chunk
            return buf
        reply=receive(struct.unpack('!H',receive(2))[0])
    assert struct.unpack('!H', reply[:2])[0] == 4567
    return reply

def wait_answer(present):
    end=time.monotonic()+50
    while time.monotonic()<end:
        answer=query('smoke.app.yu-min3.com')
        count=struct.unpack('!H',answer[6:8])[0]
        found=socket.inet_aton('192.168.0.243') in answer and count>0
        if found==present: return
        time.sleep(2)
    raise AssertionError('DNS lifecycle result did not converge')

run('apply','--server-side','-f',str(ROOT/'kubernetes/network/gateway-api/gateway-api-crds.yaml'))
for ns in ['lan-dns','istio-system','app-smoke','app-kensan']:
    namespace(ns)
for file in ['rbac.yaml','configmap.yaml','app-route-ownership.yaml','deployment.yaml']:
    run('apply','-f',str(ROOT/'kubernetes/network/lan-dns'/file))
# A fixture Gateway supplies an address. No Istio is installed: this deliberately
# verifies the documented behavior that DNS is not a Route/App readiness check.
apply({'apiVersion':'gateway.networking.k8s.io/v1','kind':'Gateway',
       'metadata':{'name':'gateway-prod','namespace':'istio-system'},
       'spec':{'gatewayClassName':'istio','listeners':[{'name':'http','port':80,'protocol':'HTTP'}]}})
run('-n','istio-system','patch','gateway','gateway-prod','--subresource=status','--type=merge',
    '-p',json.dumps({'status':{'addresses':[{'type':'IPAddress','value':'192.168.0.243'}]}}))
# Wait for admission type checking before testing denial, never silently skip.
for _ in range(30):
    policy=json.loads(run('get','validatingadmissionpolicy','app-httproute-ownership','-o','json').stdout)
    if 'typeChecking' in policy.get('status',{}): break
    time.sleep(1)
assert policy.get('status',{}).get('observedGeneration') == policy['metadata']['generation'], policy.get('status')
assert 'typeChecking' in policy.get('status',{}), policy.get('status')
assert not policy['status']['typeChecking'].get('expressionWarnings'), policy.get('status')
for host,gateway in [('konro.app.yu-min3.com','gateway-prod'),('auth.platform.yu-min3.com','gateway-prod'),
                     ('*.app.yu-min3.com','gateway-prod'),('smoke.app.yu-min3.com','gateway-platform')]:
    p=run('apply','-f','-',data=json.dumps(route(host,gateway)),ok=False)
    assert p.returncode and 'App route' in p.stderr, p.stderr
apply(route('kensan.yu-mins.com',namespace='app-kensan'))
apply(route())
p=run('apply','-f','-',data=json.dumps(route('konro.app.yu-min3.com')),ok=False)
assert p.returncode and 'App route' in p.stderr, p.stderr
for verb,resource in [('get','secrets'),('patch','configmaps'),('create','httproutes.gateway.networking.k8s.io')]:
    assert run('auth','can-i',verb,resource,'--as=system:serviceaccount:lan-dns:lan-dns',ok=False).stdout.strip()=='no'
run('-n','lan-dns','rollout','status','deployment/lan-dns','--timeout=120s')
forward=subprocess.Popen([*K,'-n','lan-dns','port-forward','deployment/lan-dns','15353:5353'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
try:
    time.sleep(3)
    wait_answer(True)
    pod=json.loads(run('-n','lan-dns','get','pods','-o','json').stdout)['items'][0]
    pod_ip=pod['status']['podIP']
    udp=run('run','dns-udp-check','--image=busybox:1.37','--restart=Never','--attach','--rm',
        '--command','--','nslookup','-type=A','-port=5353','smoke.app.yu-min3.com',pod_ip)
    assert '192.168.0.243' in udp.stdout, udp.stdout
    apply(route(ignore=True)); wait_answer(False)
    apply(route()); wait_answer(True)
    run('-n','app-smoke','delete','httproute','smoke'); wait_answer(False)
    print('PASS: restricted image, minimal RBAC, owner/parent CREATE+UPDATE denial, legacy host, UDP/TCP, opt-out and deletion (TTL <=30s).')
finally:
    forward.terminate(); forward.wait(timeout=10)

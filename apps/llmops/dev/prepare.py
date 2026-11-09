"""Generate private local credentials, CA, TLS keypair and realm outside Git."""
from pathlib import Path
import json,os,secrets,subprocess
root=Path(__file__).resolve().parents[3]
data=root/'temp/llmops-dev';data.mkdir(parents=True,exist_ok=True);os.chmod(data,0o700)
envpath=data/'dev.env'
keys=['DEV_S3_KEY','DEV_S3_SECRET','DEV_DB_SECRET','DEV_KEYCLOAK_DB_SECRET','DEV_CATALOG_SECRET','DEV_KEYCLOAK_ADMIN_SECRET']
if not envpath.exists():
 env={k:secrets.token_hex(24) for k in keys}
 env.update(DEV_UID=str(os.getuid()),DEV_DATA=str(data))
 envpath.write_text(''.join(f'{k}={v}\n' for k,v in env.items()));os.chmod(envpath,0o600)
else:env=dict(line.split('=',1) for line in envpath.read_text().splitlines())
(data/'init.sql').write_text("CREATE SCHEMA IF NOT EXISTS polaris_schema;\nCREATE USER keycloak WITH PASSWORD '"+env['DEV_KEYCLOAK_DB_SECRET']+"';\nCREATE DATABASE keycloak OWNER keycloak;\n");os.chmod(data/'init.sql',0o600)
realm=data/'realm';realm.mkdir(exist_ok=True)
(realm/'llmops-realm.json').write_text(json.dumps({'realm':'llmops','enabled':True,'sslRequired':'all'}));os.chmod(realm/'llmops-realm.json',0o600)
if not (data/'server.pem').exists():
 config=data/'tls.cnf';config.write_text('[req]\ndistinguished_name=dn\nprompt=no\n[dn]\nCN=id.localhost\n[server]\nsubjectAltName=DNS:id.localhost\nbasicConstraints=CA:FALSE\nkeyUsage=digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectKeyIdentifier=hash\nauthorityKeyIdentifier=keyid,issuer\n[ca]\nbasicConstraints=critical,CA:TRUE\nkeyUsage=critical,keyCertSign,cRLSign\nsubjectKeyIdentifier=hash\nauthorityKeyIdentifier=keyid:always\n')
 commands=[['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout','ca.key','-out','ca.pem','-days','30','-subj','/CN=LLMOps local development CA','-config','tls.cnf','-extensions','ca'],['openssl','req','-newkey','rsa:2048','-nodes','-keyout','server.key','-out','server.csr','-config','tls.cnf'],['openssl','x509','-req','-in','server.csr','-CA','ca.pem','-CAkey','ca.key','-CAcreateserial','-out','server.pem','-days','30','-extfile','tls.cnf','-extensions','server']]
 for command in commands:subprocess.run(command,cwd=data,check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
for path in data.glob('*'):
 if path.is_file():os.chmod(path,0o600)
print('Local credentials and CA prepared under ignored temp/llmops-dev/')

for name in ['ca.pem','server.pem']:os.chmod(data/name,0o644)

#!/usr/bin/env python3
"""Check production capture setup using temporary env files; never edits prod.env."""
from pathlib import Path
import os
import subprocess
import tempfile
ROOT=Path(__file__).resolve().parents[1]
def run(*args, env=None, ok=True):
    result=subprocess.run(args,cwd=ROOT,env=env,text=True,capture_output=True)
    assert (result.returncode==0)==ok,result.stdout+result.stderr
    return result
with tempfile.TemporaryDirectory(prefix='adcap-setup-') as work:
    path=Path(work)/'prod.env'
    template=(ROOT/'deploy/compose/prod.env.example').read_text()
    # Simulate an existing older file, preserving a literal secret verbatim.
    secret='ADMIN_API_TOKEN=keep-literal-$(must-not-execute)-secret'
    legacy='\n'.join(line for line in template.splitlines() if not line.startswith('CONTROLLER_CAPTURE_MODE='))
    legacy=legacy.replace('CONTROLLER_SERVICE_SECURITY_OPT=no-new-privileges:true','CONTROLLER_SERVICE_SECURITY_OPT=')
    legacy='\n'.join(secret if line.startswith('ADMIN_API_TOKEN=') else line for line in legacy.splitlines())
    path.write_text(legacy+'\n')
    env={k:v for k,v in os.environ.items() if not k.startswith(('CONTROLLER_','CAPTURE_','EDGE_','AD_PLATFORM_PUBLIC_BASE_URL','WIREGUARD_GATEWAY_','DATABASE_INCLUDE_SEEDS','API_GATEWAY_AUTO_SEED','PROD_HOST_OVERRIDE'))}
    env['CADDY_CERT_DIR']=str(Path(work)/'certs')
    run('make','setup-prod-env',f'PROD_ENV={path}','DOMAIN=ctf.example.test','SCHEME=https',env=env)
    assert 'CONTROLLER_CAPTURE_MODE=guarded\n' in path.read_text()
    assert 'CONTROLLER_SERVICE_SECURITY_OPT=no-new-privileges:true\n' in path.read_text()
    assert secret in path.read_text(),'setup changed secret'
    assert path.stat().st_mode&0o777==0o600
    run('scripts/validate-prod-compose-security.sh',str(path),env=env)
    run('scripts/validate-prod-env.sh',str(path),'base',env=env,ok=False)
    run('make','setup-prod-env',f'PROD_ENV={path}','DOMAIN=ctf.example.test','CAPTURE_MODE=disabled',env=env)
    run('make','setup-prod-env',f'PROD_ENV={path}','DOMAIN=ctf.example.test',env=env)
    assert path.read_text().count('CONTROLLER_CAPTURE_MODE=disabled\n')==1
    safe_config=path.read_text()
    # Both enforcement services must actually apply their policies on the host.
    for key, safe, unsafe in (
        ('CONTROLLER_ACCESS_MODE', 'host', 'dry-run'),
        ('WIREGUARD_GATEWAY_MODE', 'host', 'files'),
        ('WIREGUARD_GATEWAY_FIREWALL_BACKEND', 'nftables', 'none'),
    ):
        assert f'{key}={safe}\n' in safe_config
        path.write_text(safe_config.replace(f'{key}={safe}\n', f'{key}={unsafe}\n'))
        run('scripts/validate-prod-compose-security.sh',str(path),env=env,ok=False)
    path.write_text(safe_config)
    run('scripts/validate-prod-compose-security.sh',str(path),env=env)
    for seed_flag in ('API_GATEWAY_AUTO_SEED', 'DATABASE_INCLUDE_SEEDS'):
        assert f'{seed_flag}=false\n' in safe_config
        path.write_text(safe_config.replace(f'{seed_flag}=false\n', f'{seed_flag}=true\n'))
        run('scripts/validate-prod-env.sh',str(path),env=env,ok=False)
        run('scripts/validate-prod-compose-security.sh',str(path),env=env,ok=False)
    path.write_text(safe_config)
    inherited_env=env.copy()
    inherited_env['DATABASE_INCLUDE_SEEDS']='true'
    run('scripts/validate-prod-env.sh',str(path),env=inherited_env,ok=False)
    host_override=(ROOT/'deploy/compose/prod.host-enforcement.yml').read_text()
    api_gateway_env='  api-gateway:\n    extra_hosts:\n      - host.docker.internal:host-gateway\n    environment:\n'
    assert api_gateway_env in host_override
    for seed_flag in ('API_GATEWAY_AUTO_SEED', 'DATABASE_INCLUDE_SEEDS'):
        unsafe_override=Path(work)/'unsafe-host-override.yml'
        unsafe_override.write_text(host_override.replace(api_gateway_env,api_gateway_env+f'      {seed_flag}: "true"\n'))
        override_env=env.copy()
        override_env['PROD_HOST_OVERRIDE']=str(unsafe_override)
        run('scripts/validate-prod-compose-security.sh',str(path),env=override_env,ok=False)
    before=path.read_bytes()
    run('make','setup-prod-env',f'PROD_ENV={path}','DOMAIN=ctf.example.test','CAPTURE_MODE=unrestricted',env=env,ok=False)
    assert path.read_bytes()==before,'invalid mode changed configuration'
    path.write_text(template.replace('CONTROLLER_SERVICE_CAP_ADD=CHOWN,','CONTROLLER_SERVICE_CAP_ADD=SYS_ADMIN,CHOWN,'))
    run('scripts/validate-prod-env.sh',str(path),env=env,ok=False)
    path.write_text(template.replace('CONTROLLER_CAPTURE_MODE=guarded','CONTROLLER_CAPTURE_MODE=disabled').replace('CONTROLLER_SERVICE_CAP_ADD=CHOWN,','CONTROLLER_SERVICE_CAP_ADD=NET_RAW,CHOWN,'))
    run('scripts/validate-prod-env.sh',str(path),env=env,ok=False)
    path.write_text(template.replace('CONTROLLER_CAPTURE_MODE=guarded','CONTROLLER_CAPTURE_MODE=disabled').replace('CONTROLLER_SERVICE_CAP_DROP=ALL','CONTROLLER_SERVICE_CAP_DROP=none'))
    run('scripts/validate-prod-env.sh',str(path),env=env,ok=False)
print('capture setup: migration, overrides, secret preservation, compose guards, unsafe configurations passed')

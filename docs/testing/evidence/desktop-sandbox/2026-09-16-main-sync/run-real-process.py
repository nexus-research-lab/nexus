import os,json,subprocess,time
from pathlib import Path
a=Path('/private/tmp/nexus-sandbox-main-sync')
r=Path('/Users/berhand/program/go/pkg/mod/github.com/nexus-research-lab/nexus-agent-sdk-bridge@v0.1.34-0.20260916063139-6325d2acc450')
env=dict(os.environ,GOWORK='off',GOPROXY='off',GOCACHE='/private/tmp/nexus-sandbox-go-cache',NEXUS_CONFIG_DIR=str(a/'real-process-config'),NEXUS_SANDBOX_TEST_BINARY=str(a/'nxs'),NEXUS_SETTINGS_SANDBOX_LEGACY_TEST_BINARY=str(a/'nxs-before-settings'))
patterns={'real-current':'^Test(RequiredSandbox|FileSandbox|SearchSandbox|MediaFileSandbox|SkillFileSandbox|ContextFileSandbox|ProjectFileSandbox|ManagedPolicySandbox|SettingsFilesSandbox|SandboxResources)RealProcess$/^current$','real-legacy-settings':'^TestSettingsFilesSandboxRealProcess$/^legacy$'}
code=0
for name,pattern in patterns.items():
 cmd=['go','test','-json','-mod=readonly','-count=1','./client','-run',pattern]
 start=time.time()
 with (a/(name+'.jsonl')).open('w') as log: result=subprocess.run(cmd,cwd=r,env=env,stdout=log,stderr=subprocess.STDOUT)
 info={'command':cmd,'cwd':str(r),'environment':{k:v for k,v in env.items() if k in ['GOWORK','GOPROXY','GOCACHE','NEXUS_CONFIG_DIR','NEXUS_SANDBOX_TEST_BINARY','NEXUS_SETTINGS_SANDBOX_LEGACY_TEST_BINARY']},'exitCode':result.returncode,'seconds':round(time.time()-start,3)}
 (a/(name+'.result.json')).write_text(json.dumps(info,indent=2)+'\n');print(json.dumps(info),flush=True)
 code=code or result.returncode
raise SystemExit(code)

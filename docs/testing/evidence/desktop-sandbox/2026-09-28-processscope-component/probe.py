# Explicit native experiment; creates and removes only its unique user launchd job.
from pathlib import Path
import hashlib, json, os, platform, plistlib, subprocess, sys, tempfile, time, uuid

root = Path(tempfile.mkdtemp(prefix='nexus-launchd-coalition-'))
label = 'cn.nexus.sandbox-probe.' + uuid.uuid4().hex
domain = 'gui/' + str(os.getuid())
service = domain + '/' + label
source = Path(__file__).with_name('probe.c')
binary = str(root / 'probe')
subprocess.run(['clang','-Wall','-Wextra','-Werror',str(source),'-o',binary],check=True,capture_output=True,text=True,timeout=30)
stdout = root / 'worker.jsonl'
plist = root / 'probe.plist'
scenario = sys.argv[1] if len(sys.argv)>1 else 'plain'
assert scenario in ('plain','double-fork-exec','sandbox-double-fork-exec')
arguments=[binary,'worker' if scenario=='plain' else 'worker-exec']
if scenario.startswith('sandbox-'):
    profile='(version 1)(deny default)(allow process-exec)(allow process-fork)(allow process-info* (target same-sandbox))(allow mach-priv-task-port (target same-sandbox))(allow signal (target same-sandbox))(allow file-read*)(allow file-write* (subpath '+json.dumps(str(root))+'))'
    arguments=['/usr/bin/sandbox-exec','-p',profile,*arguments]
plist.write_bytes(plistlib.dumps({
    'Label': label, 'ProgramArguments': arguments,
    'RunAtLoad': True, 'KeepAlive': False, 'LaunchOnlyOnce': False,
    'AbandonProcessGroup': False, 'WorkingDirectory': str(root),
    'StandardOutPath': str(stdout), 'StandardErrorPath': str(root/'stderr.log'),
}))
report = {'scenario':scenario,'label':label, 'domain':domain, 'uid':os.getuid(), 'directory':str(root), 'events':[], 'passed':False, 'macOS':platform.mac_ver()[0], 'architecture':platform.machine(), 'kernel':platform.release(), 'sourceSHA256':hashlib.sha256(source.read_bytes()).hexdigest(), 'driverSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
loaded = False
child = None
control = None

def run(args):
    result = subprocess.run(args, text=True, capture_output=True, timeout=10)
    return {'exitCode':result.returncode,'stdout':result.stdout,'stderr':result.stderr}

def observe(stage):
    result = run([binary, 'usage', str(child['resource'])])
    report['events'].append({'stage':stage, **result})

try:
    control=subprocess.Popen([binary,'leaf'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    report['controlBefore']=run([binary,'inspect',str(control.pid)])
    observer = run([binary])
    report['observer'] = json.loads(observer['stdout'])
    result = run(['/bin/launchctl','bootstrap',domain,str(plist)])
    report['bootstrap'] = result
    if result['exitCode'] != 0:
        raise RuntimeError('isolated launchd bootstrap failed')
    loaded = True
    for attempt in range(40):
        lines = stdout.read_text().splitlines() if stdout.exists() else []
        records = [json.loads(line) for line in lines]
        child = next((record for record in records if record['kind']=='detached'),None)
        if child:
            break
        time.sleep(.1)
    if not child:
        raise RuntimeError('no detached fixture identity')
    report['worker'] = records
    assert child['auditResult']==0 and child['audit'][5]==child['pid']
    assert child['coalitionBytes']==40 and child['resource']>0
    leader=next(record for record in records if record['kind']=='root')
    assert child['resource']==leader['resource']
    assert child['resource']!=report['observer']['resource']
    control_identity=json.loads(report['controlBefore']['stdout'])
    assert control_identity['resource']!=child['resource'] and control_identity['auditResult']==0
    assert child['sid']!=leader['sid']
    time.sleep(2)
    observe('root_exited_detached_alive')
    report['inspectDetached']=run([binary,'inspect',str(child['pid'])])
    observed=json.loads(report['inspectDetached']['stdout'])
    assert observed['auditResult']==0 and observed['audit']==child['audit']
    report['members']=run([binary,'members',str(child['resource'])])
    members=json.loads(report['members']['stdout'])['members']
    assert len(members)==1 and members[0]['audit']==child['audit']
    wrong=members[0]['audit'].copy();wrong[7]^=0x40000000
    report['staleIdentitySignal']=run([binary,'signal',*[str(value) for value in wrong]])
    assert json.loads(report['staleIdentitySignal']['stdout'])['result']==3
    observe('after_rejected_stale_identity')
    details=run(['/bin/launchctl','print',service])
    report['jobState']={'exitCode':details['exitCode'], 'selected':[
        line.strip() for line in details['stdout'].splitlines()
        if line.strip().startswith(('state =','pid =','last exit code =','resource coalition =','jetsam coalition =','ID =','active count ='))]}
    result=run(['/bin/launchctl','bootout',service])
    report['bootout']=result
    if result['exitCode']==0: loaded=False
    observe('after_bootout')
    time.sleep(.5)
    observe('bootout_settled')
    report['signal']=run([binary,'signal',*[str(value) for value in members[0]['audit']]])
    assert json.loads(report['signal']['stdout'])['result'] in (0,3)
    for attempt in range(20):
        usage=run([binary,'usage',str(child['resource'])])
        counters=json.loads(usage['stdout'])
        # Counters alone are diagnostic: an in-flight fork may be active before
        # its task has been adopted. Require the owned coalition to be reaped.
        if counters.get('result')==-1 and counters.get('errno')==3:
            report['events'].append({'stage':'after_exact_child_signal',**usage});break
        time.sleep(.1)
    else:raise RuntimeError('fixture coalition reaping was not confirmed')
    assert control.poll() is None
    report['controlAfter']=run([binary,'inspect',str(control.pid)])
    assert json.loads(report['controlAfter']['stdout'])['audit']==control_identity['audit']
    report['passed']=True
finally:
    if child and child['auditResult']==0 and child['audit'][5]==child['pid']:
        report['cleanupSignal']=run([binary,'signal',*[str(value) for value in child['audit']]])
    if loaded:
        report['cleanupBootout']=run(['/bin/launchctl','bootout',service])
    if control is not None:
        control.terminate()
        report['controlExitCode']=control.wait(timeout=5)
    report['finalJobLookup']=run(['/bin/launchctl','print',service])
    if report['finalJobLookup']['exitCode']==0:report['passed']=False
    (root/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,ensure_ascii=False))
    if not report['passed']:
        raise SystemExit(1)

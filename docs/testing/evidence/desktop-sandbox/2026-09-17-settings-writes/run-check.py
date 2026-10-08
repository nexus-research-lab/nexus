from pathlib import Path
import json, subprocess, sys, time
name, cwd, *command = sys.argv[1:]
root = Path(__file__).resolve().parent
started = time.time()
with (root / (name+'.stdout.log')).open('w') as out, (root / (name+'.stderr.log')).open('w') as err:
    process = subprocess.run(command, cwd=cwd, stdout=out, stderr=err)
record = {'name':name, 'cwd':cwd, 'command':command, 'exitCode':process.returncode, 'seconds':round(time.time()-started,3)}
if '-json' in command:
    events=[]
    for line in (root/(name+'.stdout.log')).read_text().splitlines():
        try: events.append(json.loads(line))
        except json.JSONDecodeError: pass
    record['passedTests']=[{'package':e.get('Package'),'test':e['Test']} for e in events if e.get('Action')=='pass' and e.get('Test')]
    record['failedTests']=[{'package':e.get('Package'),'test':e.get('Test')} for e in events if e.get('Action')=='fail']
    record['skippedTests']=[{'package':e.get('Package'),'test':e.get('Test')} for e in events if e.get('Action')=='skip']
(root/(name+'.json')).write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps({k:v for k,v in record.items() if k not in ['passedTests','skippedTests']} | {'passed':len(record.get('passedTests',[])), 'skipped':len(record.get('skippedTests',[]))}))
if process.returncode:
    print((root/(name+'.stderr.log')).read_text()[-6000:])
    print((root/(name+'.stdout.log')).read_text()[-6000:])
sys.exit(process.returncode)

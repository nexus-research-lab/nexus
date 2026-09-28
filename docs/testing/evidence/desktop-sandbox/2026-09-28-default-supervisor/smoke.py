import os,sys,tempfile,subprocess,time,socket,urllib.request,json,pathlib,signal
repo=sys.argv[1]
bundle=pathlib.Path('/tmp/nexus-default-supervisor/Nexus.app')
run=pathlib.Path(tempfile.mkdtemp(prefix='nexus-default-supervisor-run-'))
(run/'home').mkdir()
sock=socket.socket();sock.bind(('127.0.0.1',0));port=sock.getsockname()[1];sock.close()
env={'PATH':os.environ['PATH'],'HOME':str(run/'home'),'TMPDIR':os.environ.get('TMPDIR','/tmp'),'NEXUS_APP_ROOT':repo,'NEXUS_STATE_ROOT':str(run/'state'),'NEXUS_APP_MODE':'desktop','PORT':str(port),'HOST':'127.0.0.1','LOG_STDOUT':'true','NEXUS_REMOTE_URL':'','NEXUS_RELAY_URL':''}
exe=bundle/'Contents/MacOS/nexus-server'
log=(run/'sidecar.log').open('w')
p=subprocess.Popen([str(exe)],cwd=run,env=env,stdout=log,stderr=subprocess.STDOUT)
try:
 deadline=time.monotonic()+45
 while time.monotonic()<deadline:
  if p.poll() is not None:raise RuntimeError('sidecar exited '+str(p.returncode))
  try:
   with urllib.request.urlopen(f'http://127.0.0.1:{port}/nexus/v1/health',timeout=.5) as r:
    if r.status==200 and json.load(r)['data']['status']=='ok':break
  except Exception:time.sleep(.15)
 else:raise RuntimeError('health timeout')
 duplicate=subprocess.run([str(exe)],cwd=run,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=15)
 assert duplicate.returncode != 0 and p.poll() is None, (duplicate.returncode, duplicate.stdout[-2000:])
 with urllib.request.urlopen(f'http://127.0.0.1:{port}/nexus/v1/health',timeout=1) as r: assert r.status==200 and json.load(r)['data']['status']=='ok'
 print(json.dumps({'result':'PASS','phase':'duplicate-sidecar-denied'}),flush=True)
 assert (run/'state/app/processes').is_dir()
 print(json.dumps({'result':'PASS','phase':'default-sidecar-start','health':200,'process_registry':True,'run':str(run)},ensure_ascii=False),flush=True)
finally:
 if p.poll() is None:p.send_signal(signal.SIGTERM)
 try:p.wait(timeout=15)
 except subprocess.TimeoutExpired:p.kill();p.wait();raise
 log.close()
 print(json.dumps({'phase':'sidecar-exit','code':p.returncode,'log':str(run/'sidecar.log')}),flush=True)
 if p.returncode:print((run/'sidecar.log').read_text()[-6000:])

manifest=bundle/'Contents/Resources/runtime-bootstrap.json'
original=manifest.read_bytes()
try:
 bad=json.loads(original);bad['sha256']='0'*64;manifest.write_text(json.dumps(bad))
 denied=subprocess.run([str(exe)],cwd=run,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=30)
 assert denied.returncode != 0 and b'bootstrap binary digest mismatch' in denied.stdout, denied.stdout[-2000:]
 print(json.dumps({'result':'PASS','phase':'tampered-helper-manifest-denied','code':denied.returncode}),flush=True)
finally:manifest.write_bytes(original)

"""Compile changed SDK/Bridge test packages; never execute foreign binaries."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parent
repos = {
    "sdk": (Path("/private/tmp/nexus-sandbox-capabilities.60SUhq/sdk"), ["./cmd/nxs", "./protocol", "./internal/tool/executor", "./internal/tool/builtin/glob", "./internal/tool/builtin/grep", "./internal/tool/builtin/ripgrep", "./internal/tool/builtin/file"]),
    "bridge": (Path("/private/tmp/nexus-sandbox-capabilities.60SUhq/bridge"), ["./client", "./protocol"]),
}
results = []
for platform in ("windows", "linux"):
    for name, (repo, packages) in repos.items():
        output = root / (name + "-" + platform)
        output.mkdir(exist_ok=True)
        env = dict(os.environ, GOWORK="off", GOPROXY="off", GOCACHE="/private/tmp/nexus-sandbox-review-gocache", GOOS=platform, GOARCH="amd64", CGO_ENABLED="0")
        command = ["go", "test", "-c", "-mod=readonly", "-o", str(output), *packages]
        log = root / (name + "-" + platform + "-compile.log")
        with log.open("w") as stream:
            result = subprocess.run(command, cwd=repo, env=env, stdout=stream, stderr=subprocess.STDOUT, timeout=180)
        record = dict(repository=name, revision=subprocess.check_output(["git","rev-parse","HEAD"],cwd=repo,text=True).strip(), GOOS=platform, GOARCH="amd64", CGO_ENABLED="0", packages=packages, command=command, log=log.name, exitCode=result.returncode, nativeExecution=False,
                      artifacts=[dict(path=str(f), sha256=hashlib.sha256(f.read_bytes()).hexdigest()) for f in sorted(output.iterdir()) if f.is_file()])
        results.append(record)
        (root / "cross-compilation.json").write_text(json.dumps(results,indent=2)+"\n")
        print(name,platform,"compile exit",result.returncode,flush=True)
        if result.returncode: raise SystemExit(result.returncode)

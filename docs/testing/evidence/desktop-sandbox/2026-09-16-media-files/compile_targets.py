import json
import os
import pathlib
import subprocess

root = pathlib.Path(__file__).parent
report = {"compileOnly": True, "nativeAcceptance": False, "checks": []}
targets = {
    "sdk": ("/private/tmp/nexus-sandbox-capabilities.60SUhq/sdk", ["./cmd/nxs", "./protocol", "./internal/environment/media/vision", "./internal/tool/builtin/viewimage", "./internal/tool/executor", "./internal/agent/runtime"]),
    "bridge": ("/private/tmp/nexus-sandbox-capabilities.60SUhq/bridge", ["./client", "./protocol"]),
}
for platform in ["windows", "linux"]:
    for name, (cwd, packages) in targets.items():
        label = name + "-" + platform
        output = root / "compile" / label
        output.mkdir(parents=True, exist_ok=True)
        command = ["go", "test", "-c", "-mod=readonly", "-o", str(output) + "/", *packages]
        env = dict(os.environ, GOOS=platform, GOARCH="amd64", CGO_ENABLED="0", GOWORK="off", GOPROXY="off", GOCACHE="/private/tmp/nexus-sandbox-review-gocache")
        with (root / (label + ".log")).open("w") as log:
            result = subprocess.run(command, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, timeout=240)
        report["checks"].append({"name": label, "cwd": cwd, "command": command, "exitCode": result.returncode})
        (root / "compile-report.json").write_text(json.dumps(report, indent=2) + "\n")
        print(label, result.returncode, flush=True)
        if result.returncode:
            raise SystemExit(result.returncode)

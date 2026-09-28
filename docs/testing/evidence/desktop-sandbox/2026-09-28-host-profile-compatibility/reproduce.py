"""Non-normative macOS Seatbelt nesting experiment; no product/model launch."""
import json
import pathlib
import platform
import subprocess
import tempfile


def run(name, command):
    result = subprocess.run(command, capture_output=True, text=True, timeout=10)
    return dict(name=name, code=result.returncode, stdout=result.stdout, stderr=result.stderr)


with tempfile.TemporaryDirectory(prefix="nexus-host-profile-") as temporary:
    root = pathlib.Path(temporary).resolve()
    private = root / "app"
    private.mkdir()
    secret = private / "sentinel"
    secret.write_text("unchanged")
    broad = "(version 1)(allow default)"
    protected = broad + "(deny file-read* (subpath " + json.dumps(str(private)) + "))"
    stronger = protected + "(deny file-write* (subpath " + json.dumps(str(root / "extra")) + "))"
    binary = "/usr/bin/sandbox-exec"
    cases = [
        run("unrestricted_nested", [binary, "-p", broad, binary, "-p", broad, "/usr/bin/true"]),
        run("outer_read_denied", [binary, "-p", protected, "/bin/cat", str(secret)]),
        run("same_profile_nested", [binary, "-p", protected, binary, "-p", protected, "/usr/bin/true"]),
        run("relaxed_profile_nested", [binary, "-p", protected, binary, "-p", broad, "/usr/bin/true"]),
        run("stricter_profile_nested", [binary, "-p", protected, binary, "-p", stronger, "/usr/bin/true"]),
    ]
    assert secret.read_text() == "unchanged"
    print(json.dumps(dict(os=platform.mac_ver()[0], machine=platform.machine(), cases=cases), indent=2))
    expected = dict(unrestricted_nested=0, outer_read_denied=1, same_profile_nested=0,
                    relaxed_profile_nested=71, stricter_profile_nested=71)
    if any(case["code"] != expected[case["name"]] for case in cases):
        raise SystemExit("Observation differs: inspect actual platform behavior before choosing a design.")

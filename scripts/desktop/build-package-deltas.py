"""为上一正式版本的同架构安装包生成差分，并在上传前重建和校验完整包。"""
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


def gh(*args):
    return json.loads(subprocess.check_output(["gh", *args]))


def sha(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def version(tag):
    return tuple(map(int, tag.lstrip("v").split(".")))


def build(directory, tag, temp):
    releases = gh("release", "list", "--exclude-drafts", "--exclude-pre-releases",
                  "--limit", "100", "--json", "tagName")
    earlier = [r["tagName"] for r in releases if re.fullmatch(r"v\d+\.\d+\.\d+", r["tagName"])
               and version(r["tagName"]) < version(tag)]
    if not earlier:
        return
    previous = max(earlier, key=version)
    assets = gh("release", "view", previous, "--json", "assets")["assets"]
    tool = temp / "nexus-delta"
    subprocess.run(["go", "build", "-o", tool, "./cmd/nexus-delta"], check=True)
    for prefix, ext in [("Nexus-macos-arm64-", ".dmg"), ("Nexus-macos-intel-", ".dmg"), ("NexusSetup-", ".exe")]:
        pattern = re.escape(prefix + previous[1:] + "-") + r"\d+" + re.escape(ext)
        bases = [a["name"] for a in assets if re.fullmatch(pattern, a["name"])]
        targets = list(directory.glob(prefix + tag[1:] + "-*" + ext))
        if len(bases) != 1 or len(targets) != 1:
            print("Skipping optional delta for", prefix, bases, targets)
            continue
        old = temp / bases[0]
        new = targets[0]
        subprocess.run(["gh", "release", "download", previous, "-p", old.name, "-D", temp], check=True)
        delta = temp / (new.name + ".from-" + sha(old) + ".delta")
        subprocess.run([tool, "create", old, new, delta], check=True)
        if delta.stat().st_size >= new.stat().st_size:
            print("Full package is smaller:", new.name)
            continue
        rebuilt = temp / ("rebuilt-" + new.name)
        subprocess.run([tool, "apply", old, delta, rebuilt], check=True)
        if sha(rebuilt) != sha(new):
            raise ValueError("Rebuilt package mismatch: " + new.name)
        (directory / (delta.name + ".sha256")).write_text(sha(delta) + "  " + delta.name + "\n")
        shutil.move(delta, directory / delta.name)


if __name__ == "__main__":
    with tempfile.TemporaryDirectory() as temp:
        build(Path(sys.argv[1]).resolve(), sys.argv[2], Path(temp))

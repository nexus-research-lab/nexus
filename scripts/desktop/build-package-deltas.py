"""从上一正式版的已安装应用目录生成差分，用与客户端相同的命令重建并验证。"""
import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile


def run(*args):
    subprocess.run(args, check=True)


def sha(path):
    with path.open("rb") as file:
        return hashlib.file_digest(file, "sha256").hexdigest()


@contextmanager
def mounted(dmg, root):
    root.mkdir()
    run("hdiutil", "attach", dmg, "-readonly", "-nobrowse", "-mountpoint", root, "-quiet")
    try:
        yield root / "Nexus.app"
    finally:
        run("hdiutil", "detach", root, "-quiet")


def create(tool, old, new, full, previous, current, tmp):
    patch = tmp / f"{full.name}.from-{previous}.delta"
    rebuilt = tmp / ("rebuilt.app" if full.suffix == ".dmg" else "rebuilt")
    run(tool, "create", previous, current, old, new, patch)
    run(tool, "apply", previous, current, old, patch, rebuilt)
    if full.suffix == ".dmg":
        run("codesign", "--verify", "--deep", "--strict", rebuilt)
        run("spctl", "--assess", "--type", "execute", rebuilt)
    else:
        install(rebuilt / "delta-install.exe", old)
        for item in new.rglob("*"):
            if item.is_file() and item.name != "delta-install.exe" and sha(old / item.relative_to(new)) != sha(item):
                raise ValueError(f"Installed file mismatch: {item}")
    print(f"{full.name}: {full.stat().st_size} -> {patch.stat().st_size} bytes", flush=True)
    if patch.stat().st_size < full.stat().st_size:
        target = full.parent / patch.name
        shutil.copyfile(patch, target)
        target.with_suffix(".delta.sha256").write_text(f"{sha(target)}  {target.name}\n")


def install(installer, path):
    env = {k: v for k, v in os.environ.items() if k != "GH_TOKEN" and not k.startswith("NEXUS_WINDOWS_SIGNING")}
    subprocess.run([installer, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", f"/DIR={path}"], check=True, env=env)


def build(args, tmp):
    full = args.package.resolve()
    match = re.fullmatch(r"(NexusSetup-|Nexus-macos-(?:arm64|intel)-)(\d+\.\d+\.\d+)-(\d+)(\.exe|\.dmg)", full.name)
    if not match:
        raise ValueError(full.name)
    prefix, version, build_no, ext = match.groups()
    if args.previous:
        old_package = args.previous.resolve()
    else:
        releases = json.loads(subprocess.check_output(["gh", "release", "list", "--exclude-drafts", "--exclude-pre-releases", "--limit", "100", "--json", "tagName"]))
        number = lambda v: tuple(map(int, v.split(".")))
        versions = [r["tagName"][1:] for r in releases if re.fullmatch(r"v\d+\.\d+\.\d+", r["tagName"])]
        prev = max((v for v in versions if number(v) < number(version)), default=None, key=number)
        if not prev:
            return
        assets = json.loads(subprocess.check_output(["gh", "release", "view", "v"+prev, "--json", "assets"]))["assets"]
        names = [a["name"] for a in assets if re.fullmatch(re.escape(prefix + prev + "-") + r"\d+" + re.escape(ext), a["name"])]
        if len(names) != 1:
            print("No unambiguous previous platform package")
            return
        old_package = tmp / names[0]
        run("gh", "release", "download", "v"+prev, "-D", tmp, "-p", names[0], "-p", names[0]+".sha256")
        if sha(old_package) != Path(str(old_package)+".sha256").read_text(encoding="utf-8-sig").split()[0].lower():
            raise ValueError("Previous release checksum mismatch")
    previous = old_package.name.removeprefix(prefix).removesuffix(ext)
    current = f"{version}-{build_no}"
    tool = tmp / ("nexus-delta.exe" if ext == ".exe" else "nexus-delta")
    run("go", "build", "-o", tool, "./cmd/nexus-delta")
    if ext == ".dmg":
        with mounted(old_package, tmp/"old") as old, mounted(full, tmp/"new") as new:
            create(tool, old, new, full, previous, current, tmp)
    else:
        old = tmp/"installed"
        install(old_package, old)
        create(tool, old, args.windows_tree.resolve(), full, previous, current, tmp)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("package", type=Path)
    parser.add_argument("--previous", type=Path)
    parser.add_argument("--windows-tree", type=Path)
    with tempfile.TemporaryDirectory() as tmp:
        build(parser.parse_args(), Path(tmp))

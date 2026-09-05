#!/usr/bin/env python3
"""Build and drive an isolated native frontend verification application.

INPUT: Current macOS/Web sources and an output directory.
OUTPUT: A uniquely identified QA application plus native UI evidence.
POS: Maintainer UI harness; does not start the product AppDelegate or sidecar.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import queue
import socket
import subprocess
import threading
import time
from urllib.parse import urlencode
from urllib.request import urlopen
import uuid

ROOT = Path(__file__).resolve().parents[2]


def build(output: Path) -> dict:
    if platform.system() != "Darwin":
        raise RuntimeError("The native UI harness requires macOS with an active graphical session")
    run_id = uuid.uuid4().hex
    run = output / run_id
    contents = run / "Nexus Frontend QA.app" / "Contents"
    executable = contents / "MacOS" / "NexusFrontendQA"
    executable.parent.mkdir(parents=True)
    bundle_id = "com.nexus.frontend-qa." + run_id
    with (contents / "Info.plist").open("wb") as handle:
        plistlib.dump({"CFBundleIdentifier": bundle_id, "CFBundleExecutable": executable.name,
                      "CFBundleName": "Nexus Frontend QA", "CFBundlePackageType": "APPL",
                      "CFBundleVersion": "1", "CFBundleShortVersionString": "0.0.0",
                      "LSMinimumSystemVersion": "14.0", "NSPrincipalClass": "NSApplication",
                      "NSHighResolutionCapable": True,
                      "NSAppTransportSecurity": {"NSAllowsLocalNetworking": True}}, handle)
    sources = sorted(path for path in (ROOT / "desktop/macos/Sources/NexusDesktop").rglob("*.swift")
                     if path.name != "EntryPoint.swift")
    sources.append(ROOT / "desktop/macos/UITests/FrontendHarness.swift")
    sdk = subprocess.check_output(["xcrun", "--sdk", "macosx", "--show-sdk-path"], text=True).strip()
    command = ["xcrun", "swiftc", "-swift-version", "5", "-parse-as-library", "-module-name", "NexusFrontendQA",
               "-target", platform.machine() + "-apple-macosx14.0", "-sdk", sdk,
               "-module-cache-path", str(output / "module-cache"), "-o", str(executable), *map(str, sources)]
    with (run / "build.log").open("w") as log:
        result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f"Native harness compilation failed; inspect {run / 'build.log'}")
    subprocess.run(["codesign", "--force", "--sign", "-", str(contents.parent)], check=True, capture_output=True)
    # Record both compiled native sources and the frontend served by Vite. This
    # evidence is for the working tree, which can differ from the current commit.
    inputs = sources + sorted((ROOT / "web/src").rglob("*")) + sorted((ROOT / "web/browser-tests").glob("native-ui-*")) + [
        Path(__file__).resolve(), ROOT / "web/app.html",
        ROOT / "web/vite.config.ts", ROOT / "web/package-lock.json", ROOT / "web/pnpm-lock.yaml",
        ROOT / "web/package.json", ROOT / "web/ui-gallery.html",
    ]
    manifest = {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
                for path in inputs if path.is_file()}
    (run / "source-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    settings = {"app": str(contents.parent), "executable": str(executable), "bundle_id": bundle_id,
                "output": str(run), "state_root": str(run / "state")}
    (run / "harness.json").write_text(json.dumps(settings, indent=2) + "\n")
    return settings


def require(condition, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def eventually(observe, description: str, timeout: float = 15):
    deadline = time.monotonic() + timeout
    last = None
    while time.monotonic() < deadline:
        last = observe()
        if last:
            return last
        time.sleep(0.1)
    raise AssertionError(f"Timed out: {description}; last result: {last!r}")


class NativeClient:
    """Serialize commands: WKWebView evaluation may complete asynchronously."""

    def __init__(self, settings: dict, environment: dict):
        self.settings = settings
        self.responses = queue.Queue()
        self.sequence = 0
        self.log = (Path(settings["output"]) / "native.log").open("w")
        self.process = subprocess.Popen([settings["executable"]], env=environment, stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE, stderr=self.log, text=True, bufsize=1)
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()
        try:
            self._response(0)
        except Exception:
            self.close()
            raise

    def _read(self):
        for line in self.process.stdout:
            self.log.write(line)
            self.log.flush()
            if line.startswith("NEXUS_UI_HARNESS "):
                self.responses.put(json.loads(line.removeprefix("NEXUS_UI_HARNESS ")))
        self.responses.put({"error": "Native process closed its output"})

    def _response(self, sequence):
        try:
            value = self.responses.get(timeout=30)
        except queue.Empty as error:
            raise RuntimeError("Native command timed out; inspect native.log") from error
        if "error" in value:
            raise RuntimeError(value["error"])
        require(value["id"] == sequence, f"Unexpected native response: {value}")
        return value["result"]

    def command(self, operation: str, **arguments):
        self.sequence += 1
        self.process.stdin.write(json.dumps({"id": self.sequence, "op": operation, **arguments}) + "\n")
        self.process.stdin.flush()
        return self._response(self.sequence)

    def evaluate(self, expression: str):
        return self.command("evaluate", script=f"(() => {{ return ({expression}); }})()")

    def wait(self, expression: str, description: str):
        return eventually(lambda: self.evaluate(expression), description)

    def activate(self):
        # LaunchServices activation is necessary for AppKit to route trusted
        # keyboard/pointer events into WKWebView; NSApp.activate alone is weaker.
        subprocess.run(["open", "-a", self.settings["app"]], check=True, timeout=10)
        eventually(lambda: self.command("status")["key"], "QA window activation")

    def key(self, characters: str, code: int, modifiers=()):
        self.command("key", characters=characters, key_code=code, modifiers=list(modifiers))

    def focus(self, element: str):
        self.evaluate(f"(() => {{ const e = {element}; e.scrollIntoView({{block:'center'}}); e.focus(); return true; }})()")
        self.wait(f"document.activeElement === {element}", "fixture focus precondition")

    def click(self, element: str, count: int = 1):
        point = self.evaluate(f"(() => {{ const e = {element}; e.scrollIntoView({{block:'center'}}); "
                              "const r=e.getBoundingClientRect(); return {x:r.x+r.width/2,y:r.y+r.height/2}; })()")
        self.command("click", count=count, **point)

    def snapshot(self, name: str):
        # Geometry exists at an overlay's transparent first animation frame.
        # Wait for finite UI animation and compositor frames without freezing
        # the theme's intentional infinite decoration or a busy spinner.
        self.wait("document.getAnimations().every(a => a.playState !== 'running' || "
                  "a.effect?.getComputedTiming().iterations === Infinity)", "snapshot animation settlement")
        self.evaluate("(() => { window.qaSnapshotReady=false; requestAnimationFrame(() => "
                      "requestAnimationFrame(() => { window.qaSnapshotReady=true; })); return true; })()")
        self.wait("window.qaSnapshotReady === true", "snapshot paint frames")
        return self.command("snapshot", name=name)

    def close(self):
        if self.process.poll() is None:
            try:
                self.command("quit")
                self.process.wait(timeout=5)
            except (RuntimeError, OSError, subprocess.TimeoutExpired):
                self.process.terminate()
                try:
                    self.process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    self.process.kill()
                    self.process.wait(timeout=5)
        self.reader.join(timeout=2)
        self.log.close()


def button(name: str) -> str:
    return ("[...document.querySelectorAll('button')].find(e => e.getAttribute('aria-label') === "
            f"{json.dumps(name)} || e.innerText.trim() === {json.dumps(name)})")


def inside(element: str) -> str:
    return (f"(() => {{ const e = {element}; if (!e) return false; const r=e.getBoundingClientRect(); "
            "return r.width>0 && r.height>0 && r.left>=-1 && r.top>=-1 && r.right<=innerWidth+1 && r.bottom<=innerHeight+1; })()")


def run_case(client: NativeClient, theme: str, locale: str, width: int) -> dict:
    name = f"{theme}-{locale}-{width}"
    copy = lambda en, zh: en if locale == "en" else zh
    select = button(copy("Choose model", "选择模型"))
    listbox = "document.querySelector('[role=listbox]')"
    dialog = "document.querySelector('[role=dialog]')"
    opener = button(copy("Open standard dialog", "打开标准弹窗"))
    close = button(copy("Close", "关闭"))
    confirm = button(copy("Confirm", "确认"))
    nested = button(copy("Open nested prompt", "打开嵌套确认"))
    client.command("resize", width=width, height=820 if width > 360 else 640)
    client.command("route", query=urlencode({"theme": theme, "locale": locale, "section": "foundation", "qa_case": name}))
    client.wait(f"new URL(location.href).searchParams.get('qa_case') === {json.dumps(name)} && "
                f"document.querySelector('main')?.dataset.galleryTheme === {json.dumps(theme)} && "
                f"document.querySelector('main')?.dataset.galleryLocale === {json.dumps(locale)} && !!({select})",
                f"current route {name}")
    client.wait("document.readyState === 'complete' && document.fonts.status === 'loaded'", "styles and local fonts ready")
    client.activate()
    client.evaluate("(() => { window.qaEvents=[]; window.qaErrors=[]; window.qaMarker='preserve-after-resume'; "
                    "for (const type of ['click','keydown','input']) document.addEventListener(type,e=>qaEvents.push({type,key:e.key ?? null,trusted:e.isTrusted}),true); "
                    "window.addEventListener('error',e=>qaErrors.push(e.message)); "
                    "window.addEventListener('unhandledrejection',e=>qaErrors.push(String(e.reason))); return true; })()")
    status = client.command("status")
    require(status["webview"][0] == width, f"Unexpected native width: {status}")
    require(abs(status["controls_center"][1] - 24) <= 1 and status["controls_inset"] > 60,
            f"Native controls metrics changed: {status}")
    client.wait("document.querySelector('main').scrollWidth <= innerWidth+1", "Gallery horizontal bounds")
    primary = button(copy("New conversation", "新建会话"))
    metrics = client.evaluate(f"(() => {{const e={primary}, s=getComputedStyle(e); return "
                              "{height:e.getBoundingClientRect().height,font:s.fontSize,weight:s.fontWeight,gap:s.columnGap};})()")
    require(metrics == {"height": 36, "font": "14px", "weight": "500", "gap": "8px"}, f"Button metrics: {metrics}")
    field = "document.querySelector('#gallery-name')"
    field_metrics = client.evaluate(f"(() => {{const e={field},s=getComputedStyle(e); "
                                    "return {height:e.getBoundingClientRect().height,font:s.fontSize,weight:s.fontWeight};})()")
    require(field_metrics == {"height": 36, "font": "14px", "weight": "400"}, f"Input metrics: {field_metrics}")
    client.click(field)
    client.wait(f"document.activeElement === {field}", "native pointer focuses input")
    client.evaluate(f"({field}).setSelectionRange(0, ({field}).value.length)")
    client.key("q", 12)
    client.wait(f"({field}).value === 'q'", "native text insertion")
    client.snapshot(name + "-fields")

    client.evaluate(f"(() => {{const e={select}; e.focus({{preventScroll:true}}); "
                    "e.closest('main').scrollTop += e.getBoundingClientRect().bottom-(innerHeight-16); return true;})()")
    client.wait(f"Math.abs(({select}).getBoundingClientRect().bottom-(innerHeight-16)) <= 1", "bottom-edge anchor")
    client.key("\r", 36)
    client.wait(inside(listbox) + f" && ({listbox}).dataset.placement === 'top'", "select collision placement")
    require(client.evaluate("document.querySelector('[role=option][disabled]') !== null"), "Missing disabled option")
    client.snapshot(name + "-select")
    client.key("\uf701", 125)
    client.key("\r", 36)
    client.wait(f"!({listbox}) && ({select}).innerText.includes({json.dumps(copy('Default conversation model', '默认对话模型'))})",
                "keyboard selection skips unavailable option")
    client.click(select)
    client.wait(inside(listbox), "native pointer reopens select")
    client.key("\x1b", 53)
    client.wait(f"!({listbox})", "Escape dismisses select")

    overflow = client.evaluate("document.body.style.overflow")
    client.focus(opener)
    client.key("\r", 36)
    client.wait(inside("document.querySelector('.dialog-shell')") + f" && document.activeElement === {close}", "dialog autofocus and bounds")
    require(client.evaluate("document.body.style.overflow === 'hidden'"), "Dialog did not lock scrolling")
    client.focus(confirm)
    client.key("\t", 48, ["option"])
    client.wait(f"document.activeElement === {close}", "forward focus trap")
    client.key("\t", 48, ["option", "shift"])
    client.wait(f"document.activeElement === {confirm}", "reverse focus trap")
    client.snapshot(name + "-dialog")
    modal_select = button(copy("Model inside dialog", "弹窗内模型"))
    client.click(modal_select)
    client.wait(inside(listbox), "select inside native modal")
    require(client.evaluate(f"(() => {{ const e={listbox},r=e.getBoundingClientRect(); "
                            "return e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));})()"),
            "Select is below the modal hit layer")
    client.key("\x1b", 53)
    client.wait(f"!({listbox}) && !!({dialog}) && document.activeElement === {modal_select}", "topmost Escape and select focus")
    client.focus(nested)
    client.key("\r", 36)
    client.wait("document.querySelectorAll('[role=dialog]').length === 2 && document.activeElement.tagName === 'INPUT'",
                "nested prompt autofocus")
    client.key("\x1b", 53)
    client.wait(f"document.querySelectorAll('[role=dialog]').length === 1 && document.activeElement === {nested}", "nested focus return")
    client.key("\x1b", 53)
    client.wait(f"!({dialog}) && document.activeElement === {opener}", "outer focus return")
    require(client.evaluate("document.body.style.overflow") == overflow, "Scroll lock was not restored")

    before_resume = client.command("timeline")
    count = lambda events, event: sum(entry["event"] == event for entry in events)
    client.command("hide")
    require(not client.command("status")["visible"], "Window failed to hide")
    # The production host coalesces foreground probes for five seconds. Keep the
    # window hidden past that interval so each case tests an actual new probe.
    time.sleep(5.1)
    client.command("show")
    client.activate()
    client.wait("window.qaMarker === 'preserve-after-resume'", "window restoration preserves document")
    eventually(lambda: count(client.command("timeline"), "webview.resume_check_ready") >
               count(before_resume, "webview.resume_check_ready"), "native resume readiness")
    require(count(client.command("timeline"), "webview.load_begin") == count(before_resume, "webview.load_begin"),
            "Healthy window restoration unexpectedly reloaded the document")
    events = client.evaluate("window.qaEvents")
    require(all(event["trusted"] for event in events), "Unexpected untrusted DOM input")
    require({event["type"] for event in events} >= {"click", "keydown", "input"}, "Missing native event evidence")
    require(client.evaluate("window.qaErrors") == [], "Unexpected browser error")
    return {"case": name, "status": status, "button_metrics": metrics, "field_metrics": field_metrics,
            "native_events": events, "passed": True}


def run_app_shell_case(client: NativeClient, theme: str, locale: str, width: int) -> dict:
    name = f"app-{theme}-{locale}-{width}"
    query = "document.querySelector('input[data-tour-anchor], [data-tour-anchor=launcher-composer] input')"
    enter = "document.querySelector('[data-tour-anchor=launcher-enter-app]')"
    sidebar = "document.querySelector('.sidebar-panel-shell')"
    client.command("resize", width=width, height=820 if width > 360 else 640)
    client.command("route", query=urlencode({"theme": theme, "locale": locale, "qa_case": name}))
    client.wait(f"location.pathname === '/launcher' && new URL(location.href).searchParams.get('qa_case') === {json.dumps(name)} && !!({enter})",
                f"real Launcher route {name}")
    client.wait("document.readyState === 'complete' && document.fonts.status === 'loaded'", "App styles ready")
    client.activate()
    require(client.evaluate("document.documentElement.dataset.theme") == theme, "App theme did not hydrate")
    require(client.evaluate("document.documentElement.lang") == ("zh-CN" if locale == "zh" else "en"), "App locale did not hydrate")
    client.wait(inside(enter), "Launcher entry fits the window")
    client.wait(inside(query), "Launcher input fits the window")
    # Exercise DesktopWindow's drag-region double click. A CSS/JS resize cannot
    # satisfy the native frame observations used for zoom and unzoom.
    before_zoom = client.command("status")["window"]
    client.click("document.querySelector('header[data-desktop-window-drag-region]')", count=2)
    eventually(lambda: client.command("status")["window"] != before_zoom, "native title-region zoom")
    client.click("document.querySelector('header[data-desktop-window-drag-region]')", count=2)
    eventually(lambda: client.command("status")["window"] == before_zoom, "native title-region unzoom")
    client.click(query)
    client.key("q", 12)
    client.wait(f"({query}).value === 'q'", "real controlled Launcher input")
    client.snapshot(name + "-launcher")
    client.click(enter)
    client.wait(f"location.pathname === '/app' && !!({sidebar})", "Launcher navigates through the real App router")
    client.wait(inside(sidebar), "workbench sidebar fits the native window")
    client.wait("document.querySelector('main').scrollWidth <= innerWidth + 1", "workbench horizontal bounds")
    client.wait("(() => { const labels = [...document.querySelectorAll('.shell-navigation-rail button[aria-pressed] > span:nth-child(2)')]; return labels.length === 3 && labels.every(e => { const range = document.createRange(); range.selectNodeContents(e); const text = range.getBoundingClientRect(); const box = e.getBoundingClientRect(); const style = getComputedStyle(e); return text.left >= box.left + parseFloat(style.paddingLeft) - 0.01 && text.right <= box.right - parseFloat(style.paddingRight) + 0.01; }); })()",
                "primary navigation labels remain readable")
    rail_width = client.evaluate("document.querySelector('.shell-navigation-rail').getBoundingClientRect().width")
    require(rail_width == 64, "App navigation rail is not compact")
    require(client.evaluate("document.querySelector('.desktop-app-stage') !== null") == (width > 559),
            "workbench narrow directory handoff changed")
    client.snapshot(name + "-workbench")
    status = client.command("status")
    require(status["webview"][0] == width, "App viewport differs from native content size")
    require(abs(status["controls_center"][1] - 24) <= 1 and status["controls_inset"] > 60, "App native controls metrics changed")
    client.evaluate("window.qaMarker = 'real-app-route-preserved'")
    other_width = 360 if width > 360 else 1280
    client.command("resize", width=other_width, height=640 if other_width == 360 else 820)
    client.wait(f"innerWidth === {other_width} && (document.querySelector('.desktop-app-stage') !== null) === {str(other_width > 559).lower()}",
                "live native resize updates responsive routing")
    require(client.evaluate("window.qaMarker") == "real-app-route-preserved", "Resize reloaded the document")
    client.command("resize", width=width, height=820 if width > 360 else 640)
    client.wait(f"innerWidth === {width}", "native content size restored")
    before_resume = client.command("timeline")
    count = lambda entries, event: sum(entry["event"] == event for entry in entries)
    client.command("hide")
    require(not client.command("status")["visible"], "App window failed to hide")
    time.sleep(5.1)
    client.command("show")
    client.activate()
    eventually(lambda: count(client.command("timeline"), "webview.resume_check_ready") >
               count(before_resume, "webview.resume_check_ready"), "App resume readiness")
    require(client.evaluate("window.qaMarker") == "real-app-route-preserved", "App resume lost the document")
    require(count(client.command("timeline"), "webview.load_begin") == count(before_resume, "webview.load_begin"),
            "App resume unexpectedly reloaded the document")
    require(client.evaluate("window.qaErrors") == [], "App reported a browser error")
    events = client.evaluate("window.qaEvents")
    require(all(event["trusted"] for event in events), "App received synthetic DOM input")
    require({event["type"] for event in events} >= {"click", "input", "keydown"}, "Missing App native input evidence")
    return {"case": name, "status": status, "navigation_width": rail_width, "native_events": events, "passed": True}


def verify(settings: dict, suite: str = "foundation", smoke: bool = False) -> None:
    run = Path(settings["output"])
    with socket.socket() as candidate:
        candidate.bind(("127.0.0.1", 0))
        port = candidate.getsockname()[1]
    require(port != 34343, "Allocated the product port; run again for an isolated port")
    environment = os.environ.copy()
    environment.update(NEXUS_UI_TEST_PORT=str(port), NEXUS_UI_TEST_OUTPUT=str(run),
                       NEXUS_DESKTOP_STATE_ROOT=settings["state_root"],
                       NEXUS_DESKTOP_PREFERENCES_SUITE=settings["bundle_id"] + ".bootstrap",
                       NEXUS_DESKTOP_DISABLE_UPDATE_CHECK="1")
    environment["NEXUS_UI_TEST_SURFACE"] = suite
    report = {"scope": f"Current WindowManager/WKWebView with {suite}; no AppDelegate or sidecar",
              "platform": platform.platform(), "cases": []}
    client = None
    with (run / "server.log").open("w") as log:
        server = subprocess.Popen(["node", "browser-tests/native-ui-server.mjs"], cwd=ROOT / "web", env=environment,
                                  stdout=log, stderr=subprocess.STDOUT)
        try:
            def health():
                if server.poll() is not None:
                    raise RuntimeError("Native fixture server exited; inspect server.log")
                try:
                    with urlopen(f"http://127.0.0.1:{port}/qa/health", timeout=1) as response:
                        return json.load(response)
                except OSError:
                    return None
            ready = eventually(health, "fixture server readiness")
            require(ready["kind"] == "nexus-native-ui-fixture" and ready["port"] == port, "Wrong fixture server")
            client = NativeClient(settings, environment)
            runner = run_app_shell_case if suite == "app-shell" else run_case
            for theme in (["light"] if smoke else ["light", "dark", "rain"]):
                for locale in (["en"] if smoke else ["en", "zh"]):
                    for width in ([1280] if smoke else [1280, 360]):
                        result = runner(client, theme, locale, width)
                        report["cases"].append(result)
                        print(f"PASS {result['case']}", flush=True)
            report["timeline"] = client.command("timeline")
            report["user_agent"] = client.evaluate("navigator.userAgent")
            report["transport"] = health()
            require(report["transport"]["rejectedBusinessRequests"] == 0, "UI attempted an unsupported business request")
            require(report["transport"]["rejected"] == [], "Unexpected fixture request or command")
            if suite == "app-shell":
                require(report["transport"]["socketConnections"] > 0, "App event transport was not exercised")
            report["passed"] = True
        except Exception as error:
            report["passed"] = False
            report["error"] = str(error)
            if client is not None and client.process.poll() is None:
                for operation, arguments in [("status", {}), ("snapshot", {"name": "failure"}), ("timeline", {})]:
                    try:
                        report["failure_" + operation] = client.command(operation, **arguments)
                    except Exception:
                        pass
            raise
        finally:
            (run / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
            if client is not None:
                client.close()
            server.terminate()
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=Path("/tmp/nexus-native-ui"))
    parser.add_argument("--build-only", action="store_true", help="Compile the isolated app without opening a window")
    parser.add_argument("--suite", choices=["foundation", "app-shell"], default="foundation")
    parser.add_argument("--smoke", action="store_true", help="Run only the light English desktop case")
    args = parser.parse_args()
    settings = build(args.output.resolve())
    print(json.dumps(settings, indent=2), flush=True)
    if not args.build_only:
        verify(settings, args.suite, args.smoke)


if __name__ == "__main__":
    main()

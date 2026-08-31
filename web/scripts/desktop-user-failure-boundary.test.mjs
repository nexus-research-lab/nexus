import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("native dialogs never rebuild user copy from raw exceptions", () => {
  const macApp = read("desktop/macos/Sources/NexusDesktop/AppDelegate.swift");
  const macWindow = read("desktop/macos/Sources/NexusDesktop/Window/WindowManager.swift");
  const macUpdate = read("desktop/macos/Sources/NexusDesktop/Update/DesktopUpdateChecker.swift");
  const windowsApp = read("desktop/windows/Nexus.Desktop/App.xaml.cs");
  const windowsUpdate = read("desktop/windows/Nexus.Desktop/Update/DesktopUpdateChecker.cs");

  assert.doesNotMatch(macApp, /informativeText\s*=.*localizedDescription/);
  assert.doesNotMatch(macApp, /diagnosticsURL\.path/);
  assert.doesNotMatch(macApp, /globalShortcutLastError\s*=\s*error\.localizedDescription/);
  assert.doesNotMatch(macWindow, /NSAlert\(error:/);
  assert.doesNotMatch(macUpdate, /informativeText\s*=.*localizedDescription/);
  assert.doesNotMatch(macUpdate, /更新[^\n]*\\\(error\.localizedDescription\)/);
  assert.doesNotMatch(macUpdate, /defaults\.set\(error\.localizedDescription, forKey: DefaultsKey\.lastErrorMessage\)/);
  assert.doesNotMatch(windowsApp, /\?\s*exception\.Message/);
  assert.doesNotMatch(windowsApp, /诊断文件：\{diagnosticsPath\}/);
  assert.doesNotMatch(windowsUpdate, /NexusDialogWindow\.(?:ShowMessage|Confirm)\([\s\S]{0,500}exception\.Message/);
  assert.doesNotMatch(windowsUpdate, /LastErrorMessage\s*=\s*exception\.Message/);
});

test("desktop bridge keeps raw causes internal and returns stage-specific safe copy", () => {
  const macBridge = read("desktop/macos/Sources/NexusDesktop/Bridge/DesktopBridgeHandler.swift");
  const macBridgeScript = read("desktop/macos/Sources/NexusDesktop/Bridge/DesktopBridgeScript.swift");
  const windowsBridge = read("desktop/windows/Nexus.Desktop/Bridge/DesktopBridgeHandler.cs");
  const windowsBridgeScript = read("desktop/windows/Nexus.Desktop/Bridge/DesktopBridgeScript.cs");
  const macCopy = read("desktop/macos/Sources/NexusDesktop/DesktopFailureCopy.swift");
  const windowsCopy = read("desktop/windows/Nexus.Desktop/Dialog/DesktopFailureCopy.cs");

  assert.doesNotMatch(macBridge, /reject\([^)]*error\.localizedDescription/);
  assert.doesNotMatch(windowsBridge, /RejectAsync\([^)]*exception\.Message/);
  assert.match(macBridge, /DesktopFailureCopy\.bridgeMessage\(for: request\.kind\)/);
  assert.match(windowsBridge, /DesktopFailureCopy\.BridgeMessage\(kind\)/);
  assert.match(macBridge, /payload\["migration_error"\] = DesktopFailureCopy\.stateRootMigrationMessage/);
  assert.match(windowsBridge, /status\["migration_error"\] = DesktopFailureCopy\.StateRootMigrationMessage/);

  for (const source of [macBridgeScript, windowsBridgeScript]) {
    assert.doesNotMatch(source, /reject\(error\)/);
    assert.doesNotMatch(source, /Desktop bridge request (?:failed|timed out)/);
    assert.match(source, /已有设置、会话、任务和文件未被修改/);
    assert.match(source, /桌面操作结果待确认/);
    assert.match(source, /相关设置或内容需要到对应页面核对/);
    assert.match(source, /timeoutMessage\(request\.kind\)/);
    assert.match(source, /这次读取不会修改已有设置、会话、任务或文件/);
    assert.match(source, /更新状态待核对/);
    assert.match(source, /不要移动或删除新旧数据目录/);
    assert.doesNotMatch(source, /可能[^。；]*也可能/);
  }

  for (const source of [macCopy, windowsCopy]) {
    assert.match(source, /数据目录迁移结果待确认/);
    assert.match(source, /不要移动或删除新旧数据目录/);
    assert.match(source, /核对当前状态/);
    assert.match(source, /已继续使用原目录/);
    assert.match(source, /新目录未启用/);
    assert.match(source, /手动合并或删除/);
  }
});

test("browser popup never renders background error text and announces complete recovery copy", () => {
  const popup = read("desktop/browser-extension/popup.js");
  const popupHTML = read("desktop/browser-extension/popup.html");

  assert.doesNotMatch(popup, /(?:messageNode|statusNode)\.textContent\s*=\s*error\?\.message/);
  assert.doesNotMatch(popup, /(?:messageNode|statusNode)\.textContent\s*=\s*String\(error\)/);
  assert.match(popup, /是否已经生效暂时无法确认/);
  assert.match(popup, /不会因此改变/);
  assert.match(popup, /核对地址与连接状态/);
  assert.match(popup, /测试不会修改连接设置/);
  assert.match(popup, /先检查标签页/);
  assert.match(popupHTML, /id="message"[^>]*aria-atomic="true"/);
  assert.match(popupHTML, /id="hint"[^>]*aria-atomic="true"/);
});

function read(relativePath) {
  return fs.readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("desktop render recovery is event-driven instead of periodically probing", () => {
  const webWatchdog = read("web/src/bootstrap/recovery/render-watchdog.ts");
  const windowsWindow = read("desktop/windows/Nexus.Desktop/Window/MainWindow.xaml.cs");
  const windowsWebView = read("desktop/windows/Nexus.Desktop/WebView/WebViewHost.cs");
  const macWindow = read("desktop/macos/Sources/NexusDesktop/Window/WindowManager.swift");
  const macWebView = read("desktop/macos/Sources/NexusDesktop/WebView/WebViewHost.swift");

  assert.doesNotMatch(webWatchdog, /setInterval|APP_RENDER_WATCHDOG_INTERVAL_MS/);
  assert.match(webWatchdog, /addEventListener\("focus"/);
  assert.match(webWatchdog, /visibilitychange/);

  assert.doesNotMatch(windowsWindow, /DispatcherTimer|periodic_visible/);
  assert.match(windowsWindow, /OnActivated/);
  assert.match(windowsWebView, /ProcessFailed/);
  assert.match(windowsWebView, /NavigationCompleted \+= HandleNavigationCompleted/);
  assert.equal(
    windowsWebView.match(/ShouldSkipResumeCheckForNavigation\(reason, observedNavigationId\)/g)?.length,
    3,
  );
  assert.match(windowsWebView, /activeNavigationId = args\.NavigationId/);
  assert.match(windowsWebView, /lastNavigationId == observedNavigationId/);

  assert.doesNotMatch(macWindow, /installMainWindowHealthProbe|periodic_visible/);
  assert.match(macWindow, /windowDidBecomeKey/);
  assert.match(macWebView, /webViewWebContentProcessDidTerminate/);
});

function read(relativePath) {
  return fs.readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

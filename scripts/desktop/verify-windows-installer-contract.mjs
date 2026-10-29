#!/usr/bin/env node

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const rootDirectory = path.resolve(scriptDirectory, "../..");
const installerScriptArgument = process.argv.indexOf("--installer-script");
const installerScriptPath = installerScriptArgument >= 0
  ? process.argv[installerScriptArgument + 1]
  : path.join(scriptDirectory, "package-windows-app.ps1");

if (!installerScriptPath) {
  throw new Error("--installer-script requires a path");
}

const read = (relativePath) => fs.readFileSync(path.join(rootDirectory, relativePath), "utf8");
const capture = (source, pattern, label) => {
  const match = source.match(pattern);
  assert.ok(match, `Missing ${label}`);
  return match[1];
};

const coordinatorSource = read(
  "desktop/windows/Nexus.Desktop/Lifecycle/DesktopSingleInstanceCoordinator.cs",
);
const protocolSource = read(
  "desktop/windows/Nexus.Desktop/Runtime/DesktopProtocolRouter.cs",
);
const installerSource = fs.readFileSync(path.resolve(installerScriptPath), "utf8");
const sidecarSupervisorSource = read(
  "desktop/windows/Nexus.Desktop/Sidecar/SidecarSupervisor.cs",
);

const applicationMutex = capture(
  coordinatorSource,
  /MutexName\s*=\s*@"([^"]+)"/,
  "application mutex",
);
const applicationExitArgument = capture(
  protocolSource,
  /ExitCommandArgument\s*=\s*"([^"]+)"/,
  "application exit argument",
);
const installerMutex = capture(
  installerSource,
  /NexusMutexName\s*=\s*'([^']+)'/,
  "installer mutex",
);
const installerExitArgument = capture(
  installerSource,
  /NexusExitArgument\s*=\s*'([^']+)'/,
  "installer exit argument",
);

assert.equal(installerMutex, applicationMutex, "Installer and application mutexes differ");
assert.equal(
  installerExitArgument,
  applicationExitArgument,
  "Installer and application exit arguments differ",
);
assert.match(installerSource, /function PrepareToInstall\s*\(/);
assert.match(installerSource, /CheckForMutexes\(NexusMutexName\)/);
assert.match(installerSource, /RestartApplications=no/);
// The ordinary desktop installer stays per-user. A privileged sandbox helper
// needs its own protected installation and cannot elevate this mutable bundle.
assert.match(installerSource, /^PrivilegesRequired=lowest\r?$/m);
assert.match(installerSource, /^DefaultDirName=\{localappdata\}\\Programs\\[^\r\n]+\r?$/m);

// Closing must leave the WPF Closing callback before requesting a second close.
const windowSource = read("desktop/windows/Nexus.Desktop/Window/MainWindow.xaml.cs");
const closing = windowSource.slice(windowSource.indexOf("protected override void OnClosing"), windowSource.indexOf("protected override void OnClosed"));
assert.doesNotMatch(closing, /\bClose\(\)/);
assert.match(closing, /App\.RequestApplicationExit\(0\)/);
const appSource = read("desktop/windows/Nexus.Desktop/App.xaml.cs");
assert.match(appSource, /await sidecar\.StopAsync\(\)/);
assert.match(appSource, /sidecar\?\.Dispose\(\);[\s\S]*singleInstance\?\.Dispose\(\)/);
assert.doesNotMatch(coordinatorSource, /serverTask\?\.Wait\(/);
assert.doesNotMatch(sidecarSupervisorSource, /GetProcessesByName/);

assert.ok(read("scripts/desktop/build-windows-app.ps1").includes('"nxs-mxc.exe", "nxs-mxc-LICENSE.txt"'), "bundle Rust helper and license");
assert.ok(read("scripts/desktop/package-windows-app.ps1").includes('(Join-Path $AppBuildDir "Resources/bin/nxs-mxc.exe")'), "sign Rust helper");
assert.ok(read("scripts/desktop/package-windows-app.ps1").includes('Missing bundled Rust sandbox helper'), "reject incomplete sandbox package");

console.log("Windows installer shutdown, Rust helper, and per-user installation contracts verified");

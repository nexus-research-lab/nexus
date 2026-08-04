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

console.log("Windows installer shutdown contract verified");

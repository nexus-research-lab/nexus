#!/usr/bin/env node
// INPUT: The Nexus checkout and its Go toolchain; on Windows, the native host.
// OUTPUT: Reproducible cross-architecture build/native smoke evidence.
// POS: Windows sandbox gate; it never upgrades component or build evidence to
// a release acceptance claim.

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { requirePassedTests } from "./sandbox-test-evidence.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const reportDirectory = fs.mkdtempSync(path.join(os.tmpdir(), "nexus-windows-sandbox-"));
const baseEnvironment = {
  ...process.env,
  GOWORK: "off",
  CGO_ENABLED: "0",
};
const report = {
  version: 1,
  startedAt: new Date().toISOString(),
  platform: process.platform,
  architecture: process.arch,
  scope: process.platform === "win32" ? "windows-native-host-and-cross-build" : "windows-cross-build",
  releaseAccepted: false,
  passed: false,
  checks: [],
};

console.log(`Windows sandbox evidence: ${reportDirectory}`);

function run(name, command, args, environment = {}) {
  const result = spawnSync(command, args, {
    cwd: root,
    env: { ...baseEnvironment, ...environment },
    encoding: "utf8",
    timeout: 180_000,
    maxBuffer: 64 << 20,
  });
  fs.writeFileSync(path.join(reportDirectory, `${name}.stdout.log`), result.stdout ?? "");
  fs.writeFileSync(path.join(reportDirectory, `${name}.stderr.log`), result.stderr ?? "");
  report.checks.push({ name, command, args, exitCode: result.status, signal: result.signal });
  if (result.error || result.status !== 0) {
    throw new Error(`${name} failed: ${result.error?.message ?? `exit ${result.status}`}`);
  }
  return result.stdout ?? "";
}

function runGoTest(name, packages, requiredTests, environment = {}) {
  const parents = [...new Set(requiredTests.map((test) => test.split("/")[0]))];
  const output = run(
    name,
    "go",
    ["test", "-mod=readonly", "-json", "-count=1", "-timeout=2m", ...packages, "-run", `^(${parents.join("|")})$`],
    environment,
  );
  report.checks.at(-1).passedTests = requirePassedTests(output, 0, requiredTests);
}

function artifactName(prefix, packagePath, goarch) {
  return `${prefix}-${packagePath.replaceAll("/", "-")}-${goarch}.test.exe`;
}

try {
  run("installer-contract", "node", ["scripts/desktop/verify-windows-installer-contract.mjs"]);

  for (const goarch of ["amd64", "arm64"]) {
    for (const packagePath of ["./internal/runtime", "./internal/runtime/clientopts", "./internal/infra/confinedfs"]) {
      run(
        `test-compile-${goarch}-${packagePath.replaceAll("/", "-")}`,
        "go",
        ["test", "-mod=readonly", "-c", packagePath, "-o", path.join(reportDirectory, artifactName("nexus", packagePath, goarch))],
        { GOOS: "windows", GOARCH: goarch },
      );
    }
    for (const commandPath of ["./cmd/nexus-server", "./cmd/nexusctl", "./cmd/nexuscfg"]) {
      run(
        `build-${goarch}-${commandPath.replaceAll("/", "-")}`,
        "go",
        ["build", "-trimpath", "-mod=readonly", "-o", path.join(reportDirectory, `nexus-${commandPath.replaceAll("/", "-")}-${goarch}.exe`), commandPath],
        { GOOS: "windows", GOARCH: goarch },
      );
    }
  }

  if (process.platform === "win32") {
    const nativeGoarch = process.arch === "arm64" ? "arm64" : process.arch === "x64" ? "amd64" : "";
    if (!nativeGoarch) {
      throw new Error(`unsupported native Windows Node architecture: ${process.arch}`);
    }
    runGoTest(
      "native-process-identity",
      ["./internal/runtime"],
      ["TestSandboxProcessMarkerUsesConservativeIdentityFallback", "TestWindowsSandboxProcessIdentityMatchesCurrentProcess"],
      { GOOS: "windows", GOARCH: nativeGoarch },
    );
  }

  report.passed = true;
  report.finishedAt = new Date().toISOString();
  fs.writeFileSync(path.join(reportDirectory, "report.json"), `${JSON.stringify(report, null, 2)}\n`);
  console.log(`Windows sandbox gate passed; releaseAccepted=false; report=${path.join(reportDirectory, "report.json")}`);
} catch (error) {
  report.error = String(error?.message ?? error);
  report.finishedAt = new Date().toISOString();
  fs.writeFileSync(path.join(reportDirectory, "report.json"), `${JSON.stringify(report, null, 2)}\n`);
  console.error(`Windows sandbox gate failed; report=${path.join(reportDirectory, "report.json")}`);
  console.error(report.error);
  process.exitCode = 1;
}

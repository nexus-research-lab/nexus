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
// Includes native settings recovery, junction rejection and explicit P3 probes.
const WINDOWS_NATIVE_SDK_COMMIT = "38221838946f17f87be9da4af5f10ddc95671edf";
const WINDOWS_NATIVE_SDK_PACKAGE = "./internal/tool/builtin/bash/sandboxexec";
const WINDOWS_NATIVE_TESTS = [
  "TestWindowsPrivateDesktopLifecycle",
  "TestWindowsCommandPreservesArguments",
  "TestWindowsCommandRejectsAmbiguousInputs",
  "TestWindowsCommandEnvironmentIsExplicit",
  "TestWindowsCommandCloseCanRetryWithoutReviving",
  "TestWindowsCommandOwnerRejectsHost",
  "TestWindowsCommandConcurrentCloseHonorsCancellation",
  "TestWindowsCommandClosePreservesBorrowedHandles",
  "TestWindowsCommandCloseCollectsUnassignedRoot",
  "TestWindowsProcessCreationRejectsRetiredJob",
  "TestWindowsProcessCreationRejectsCanceledAdmission",
  "TestWindowsProcessCanceledStartCannotBeRevived",
  "TestWindowsSandboxJobTerminatesAssignedSuspendedProcess",
  "TestWindowsSandboxJobTerminatesRunningDescendants",
  "TestWindowsRunnerHostExitTerminatesRunningDescendants",
  "TestWindowsRunnerExecutionLifecycle",
  "TestWindowsRunnerHostLifetime",
  "TestWindowsRunnerPipePeerMatchesKernelIdentity",
  "TestWindowsRunnerPipeRejectsHostExecutionIdentity",
  "TestWindowsRunnerThreadProtectionRejectsHost",
  "TestWindowsSandboxIdentityRejectsHostToken",
  "TestWindowsBrokerIdentityRejectsCurrentAccount",
  "TestWindowsHostDenialMergeIsIdempotent",
  "TestWindowsTemporaryDirectoryRejectsHostIdentity",
  "TestWindowsUnsupportedExecutionDoesNotPrepareResources",
  "TestWindowsJunctionCannotAuthorizeWorkspaceIO/outside",
  "TestWindowsJunctionCannotAuthorizeWorkspaceIO/inside",
  "TestWindowsJunctionCannotAuthorizeWorkspaceIO/broken",
];
const WINDOWS_NATIVE_SETTINGS_TESTS = [
  "TestSettingsBindingConcurrentUpdates",
  "TestDocumentStoreWritesTwoDocuments",
  "TestDocumentStoreRollsBackProvenlyAppliedDocuments",
  "TestDocumentStoreBreaksHardlinkOnWrite",
  "TestSettingsJournalIsClearedAfterSuccessfulUpdate",
  "TestSettingsJournalCrossRootMixedStateFailsClosed",
  "TestSettingsJournalDoesNotStoreDocumentPlaintext",
  "TestSettingsJournalProcessExitRecovery/committed-0",
  "TestSettingsJournalProcessExitRecovery/committed-1",
  "TestSettingsJournalProcessExitRecovery/committed-2",
  "TestWindowsSettingsJournalLongPath",
  "TestWindowsSettingsJournalPublishPreservesExisting",
];
const WINDOWS_NATIVE_BRIDGE_TESTS = [
  "TestWindowsJobAdmissionAndCleanup",
  "TestWindowsProbeCancellationClosesJob",
  "TestWindowsProbeRejectsLingeringOutput",
  "TestWindowsHostCrashClosesRuntimeJob",
  "TestClaudeNativeSandboxProbeUsesSettingsAndScrubsSecrets",
  "TestClaudeNativeSandboxProbeFailsClosedWhenSettingsFlagIsMissing",
  "TestClaudeRestrictedProbeRequiresAdvertisedFlagAndScrubsSecrets",
  "TestClaudeRestrictedProbeFailsClosedWhenFlagIsMissing",
];

const argumentsList = process.argv.slice(2);
const nativeMode = argumentsList.includes("--native");
const unknownArguments = argumentsList.filter((argument) => argument !== "--native");
const sdkSource = process.env.NEXUS_SANDBOX_SDK_SOURCE ?? "";
const configuredReportRoot = process.env.NEXUS_SANDBOX_REPORT_DIRECTORY ?? "";
const reportRoot = configuredReportRoot ? path.resolve(configuredReportRoot) : os.tmpdir();
fs.mkdirSync(reportRoot, { recursive: true });
const reportDirectory = fs.mkdtempSync(path.join(reportRoot, "nexus-windows-sandbox-"));
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
  mode: nativeMode ? "native" : "cross-build",
  scope: nativeMode
    ? "windows-native-components-and-cross-build"
    : process.platform === "win32"
      ? "windows-native-host-and-cross-build"
      : "windows-cross-build",
  releaseAccepted: false,
  passed: false,
  checks: [],
  ...(nativeMode
    ? {
        sdkPackage: WINDOWS_NATIVE_SDK_PACKAGE,
        requiredNativeTests: WINDOWS_NATIVE_TESTS,
        requiredSettingsTests: WINDOWS_NATIVE_SETTINGS_TESTS,
        requiredBridgeTests: WINDOWS_NATIVE_BRIDGE_TESTS,
      }
    : {}),
};

console.log(`Windows sandbox evidence: ${reportDirectory}`);

function run(name, command, args, environment = {}, cwd = root) {
  const result = spawnSync(command, args, {
    cwd,
    env: { ...baseEnvironment, ...environment },
    encoding: "utf8",
    timeout: 180_000,
    maxBuffer: 64 << 20,
  });
  fs.writeFileSync(path.join(reportDirectory, `${name}.stdout.log`), result.stdout ?? "");
  fs.writeFileSync(path.join(reportDirectory, `${name}.stderr.log`), result.stderr ?? "");
  report.checks.push({ name, command, args, cwd, exitCode: result.status, signal: result.signal });
  if (result.error || result.status !== 0) {
    throw new Error(`${name} failed: ${result.error?.message ?? `exit ${result.status}`}`);
  }
  return result.stdout ?? "";
}

function runGoTest(name, packages, requiredTests, environment = {}, cwd = root) {
  const parents = [...new Set(requiredTests.map((test) => test.split("/")[0]))];
  const output = run(
    name,
    "go",
    ["test", "-mod=readonly", "-json", "-count=1", "-timeout=2m", ...packages, "-run", `^(${parents.join("|")})$`],
    environment,
    cwd,
  );
  report.checks.at(-1).passedTests = requirePassedTests(output, 0, requiredTests);
}

function artifactName(prefix, packagePath, goarch) {
  return `${prefix}-${packagePath.replaceAll("/", "-")}-${goarch}.test.exe`;
}

try {
  if (unknownArguments.length) {
    throw new Error(`unknown arguments: ${unknownArguments.join(", ")}`);
  }
  if (nativeMode && process.platform !== "win32") {
    throw new Error("--native requires a Windows host; cross-build evidence cannot stand in for native execution");
  }
  report.nexusCommit = run("nexus-revision", "git", ["rev-parse", "HEAD"]).trim();
  report.nexusWorktreeStatus = run("nexus-status", "git", ["status", "--porcelain"]).trim();
  const bridgeModule = JSON.parse(run("bridge-module", "go", ["list", "-mod=readonly", "-m", "-json", "github.com/nexus-research-lab/nexus-agent-sdk-bridge"]));
  if (bridgeModule.Replace) throw new Error("Windows evidence requires the pinned remote Bridge module without replacement");
  report.bridgeVersion = bridgeModule.Version;
  report.bridgeSum = bridgeModule.Sum;
  if (nativeMode) {
    if (!path.isAbsolute(sdkSource)) {
      throw new Error("--native requires NEXUS_SANDBOX_SDK_SOURCE to be an absolute SDK checkout path");
    }
    if (!fs.existsSync(path.join(sdkSource, "go.mod"))) {
      throw new Error(`NEXUS_SANDBOX_SDK_SOURCE is not a Go checkout: ${sdkSource}`);
    }
    const sdkRevision = run("sdk-revision", "git", ["rev-parse", "HEAD"], {}, sdkSource).trim();
    report.sdkSource = sdkSource;
    report.sdkCommit = sdkRevision;
    if (sdkRevision !== WINDOWS_NATIVE_SDK_COMMIT) {
      throw new Error(`SDK checkout must be ${WINDOWS_NATIVE_SDK_COMMIT}, got ${sdkRevision}`);
    }
    const sdkStatus = run("sdk-status", "git", ["status", "--porcelain"], {}, sdkSource).trim();
    if (sdkStatus) {
      throw new Error("SDK checkout must be clean for native evidence");
    }
  }

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
      ["TestSandboxProcessMarkerUsesConservativeIdentityFallback", "TestWindowsSandboxProcessIdentityMatchesCurrentProcess", "TestWindowsSandboxProcessIdentityRejectsRetainedExitedProcess"],
      { GOOS: "windows", GOARCH: nativeGoarch },
    );
    runGoTest(
      "native-host-resources",
      ["./internal/runtime", "./internal/runtime/clientopts", "./internal/infra/confinedfs"],
      [
        "TestSandboxCrashRestartRequiresExplicitReconcile",
        "TestSandboxCleanupUnknownSurvivesRestart",
        "TestDesktopSandboxCopiesHostPreparedResources",
        "TestBuildAgentClientOptionsInstallsClaudeNativeContract",
        "TestReadFileSurvivesAtomicReplacement",
        "TestWindowsConfinedFSRejectsJunction",
      ],
      { GOOS: "windows", GOARCH: nativeGoarch },
    );
    runGoTest(
      "native-bridge-lifecycle",
      ["github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"],
      WINDOWS_NATIVE_BRIDGE_TESTS,
      { GOOS: "windows", GOARCH: nativeGoarch },
    );
  }

  if (nativeMode) {
    const nativeGoarch = process.arch === "arm64" ? "arm64" : process.arch === "x64" ? "amd64" : "";
    if (!nativeGoarch) {
      throw new Error(`unsupported native Windows Node architecture: ${process.arch}`);
    }
    runGoTest(
      "native-sdk-components",
      [WINDOWS_NATIVE_SDK_PACKAGE],
      WINDOWS_NATIVE_TESTS,
      { GOOS: "windows", GOARCH: nativeGoarch },
      sdkSource,
    );
    runGoTest(
      "native-sdk-settings",
      ["./internal/config/settings"],
      WINDOWS_NATIVE_SETTINGS_TESTS,
      { GOOS: "windows", GOARCH: nativeGoarch },
      sdkSource,
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

#!/usr/bin/env node
// INPUT: An explicit nxs binary, or an SDK repository/ref to export and build.
// OUTPUT: Pinned-dependency test logs and a scoped JSON evidence report.
// POS: Opt-in sandbox baseline; no model calls, setup, policy changes or release acceptance.

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { requirePassedTests } from "./sandbox-test-evidence.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const usage = "node scripts/desktop/check-sandbox-baseline.mjs (--nxs /absolute/nxs | --sdk-source /repo [--sdk-ref ref])";
const options = {};
for (let index = 2; index < process.argv.length; index += 2) {
  const key = process.argv[index];
  const value = process.argv[index + 1];
  if (!["--nxs", "--sdk-source", "--sdk-ref"].includes(key) || !value || value.startsWith("--") || options[key]) {
    throw new Error(usage);
  }
  options[key] = value;
}
if (Boolean(options["--nxs"]) === Boolean(options["--sdk-source"]) || options["--sdk-ref"] && !options["--sdk-source"]) {
  throw new Error(usage);
}
if (options["--sdk-source"] && process.platform !== "darwin") {
  throw new Error("SDK native baseline currently requires macOS; use --nxs for host integration only.");
}
for (const key of ["--nxs", "--sdk-source"]) {
  if (options[key] && !path.isAbsolute(options[key])) throw new Error(`${key} requires an absolute path`);
}

const reportDirectory = fs.mkdtempSync(path.join(os.tmpdir(), "nexus-sandbox-baseline-"));
const environment = { ...process.env, GOWORK: "off", NEXUS_CONFIG_DIR: path.join(reportDirectory, "config") };
const report = {
  version: 1, startedAt: new Date().toISOString(),
  platform: process.platform, architecture: process.arch, osRelease: os.release(),
  scope: options["--sdk-source"] ? "host-and-macos-native-baseline" : "host-integration-only",
  releaseAccepted: false, passed: false, checks: [],
};
console.log(`Sandbox evidence: ${reportDirectory}`);

// Commands never pass through a shell; output stays in an isolated evidence directory.
function run(name, command, args, cwd = root, env = environment) {
  console.log(`Checking ${name}`);
  const result = spawnSync(command, args, { cwd, env, encoding: "utf8", timeout: 180_000, maxBuffer: 64 << 20 });
  fs.writeFileSync(path.join(reportDirectory, `${name}.stdout.log`), result.stdout ?? "");
  fs.writeFileSync(path.join(reportDirectory, `${name}.stderr.log`), result.stderr ?? "");
  report.checks.push({ name, command, args, cwd, exitCode: result.status, signal: result.signal });
  if (result.error || result.status !== 0) throw new Error(`${name} failed: ${result.error?.message ?? `exit ${result.status}`}`);
  return result.stdout;
}

function testGroup(name, packages, requiredTests, cwd = root, env = environment) {
  // Go splits -run at slashes; run each parent, then require exact child evidence.
  const parents = [...new Set(requiredTests.map((test) => test.split("/")[0]))];
  const pattern = `^(${parents.join("|")})$`;
  const output = run(name, "go", ["test", "-mod=readonly", "-json", "-count=1", "-timeout=2m", ...packages, "-run", pattern], cwd, env);
  report.checks.at(-1).passedTests = requirePassedTests(output, 0, requiredTests);
}

const sha256 = (file) => createHash("sha256").update(fs.readFileSync(file)).digest("hex");
try {
  report.nexusRevision = run("nexus-revision", "git", ["rev-parse", "HEAD"]).trim();
  report.nexusChanges = run("nexus-changes", "git", ["status", "--porcelain"]).trim().split("\n").filter(Boolean);
  report.goVersion = run("go-version", "go", ["env", "GOVERSION"]).trim();
  environment.GOTOOLCHAIN = report.goVersion;
  const bridge = JSON.parse(run("bridge-module", "go", ["list", "-mod=readonly", "-m", "-json", "github.com/nexus-research-lab/nexus-agent-sdk-bridge"]));
  if (bridge.Replace) throw new Error("Bridge replacement cannot satisfy pinned dependency acceptance.");
  report.bridge = { path: bridge.Path, version: bridge.Version, sum: bridge.Sum };
  let binary = options["--nxs"];
  let sdkSource;
  if (options["--sdk-source"]) {
    const repository = fs.realpathSync(options["--sdk-source"]);
    report.sdkRevision = run("sdk-revision", "git", ["rev-parse", "--verify", `${options["--sdk-ref"] ?? "HEAD"}^{commit}`], repository).trim();
    report.sdkWorktreeChanges = run("sdk-changes-excluded", "git", ["status", "--porcelain"], repository).trim().split("\n").filter(Boolean);
    sdkSource = path.join(reportDirectory, "sdk");
    fs.mkdirSync(sdkSource);
    const archive = path.join(reportDirectory, "sdk.tar");
    run("sdk-export", "git", ["archive", "--format=tar", "--output", archive, report.sdkRevision], repository);
    run("sdk-extract", "tar", ["-xf", archive, "-C", sdkSource]);
    binary = path.join(reportDirectory, "nxs");
    run("sdk-build", "go", ["build", "-mod=readonly", "-o", binary, "./cmd/nxs"], sdkSource);
  }
  binary = fs.realpathSync(binary);
  fs.accessSync(binary, fs.constants.X_OK);
  report.runtime = { path: binary, sha256: sha256(binary), source: sdkSource ? "fixed-sdk-archive" : "explicit-binary" };
  environment.NEXUS_SANDBOX_TEST_BINARY = binary;

  testGroup("host-policy", ["./internal/runtime/clientopts", "./internal/runtime/permission", "./internal/service/nxsruntime"], [
    "TestDesktopSandboxPolicySeparatesResourcesAndFullAccess",
    "TestDesktopSandboxDoesNotAlterServerIsolationOrDisabledFeature",
    "TestDesktopSandboxRealRuntimeNegotiation",
    "TestSandboxApprovalKeepsScopeAndDisallowsPersistentRules",
    "TestUnknownApprovalBoundaryFailsBeforeCreatingPending",
    "TestCancelledApprovalDoesNotCreateNewPending",
    "TestSandboxNetworkApprovalUsesOneConnectionScope",
    "TestSandboxDiagnosisDoesNotInventAvailability",
    "TestSandboxDiagnosisRealNXS",
  ]);
  testGroup("host-lifecycle", ["./internal/runtime", "./internal/service/room/realtime"], [
    "TestDesktopSandboxModeChangeRetiresInsteadOfHotUpdate",
    "TestManagerHandlesSandboxModeReplacementAsExpectedTransition",
    "TestManagerReplacesRuntimeForSandboxTransitions",
    "TestRoomSandboxTransitionCancelsApprovalAndClosesWithoutReplay",
    "TestRoomSandboxTransitionDoesNotHideCleanupFailure",
    "TestAgentClientCleanupFailureBlocksReconnect",
    "TestAgentClientStaleStartupCleanupFailureStopsRetry",
    "TestManagerCleanupFailureRetainsSessionFence",
    "TestManagerCleanupFailureRetainsSessionFence/synchronous",
    "TestManagerCleanupFailureRetainsSessionFence/after_timeout",
    "TestCleanupFailureIsNotAnOrdinaryClosedTransport",
    "TestManagerBulkCleanupReportsAndRetainsFailure",
    ...["owner", "idle", "agent_revocation"].map((entry) => `TestManagerBulkCleanupReportsAndRetainsFailure/${entry}`),
    "TestProcessPolicyIncludesHostSandboxRequirements",
  ]);
  if (sdkSource) {
    testGroup("macos-backend-path", ["./internal/tool/builtin/bash/sandboxexec"], [
      "TestMacOSSandboxDependencyUsesSystemPath",
      "TestMacOSSandboxIgnoresTaskPath",
      "TestMacOSSandboxIgnoresTaskPath/shadowed_path",
      "TestMacOSSandboxIgnoresTaskPath/empty_path",
    ], sdkSource, { ...environment, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-resource-denials", ["./internal/tool/executor"], [
      "TestDarwinMandatorySandboxReadDenyPrecedence",
      ...["same_root", "parent_root", "nested_root", "symlink_target"].flatMap((grant) =>
        ["Read", "Bash"].map((tool) => `TestDarwinMandatorySandboxReadDenyPrecedence/${grant}/${tool}`)),
      "TestDarwinMandatorySandboxCannotMoveDeniedAncestor",
      "TestDarwinMandatorySandboxCannotMoveDeniedAncestor/read",
      "TestDarwinMandatorySandboxCannotMoveDeniedAncestor/write",
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-resource-scopes", ["./internal/tool/executor"], [
      "TestDarwinSandboxResourceScopes",
      "TestDarwinSandboxResourceScopes/read-only",
      "TestDarwinSandboxResourceScopes/workspace-write",
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-native", ["./internal/tool/executor"], [
      "TestDarwinSandboxFileToolsRuntime",
      "TestDarwinFileInstructionsShareSandbox",
      "TestDarwinLargeFileReadRetainsFullState",
      "TestDarwinRequiredSandboxChildWriteBoundary",
      "TestDarwinRequiredSandboxBlocksDirectNetwork",
      "TestDarwinRequiredSandboxApprovesPendingNetworkWithoutReplay",
      "TestDarwinRequiredSandboxBackgroundNetworkKeepsCommandApproval",
      "TestDarwinRequiredSandboxBackgroundNetworkKeepsCommandApproval/foreground_completion",
      "TestDarwinRequiredSandboxBackgroundNetworkKeepsCommandApproval/permission_change",
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
  }
  if (sha256(binary) !== report.runtime.sha256) throw new Error("Runtime binary changed during acceptance.");
  report.passed = true;
} catch (error) {
  report.error = error.message;
  console.error(error.message);
  process.exitCode = 1;
} finally {
  report.finishedAt = new Date().toISOString();
  fs.writeFileSync(path.join(reportDirectory, "report.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(`${report.passed ? "PASS" : "FAIL"}: ${report.scope}; installation and release acceptance remain separate.`);
}

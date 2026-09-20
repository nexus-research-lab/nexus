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
    "TestBuildAgentClientOptionsPreservesHostProviderOwnership",
    "TestBuildAgentClientOptionsPreservesHostProviderOwnership/extra",
    "TestBuildAgentClientOptionsPreservesHostProviderOwnership/configuration",
    "TestBuildClaudeOptionsDoNotClaimNXSProviderOwnership",
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
    "TestProviderOwnershipChangeReplacesRuntime",
    ...["NEXUS_PROVIDER_MANAGED_BY_HOST", "NEXUS_SUBPROCESS_ENV_SCRUB", "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB", "NEXUS_AUTO_DREAM_WAKE_MODE"].map((key) => `TestProviderOwnershipChangeReplacesRuntime/${key}`),
    "TestProcessPolicyFingerprintAllowsProviderHotUpdateButRejectsIsolationChange",
  ]);
  testGroup("host-settings-recovery", ["./internal/storage/configuration", "./internal/service/configuration"], [
    "TestRevisionKeyConcurrentInitialization",
    "TestRevisionKeyAcrossProcesses",
    "TestRevisionKeyMigrationPreservesReceipts",
    "TestRevisionKeyRejectsMissingOrCorruptState",
    ...["missing", "malformed", "empty", "unknown_version", "invalid_bootstrap"].map((entry) => `TestRevisionKeyRejectsMissingOrCorruptState/${entry}`),
    "TestConfigurationRevisionSurvivesDatabaseReopen",
    "TestConfigurationRevisionBindsScopeAndSecrets",
    ...["scope", "domain", "target", "version", "secret"].map((entry) => `TestConfigurationRevisionBindsScopeAndSecrets/${entry}`),
    "TestConfigurationRevisionLegacyReceiptIsIncomparable",
    "TestConfigurationReceiptReviewAndHumanReconcileDoesNotReplay",
  ]);
  if (sdkSource) {
    testGroup("provider-environment", ["./client", "./internal/config/env", "./internal/agent/runtime", "./internal/tool/executor/hooks", "./internal/mcp/client"], [
      "TestHostManagedSettingsCannotRedirectProvider",
      ...["project", "flag"].flatMap((source) => ["environment", "provider"].map((shape) => `TestHostManagedSettingsCannotRedirectProvider/${source}/${shape}`)),
      "TestHostManagedSettingsCannotReplaceProviderRequestBody",
      "TestHostManagedSettingsPreserveProviderOwnership",
      "TestStandaloneSettingsRetainProviderInputs",
      "TestHostManagedSubprocessRemovesSDKCredentials",
      ...["OPENAI_API_KEY", "OPENAI_CUSTOM_HEADERS", "NEXUS_WEBSEARCH_API_KEY", "NEXUS_WEBFETCH_SUMMARIZER_API_KEY", "NEXUS_WEBFETCH_DOMAIN_CHECK_API_KEY", "NEXUS_CLIENT_KEY", "NEXUS_CLIENT_KEY_PASSPHRASE", "NEXUS_API_KEY_FILE_DESCRIPTOR", "NEXUS_OAUTH_TOKEN_FILE_DESCRIPTOR", "INPUT_OPENAI_API_KEY"].map((key) => `TestHostManagedSubprocessRemovesSDKCredentials/${key}`),
      "TestHostProviderOwnershipCannotBeDisabledByTaskEnvironment",
      "TestCommandHookRespectsRuntimeProviderOwnership",
      "TestShellProcessesRespectRuntimeProviderOwnership",
      ...["bash", "selected_shell", "powershell_environment"].map((shell) => `TestShellProcessesRespectRuntimeProviderOwnership/${shell}`),
      "TestMemoryModelRespectsHostProviderOwnership",
      "TestMemoryModelRespectsHostProviderOwnership/standalone",
      "TestMemoryModelRespectsHostProviderOwnership/host_managed",
      "TestHostManagedBackgroundSettingDoesNotAcknowledgeUnusedUpdate",
      "TestHTTPHookCannotInterpolateHostProviderCredentials",
      ...["process", "runtime", "standalone"].map((mode) => `TestHTTPHookCannotInterpolateHostProviderCredentials/${mode}`),
      "TestMCPInterpolationCannotBorrowHostProviderCredentials",
      "TestStandaloneMCPInterpolationPreservesEnvironment",
    ], sdkSource);
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
    testGroup("macos-search-contract", ["./cmd/nxs"], [
      "TestSandboxSearchToolsNegotiation",
      "TestSandboxSearchToolsRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxSearchToolsRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-search", ["./internal/tool/executor"], [
      "TestDarwinSandboxSearchToolsRuntime",
      ...["direct", "symlink"].flatMap((target) =>
        ["glob", "content", "count", "files_with_matches"].map((mode) => `TestDarwinSandboxSearchToolsRuntime/${target}/${mode}`)),
      "TestDarwinSandboxSearchAuxiliaryCannotExpandResources",
      "TestDarwinSandboxSearchPreservesResults",
      ...["glob", "absolute_glob", "content", "single_file", "count", "filenames", "no_matches", "suggestion"].map(
        (scenario) => `TestDarwinSandboxSearchPreservesResults/${scenario}`),
      "TestDarwinSandboxSearchPreparationFailsClosed",
      ...["Glob", "Grep"].map((tool) => `TestDarwinSandboxSearchPreparationFailsClosed/${tool}`),
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-media-contract", ["./cmd/nxs"], [
      "TestSandboxMediaFilesNegotiation",
      "TestSandboxMediaFilesRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxMediaFilesRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-media-files", ["./internal/tool/executor"], [
      "TestDarwinSandboxMediaFileSources",
      ...["absolute", "file_url", "workspace_link", "attachment_reference"].map(
        (scenario) => `TestDarwinSandboxMediaFileSources/${scenario}`),
      "TestDarwinSandboxMediaPreprocess",
      ...["user_absolute", "user_file_url", "user_link", "tool_absolute", "tool_file_url", "tool_link"].map(
        (scenario) => `TestDarwinSandboxMediaPreprocess/${scenario}`),
      "TestDarwinSandboxMediaAllowedSources",
      ...["absolute", "relative", "file_url", "attachment_reference", "inline", "user_preprocess", "tool_preprocess"].map(
        (scenario) => `TestDarwinSandboxMediaAllowedSources/${scenario}`),
      "TestDarwinSandboxMediaPreparationFailsClosed",
      ...["view_image", "preprocess"].map((scenario) => `TestDarwinSandboxMediaPreparationFailsClosed/${scenario}`),
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-skill-contract", ["./cmd/nxs"], [
      "TestSandboxSkillFilesNegotiation",
      "TestSandboxSkillFilesRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxSkillFilesRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-skill-files", ["./internal/tool/executor"], [
      "TestDarwinSandboxSkillFileEntrypoints",
      ...["additional", "symlink", "body", "user"].flatMap((source) =>
        ["initial_listing", "slash_catalog", "slash_run", "skill", "discover"].map(
          (entry) => `TestDarwinSandboxSkillFileEntrypoints/${source}/${entry}`)),
      "TestDarwinSandboxSkillAllowedEntrypoints",
      ...["project", "additional", "user", "symlink"].flatMap((source) =>
        ["initial_listing", "slash_catalog", "slash_run", "skill", "discover"].map(
          (entry) => `TestDarwinSandboxSkillAllowedEntrypoints/${source}/${entry}`)),
      "TestDarwinSandboxSkillDynamicDiscovery",
      ...["repository", "no_repository", "gitignored", "denied_body", "conditional"].map(
        (scenario) => `TestDarwinSandboxSkillDynamicDiscovery/${scenario}`),
      "TestDarwinSandboxSkillGitAuxiliary",
      "TestDarwinSandboxSkillMemoryGate",
      "TestDarwinSandboxSkillMemorySettings",
      ...["inline_enabled", "absolute_enabled", "relative_enabled", "relative_disabled"].map(
        (scenario) => `TestDarwinSandboxSkillMemorySettings/${scenario}`),
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-context-contract", ["./cmd/nxs"], [
      "TestSandboxContextFilesNegotiation",
      "TestSandboxContextFilesRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxContextFilesRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-context-startup", ["./internal/agent/runtime"], [
      "TestDarwinSandboxStartupInstructions",
      ...["project", "user", "local", "rule", "symlink", "include", "additional", "managed"].flatMap((source) =>
        [false, true].map((allowed) => `TestDarwinSandboxStartupInstructions/${source}/allowed=${allowed}`)),
    ], sdkSource, { ...environment, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-context-recovery", ["./internal/agent/runtime"], [
      "TestDarwinSandboxCompactFileRestore",
      ...["denied", "symlink", "allowed"].map((source) => `TestDarwinSandboxCompactFileRestore/${source}`),
      "TestDarwinSandboxInstructionSettings",
      ...["user", "project", "local", "relative_flag", "managed", "dropin", "symlink"].flatMap((source) =>
        [false, true].map((allowed) => `TestDarwinSandboxInstructionSettings/${source}/allowed=${allowed}`)),
      "TestDarwinSandboxInstructionReload",
      ...["settings", "cancel", "symlink"].map((failure) => `TestDarwinSandboxInstructionReload/${failure}`),
    ], sdkSource, { ...environment, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-project-contract", ["./cmd/nxs"], [
      "TestSandboxProjectFilesNegotiation",
      "TestSandboxProjectFilesRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxProjectFilesRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-project-files", ["./internal/agent/runtime"], [
      "TestDarwinSandboxProjectFiles",
      ...["project_agent", "user_agent", "project_command", "user_command", "skill", "file_symlink", "directory_symlink", "directory", "hook_settings"].flatMap((source) =>
        [false, true].map((allowed) => `TestDarwinSandboxProjectFiles/${source}/allowed=${allowed}`)),
      "TestDarwinSandboxProjectRefresh",
      ...["symlink", "cancel", "malformed_settings", "changed_agent", "changed_hook"].map(
        (failure) => `TestDarwinSandboxProjectRefresh/${failure}`),
    ], sdkSource, { ...environment, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-managed-contract", ["./cmd/nxs"], [
      "TestSandboxManagedPolicyNegotiation", "TestSandboxManagedPolicyRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxManagedPolicyRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-managed-sources", ["./internal/config", "./client", "./internal/tool/builtin/bash/sandboxexec"], [
      "TestManagedPolicySnapshot", "TestManagedPolicyInvalidSource", "TestManagedPolicyRejectsFIFO",
      ...["empty", "null", "array", "malformed", "deny_string", "deny_element", "managed_only_type", "sandbox_null", "sandbox_deny_type"].map(
        (source) => `TestManagedPolicyInvalidSource/${source}`),
      "TestManagedPolicySettingsCannotRedirectSource", "TestManagedPolicyStartupRejectsUnknown",
      "TestManagedPolicyConfigurationFailure",
      ...["parse", "typed", "provided_error", "disabled", "force_unsandboxed"].map(
        (scenario) => `TestManagedPolicyConfigurationFailure/${scenario}`),
      "TestMandatoryPolicyDoesNotReadTaskSettings",
    ], sdkSource);
    testGroup("macos-managed-integrity", ["./internal/agent/runtime"], [
      "TestDarwinManagedPolicyIntegrity",
      ...["malformed", "deleted", "relaxed"].map((change) => `TestDarwinManagedPolicyIntegrity/${change}`),
    ], sdkSource, { ...environment, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-settings-contract", ["./cmd/nxs"], [
      "TestSandboxSettingsFilesNegotiation", "TestSandboxSettingsFilesRequirement",
      ...["supported", "file_only", "missing_base", "missing_files", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxSettingsFilesRequirement/${scenario}`),
    ], sdkSource);
    testGroup("macos-settings-snapshots", ["./client", "./internal/config/settings", "./internal/agent/runtime", "./cmd/nxs"], [
      "TestSettingsProfileRejectsUnknown",
      ...["user", "project", "flag", "inline"].map((source) => `TestSettingsProfileRejectsUnknown/${source}`),
      "TestSettingsProfileCannotRedirectConfigRoot", "TestSettingsProfileSkipsDisabledSource", "TestSettingsProfileRejectsFIFO",
      "TestSettingsBindingSnapshot", "TestSettingsBindingConcurrentUpdates", "TestSettingsBindingUpdate",
      ...["canceled", "conflict", "success"].map((outcome) => `TestSettingsBindingUpdate/${outcome}`),
      "TestSettingsControlKeepsAppliedState", "TestRuntimeSettingsControlSnapshot",
    ], sdkSource);
    testGroup("settings-writes-contract", ["./cmd/nxs"], [
      "TestSandboxSettingsWritesNegotiation", "TestSandboxSettingsWritesRequirement",
      ...["supported", "missing_base", "missing_files", "missing_settings_requirement", "missing_settings_capability", "missing_writes_capability", "string", "null", "disabled"].map(
        (scenario) => `TestSandboxSettingsWritesRequirement/${scenario}`),
      "TestRuntimeInitializationAdmission", "TestRuntimeTextCLISessionAdmission",
    ], sdkSource);
    testGroup("settings-writers", ["./client", "./internal/config/settings", "./internal/agent/runtime", "./internal/tool/executor", "./internal/tool/builtin/config"], [
      "TestSettingsProfileStaysBelowExplicitOptionsAndEnv",
      "TestDocumentTemporaryPattern", "TestDocumentStoreSharesMissingProjectDirectoryGeneration",
      "TestDocumentStoreWritesTwoDocuments", "TestDocumentStoreAdoptsPhysicalParentCreatedAfterBinding",
      "TestDocumentStorePreflightsAllDocumentSizes", "TestDocumentStoreRejectsLeafSymlink",
      "TestDocumentStoreRejectsParentSymlink", "TestDocumentStoreRejectsParentReplacement",
      "TestDocumentStoreBreaksHardlinkOnWrite", "TestDocumentStorePreservesExistingMode",
      "TestDocumentStorePreservesExistingPermissionBitsUnderUmask", "TestDocumentStoreRejectsExistingReadOnlyLeaf",
      "TestSettingsJournalIsClearedAfterSuccessfulUpdate", "TestSettingsJournalRecoversFullOldState",
      "TestSettingsJournalRecoversFullNewState", "TestSettingsJournalMixedStateFailsClosed",
      "TestSettingsJournalCrossRootMixedStateFailsClosed",
      "TestSettingsJournalDoesNotStoreDocumentPlaintext",
      "TestSettingsBindingUnknownIsShared",
      ...["parent", "child"].map((runtime) => `TestSettingsBindingUnknownIsShared/${runtime}`),
      "TestSettingsBindingConfigTarget",
      ...["user_default", "relative_flag", "absolute_flag", "inline_falls_back_to_user", "no_writable_source"].map(
        (scenario) => `TestSettingsBindingConfigTarget/${scenario}`),
      "TestSettingsBindingConfigNoOp", "TestSettingsBindingConfigRejectsOverride", "TestSettingsBindingConfigRejectsSameValueOverride",
      "TestPermissionSettingsWriteIdentity",
      ...["file_symlink", "directory_symlink", "directory_replaced", "hardlink"].map(
        (scenario) => `TestPermissionSettingsWriteIdentity/${scenario}`),
      "TestDefinitionDisablesConcurrentExecution", "TestRunGetReadsBoundStoreDocument",
      "TestRunGetPrefersCanonicalValueOverLegacyFallback", "TestRunSetWritesCanonicalPathAndPreservesLegacyValue",
      "TestRunSetUsesControlledStoreUpdateAndRequestsRestart", "TestRunSetNoOpDoesNotRequestRestart",
      "TestRunWithoutStoreFailsClosed", "TestRunStoreErrorsDoNotFallBack",
      ...["read", "update"].map((operation) => `TestRunStoreErrorsDoNotFallBack/${operation}`),
      "TestConfigWriteRequiresFreshRuntime",
    ], sdkSource);
    testGroup("settings-request-admission", ["./internal/provider", "./internal/agent/runtime", "./internal/tool/executor"], [
      "TestWithRequestAdmissionGuardsStreamAndComplete", "TestWithRequestAdmissionPreservesOptionalCapabilities",
      "TestRunQueryStopsAllSamplingAfterConfigWriteWithToolSummariesEnabled",
      "TestManualCompactRechecksConfigurationAfterPreCompactHook",
      "TestWebFetchSummaryRechecksConfiguration",
      ...["environment", "adapter"].flatMap((source) => ["unchanged", "changed"].map(
        (change) => `TestWebFetchSummaryRechecksConfiguration/${source}/${change}`)),
    ], sdkSource);
    testGroup("macos-settings-writes", ["./client", "./internal/config/settings", "./internal/tool/executor"], [
      "TestDarwinMandatorySandboxProtectsSettingsTemporaryEntries",
      "TestDarwinMandatorySandboxProtectsLiteralFlagSettingsMetacharacters",
      ...["settings", "temporary"].map((target) => `TestDarwinMandatorySandboxProtectsLiteralFlagSettingsMetacharacters/${target}`),
      "TestDarwinMandatorySandboxProtectsPhysicalSettingsAliases",
      ...["target_physical", "target_logical", "temporary_physical"].map(
        (target) => `TestDarwinMandatorySandboxProtectsPhysicalSettingsAliases/${target}`),
      "TestDarwinMandatorySandboxCannotCreateSettingsHardlink",
      "TestDarwinRequiredSandboxRejectsHardlinkedSettingsProfile", "TestRequiredSandboxBindingRejectsHardlinkedSettings",
    ], sdkSource, { ...environment, NEXUS_FILE_HELPER_TEST_BINARY: binary, NEXUS_SANDBOX_INTEGRATION: "1" });
    testGroup("macos-settings-native", ["./client", "./internal/agent/runtime"], [
      "TestDarwinSettingsProfileFiles",
      ...["user", "project", "local", "flag", "symlink", "filtered"].flatMap((source) =>
        ["denied", "allowed"].map((access) => `TestDarwinSettingsProfileFiles/${source}/${access}`)),
      "TestDarwinSettingsIntegrity",
      ...["malformed", "deleted", "relaxed"].map((change) => `TestDarwinSettingsIntegrity/${change}`),
    ], sdkSource, { ...environment, NEXUS_SANDBOX_INTEGRATION: "1" });
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

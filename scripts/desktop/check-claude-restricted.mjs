#!/usr/bin/env node
// INPUT: A Claude Code executable and an explicitly expected version.
// OUTPUT: JSON evidence for the local CLI's restricted flag and safe rejection paths.
// POS: No-model-request probe; it never supplies a prompt to a command that could reach a model.

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";

const usage = "node scripts/desktop/check-claude-restricted.mjs --expected-version X.Y.Z [--binary /absolute/claude] [--report /absolute/report.json]";
const options = {};
for (let index = 2; index < process.argv.length; index += 2) {
  const key = process.argv[index];
  const value = process.argv[index + 1];
  if (!["--binary", "--expected-version", "--report"].includes(key) || !value || value.startsWith("--") || options[key]) {
    throw new Error(usage);
  }
  options[key] = value;
}
if (!options["--expected-version"]) throw new Error(usage);
if (options["--binary"] && !path.isAbsolute(options["--binary"])) {
  throw new Error("--binary requires an absolute path");
}
if (options["--report"] && !path.isAbsolute(options["--report"])) {
  throw new Error("--report requires an absolute path");
}

const binary = options["--binary"] ?? process.env.CLAUDE_BIN ?? "claude";
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "nexus-claude-restricted-probe-"));
// The probe commands are flag parsing/help only. Remove provider credentials and point
// configuration at an empty temporary directory so a local login cannot become a request.
const env = {
  PATH: process.env.PATH ?? "/usr/bin:/bin",
  HOME: scratch,
  XDG_CONFIG_HOME: path.join(scratch, "config"),
  CLAUDE_CONFIG_DIR: path.join(scratch, "claude-config"),
  CI: "1",
};
fs.mkdirSync(env.XDG_CONFIG_HOME, { recursive: true });
fs.mkdirSync(env.CLAUDE_CONFIG_DIR, { recursive: true });

function run(args) {
  const result = spawnSync(binary, args, {
    env,
    encoding: "utf8",
    timeout: 15_000,
    maxBuffer: 8 << 20,
  });
  if (result.error) throw new Error(`${binary} ${args.join(" ")} failed: ${result.error.message}`);
  return {
    args,
    exitCode: result.status,
    signal: result.signal,
    stdout: result.stdout ?? "",
    stderr: result.stderr ?? "",
    text: `${result.stdout ?? ""}${result.stderr ?? ""}`,
  };
}

const version = run(["--version"]);
const help = run(["--help"]);
const dangerousBypass = run(["--restricted", "--dangerously-skip-permissions"]);
const bypassMode = run(["--restricted", "--permission-mode", "bypassPermissions"]);
const versionText = version.text.trim();
const actualVersion = versionText.match(/\b\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?\b/)?.[0] ?? null;
const restrictedHelp = /--restricted\s+Restricted mode:/s.test(help.text);
const expectedRejection = /bypassPermissions not supported in restricted mode/i;
const report = {
  version: 1,
  probe: "claude-restricted-cli-no-model-request",
  binary,
  expectedVersion: options["--expected-version"],
  actualVersion,
  restrictedHelp,
  rejection: {
    dangerousBypass: { exitCode: dangerousBypass.exitCode, message: expectedRejection.test(dangerousBypass.text) },
    permissionModeBypass: { exitCode: bypassMode.exitCode, message: expectedRejection.test(bypassMode.text) },
  },
  commands: [
    { ...version, text: undefined },
    { ...help, text: undefined },
    { ...dangerousBypass, text: undefined },
    { ...bypassMode, text: undefined },
  ],
  scope: {
    noPromptSupplied: true,
    noModelRequestEvidence: "The probe only invokes --version, --help, and preflight-rejected bypass combinations.",
    cancellationAndCleanup: "not tested; requires an authenticated, isolated Claude session and platform evidence",
  },
};
if (options["--report"]) fs.writeFileSync(options["--report"], `${JSON.stringify(report, null, 2)}\n`);
console.log(JSON.stringify(report, null, 2));

const failed = [
  actualVersion !== options["--expected-version"] && `expected ${options["--expected-version"]}, got ${actualVersion ?? "unknown"}`,
  !restrictedHelp && "--help did not describe --restricted",
  dangerousBypass.exitCode === 0 && "dangerous bypass unexpectedly succeeded",
  !expectedRejection.test(dangerousBypass.text) && "dangerous bypass rejection message missing",
  bypassMode.exitCode === 0 && "bypass permission mode unexpectedly succeeded",
  !expectedRejection.test(bypassMode.text) && "bypass permission mode rejection message missing",
].filter(Boolean);
if (failed.length) {
  console.error(`Claude restricted probe failed: ${failed.join("; ")}`);
  process.exitCode = 1;
}

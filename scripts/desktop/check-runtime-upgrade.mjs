#!/usr/bin/env node
// INPUT: Explicit released/candidate nxs binaries; current pinned Bridge module.
// OUTPUT: Non-skipped upgrade/resume/rollback evidence with binary hashes.
// POS: Runtime data compatibility gate with a local Provider fixture; no external model.
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { requirePassedTests } from "./sandbox-test-evidence.mjs";

if (process.platform !== "darwin") throw new Error("Native macOS is required.");
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const binaries = {
  previous: process.env.NEXUS_SANDBOX_UPGRADE_FROM_BINARY,
  candidate: process.env.NEXUS_SANDBOX_TEST_BINARY,
};
const hash = (file) => createHash("sha256").update(fs.readFileSync(file)).digest("hex");
const report = { version: 1, scope: "runtime-data-compatibility", releaseAccepted: false, passed: false, binaries: {} };
for (const [name, file] of Object.entries(binaries)) {
  if (!file || !path.isAbsolute(file)) throw new Error(`${name} runtime requires an explicit absolute binary path`);
  fs.accessSync(file, fs.constants.X_OK);
  report.binaries[name] = { path: fs.realpathSync(file), sha256: hash(file) };
}
const output = fs.mkdtempSync(path.join(os.tmpdir(), "nexus-runtime-upgrade-"));
console.log(`Runtime upgrade evidence: ${output}`);
const env = { ...process.env, GOWORK: "off" };
delete env.GOFLAGS;
const run = (args) => spawnSync("go", args, { cwd: root, env, encoding: "utf8", timeout: 240_000, maxBuffer: 32 << 20 });
try {
  const module = run(["list", "-mod=readonly", "-m", "-json", "github.com/nexus-research-lab/nexus-agent-sdk-bridge"]);
  if (module.status !== 0) throw new Error("Cannot resolve pinned Bridge");
  report.bridge = JSON.parse(module.stdout);
  if (report.bridge.Replace) throw new Error("Upgrade acceptance requires the pinned Bridge without replacement");
  const parent = "TestDesktopSandboxRuntimeUpgradeAndRollback";
  const required = [parent, ...["before_upgrade", "after_upgrade", "after_rollback"].map((phase) => `${parent}/${phase}`)];
  const result = run(["test", "-mod=readonly", "-json", "./internal/runtime/clientopts", "-run", `^${parent}$`, "-count=1", "-timeout=3m"]);
  fs.writeFileSync(path.join(output, "tests.jsonl"), result.stdout ?? "");
  fs.writeFileSync(path.join(output, "stderr.log"), result.stderr ?? "");
  report.tests = requirePassedTests(result.stdout ?? "", result.status, required);
  report.passed = true;
} finally {
  fs.writeFileSync(path.join(output, "report.json"), `${JSON.stringify(report, null, 2)}\n`);
}

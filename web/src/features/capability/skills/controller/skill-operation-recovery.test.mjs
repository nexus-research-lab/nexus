// @vitest-environment node

import assert from "node:assert/strict";

import path from "node:path";
import { test } from "vitest";

test("Skill recovery locks are exact to operation, Skill, and source", async () => {
  const { skillOperationIntentKey } = await recoveryModule();
  const update = {
    baselineHasUpdate: true,
    kind: "update",
    skillName: "reports",
  };
  assert.notEqual(
    skillOperationIntentKey(update),
    skillOperationIntentKey({ ...update, kind: "delete" }),
  );
  assert.notEqual(
    skillOperationIntentKey(update),
    skillOperationIntentKey({ ...update, skillName: "reports|other" }),
  );
  assert.equal(
    skillOperationIntentKey(update),
    skillOperationIntentKey({ ...update, baselineHasUpdate: false }),
  );

  const external = {
    kind: "import_external",
    skillName: "reports",
    sourceKey: "private-a",
    sourceRef: "reports-v1",
  };
  assert.notEqual(
    skillOperationIntentKey(external),
    skillOperationIntentKey({ ...external, sourceKey: "private-b" }),
  );
  assert.notEqual(
    skillOperationIntentKey(external),
    skillOperationIntentKey({ ...external, sourceRef: "reports-v2" }),
  );

  const local = {
    fileLastModified: 7,
    fileName: "reports.zip",
    fileSize: 42,
    fileType: "application/zip",
    kind: "import_local",
  };
  assert.notEqual(
    skillOperationIntentKey(local),
    skillOperationIntentKey({ ...local, fileLastModified: 8 }),
  );
});

test("only authoritative target states prove supported Skill outcomes", async () => {
  const { reconcileSkillOperation } = await recoveryModule();
  const detail = {
    has_update: false,
    name: "reports",
    source_ref: "owner/repo/reports",
  };

  assert.equal(reconcileSkillOperation({
    baselineHasUpdate: true,
    kind: "update",
    skillName: "reports",
  }, detail), "applied");
  assert.equal(reconcileSkillOperation({
    baselineHasUpdate: false,
    kind: "update",
    skillName: "reports",
  }, detail), "unproven");
  assert.equal(reconcileSkillOperation({
    kind: "delete",
    skillName: "reports",
  }, null), "applied");
  assert.equal(reconcileSkillOperation({
    kind: "delete",
    skillName: "reports",
  }, detail), "unproven");
  assert.equal(reconcileSkillOperation({
    kind: "import_external",
    skillName: "reports",
    sourceKey: "skills-sh",
    sourceRef: "skills.sh:owner/repo/reports",
  }, detail), "applied");
  assert.equal(reconcileSkillOperation({
    kind: "import_external",
    skillName: "reports",
    sourceKey: "skills-sh",
    sourceRef: "other/repo/reports",
  }, detail), "unproven");
});

test("local, Git, and update-check reads cannot invent write success", async () => {
  const { reconcileSkillOperation } = await recoveryModule();
  assert.equal(reconcileSkillOperation({
    fileLastModified: 1,
    fileName: "a.zip",
    fileSize: 1,
    fileType: "application/zip",
    kind: "import_local",
  }, null), "unproven");
  assert.equal(reconcileSkillOperation({
    branch: "main",
    kind: "import_git",
    path: "skills/a",
    url: "https://example.test/repo.git",
  }, null), "unproven");
  assert.equal(reconcileSkillOperation({ kind: "check_updates" }, null), "unproven");
});

function recoveryModule() {
  return import("@/features/capability/skills/controller/skill-operation-recovery.ts");
}

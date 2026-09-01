import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { importLeafTypeScriptModule } from "./import-leaf-typescript-module.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("Agent delete recovery copy follows domain evidence", async () => {
  const { getContactsPagePresentation } = await importLeafTypeScriptModule(
    webRoot,
    "src/pages/contacts/contacts-page-model.ts",
  );
  const present = (deleteFailure) => getContactsPagePresentation({
    contactCount: 1,
    deleteFailure,
    loading: false,
    pendingDeleteAgent: { name: "Researcher" },
    selectedAgent: null,
  }).deleteDialog;

  const notApplied = present({ directoryCheck: "not_checked", kind: "not_applied" });
  assert.equal(notApplied.failure.title, "成员没有删除");
  assert.match(notApplied.failure.impact, /仍然保留/);
  assert.equal(notApplied.confirmText, "删除成员");
  assert.equal(notApplied.variant, "danger");

  const unknown = present({ directoryCheck: "not_checked", kind: "outcome_unknown" });
  assert.match(unknown.failure.title, /无法确认/);
  assert.match(unknown.failure.impact, /删除结果待核对/);
  assert.match(unknown.failure.impact, /没有自动再次删除/);
  assert.match(unknown.failure.nextStep, /不要再次删除/);
  assert.equal(unknown.confirmText, "刷新成员列表");
  assert.equal(unknown.variant, "default");

  const committed = present({
    directoryCheck: "failed",
    kind: "committed_cleanup_incomplete",
  });
  assert.match(committed.failure.title, /已删除/);
  assert.match(committed.failure.impact, /删除已经提交/);
  assert.match(committed.failure.nextStep, /不要再次删除/);

  const stillPresent = present({
    directoryCheck: "target_present",
    kind: "outcome_unknown",
  });
  assert.match(stillPresent.failure.title, /仍在列表中/);
  assert.match(stillPresent.failure.impact, /不能证明先前删除结果/);
  assert.match(stillPresent.failure.impact, /确认前保持保护/);
  assert.match(stillPresent.failure.nextStep, /不要重新删除/);

  assert.doesNotMatch(
    JSON.stringify([notApplied, unknown, committed, stillPresent]),
    /not_applied|outcome_unknown|committed_cleanup_incomplete/,
  );
});

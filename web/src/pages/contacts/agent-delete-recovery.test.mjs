// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("localized deletion states preserve recovery actions for every outcome", async () => {
  const { getContactsPagePresentation } = await import("@/pages/contacts/contacts-page-model.ts");
  const { enNavigationMessages: en } = await import("@/shared/i18n/catalog/en/navigation.ts");
  const { zhNavigationMessages: zh } = await import("@/shared/i18n/catalog/zh/navigation.ts");
  const translate = (catalog) => (key, params = {}) => {
    assert.equal(typeof catalog[key], "string", key);
    return Object.entries(params).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), catalog[key]);
  };
  for (const kind of ["not_applied", "committed_cleanup_incomplete", "resource_absent", "outcome_unknown"]) {
    for (const directoryCheck of ["failed", "not_checked", "target_present"]) {
      const input = {contactCount: 1, deleteFailure: {kind, directoryCheck}, loading: false, pendingDeleteAgent: {name: "Researcher"}, selectedAgent: null};
      const result = getContactsPagePresentation({...input, t: translate(en)}).deleteDialog;
      assert.equal(result.variant, getContactsPagePresentation({...input, t: translate(zh)}).deleteDialog.variant);
      assert.equal(result.confirmText, kind === "not_applied" ? "Delete member" : "Refresh members");
      assert.match(result.message, /Researcher/);
      assert.doesNotMatch(JSON.stringify(result), /[\u4e00-\u9fff]|contacts\.delete\./);
      assert.ok(result.failure.title && result.failure.impact && result.failure.nextStep);
    }
  }
});

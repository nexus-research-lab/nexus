import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("personal mutations use safe copy and distinguish rejected, committed, and unknown results", async () => {
  const { buildPersonalMutationFailure } = await server.ssrLoadModule(
    "/src/features/settings/personal/use-personal-settings-controller.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { projectMutationFailure } = await server.ssrLoadModule(
    "/src/lib/error-message.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];
  const privateDetail = "/private/user/path request-id=secret";

  const rejected = buildPersonalMutationFailure(
    projectMutationFailure(new ApiRequestError(privateDetail, 400, {
      version: 1,
      code: "profile.rejected",
      category: "validation",
      effect: "not_applied",
    }), "safe fallback"),
    "avatar",
    t,
  );
  assert.equal(rejected.tone, "error");
  assert.match(rejected.impact, /没有更改/);
  assert.doesNotMatch(JSON.stringify(rejected), /private\/user|request-id|secret/);

  const committed = buildPersonalMutationFailure(
    projectMutationFailure(new ApiRequestError(privateDetail, 500, {
      version: 1,
      code: "password.changed",
      category: "internal",
      effect: "committed",
    }), "safe fallback"),
    "password",
    t,
  );
  assert.equal(committed.tone, "warning");
  assert.match(committed.impact, /已经修改/);
  assert.match(committed.nextStep, /不要再次提交/);
  assert.doesNotMatch(JSON.stringify(committed), /private\/user|request-id|secret/);

  const unknown = buildPersonalMutationFailure(
    projectMutationFailure(
      new ApiTransportError(privateDetail, "response_interrupted", "unknown"),
      "safe fallback",
    ),
    "password",
    t,
  );
  assert.equal(unknown.tone, "warning");
  assert.match(unknown.title, /无法确认/);
  assert.match(unknown.nextStep, /不要重复提交/);
  assert.doesNotMatch(JSON.stringify(unknown), /private\/user|request-id|secret/);
});

test("workspace read reconciliation updates the saved path without discarding a draft", async () => {
  const model = await server.ssrLoadModule(
    "/src/features/settings/general/model/workspace-settings-model.ts",
  );
  const current = model.replaceWorkspaceDraft(
    model.buildStateRootSettingsSnapshot({ current_path: "/old" }),
    "/draft",
  );
  const reconciled = model.reconcileStateRootSettingsSnapshot(current, {
    current_path: "/actual",
  });
  assert.equal(reconciled.currentPath, "/actual");
  assert.equal(reconciled.savedPath, "/actual");
  assert.equal(reconciled.draftPath, "/draft");
  assert.equal(model.canSaveWorkspaceSettings(reconciled, false), true);
});

test("settings recovery copy is available in both locales", async () => {
  const [{ zhMessages }, { enMessages }] = await Promise.all([
    server.ssrLoadModule("/src/shared/i18n/catalog/zh/index.ts"),
    server.ssrLoadModule("/src/shared/i18n/catalog/en/index.ts"),
  ]);
  const keys = [
    "settings.desktop.export_logs_failed_impact",
    "settings.desktop.export_logs_failed_next_step",
    "settings.general.state_root_unknown_impact",
    "settings.general.state_root_unknown_next_step",
    "settings.general.default_model_catalog_failed_impact",
    "settings.general.default_model_catalog_failed_next_step",
    "settings.personal.avatar_unknown_impact",
    "settings.personal.avatar_unknown_next_step",
    "settings.personal.password_unknown_impact",
    "settings.personal.password_unknown_next_step",
  ];
  for (const key of keys) {
    assert.ok(zhMessages[key], `missing zh copy for ${key}`);
    assert.ok(enMessages[key], `missing en copy for ${key}`);
  }
});

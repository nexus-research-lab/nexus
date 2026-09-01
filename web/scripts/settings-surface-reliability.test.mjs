import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
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

test("desktop and workspace bridge failures are complete, untruncated, and reconciled read-only", async () => {
  const [desktopHook, desktopView, workspaceHook, workspaceView] = await Promise.all([
    read("src/features/settings/general/use-desktop-settings.ts"),
    read("src/features/settings/general/sections/settings-desktop-section.tsx"),
    read("src/features/settings/general/use-workspace-settings.ts"),
    read("src/features/settings/general/sections/settings-workspace-section.tsx"),
  ]);

  assert.doesNotMatch(desktopHook, /getErrorMessage/);
  assert.match(desktopHook, /export_logs_failed_impact/);
  assert.match(desktopHook, /export_logs_failed_next_step/);
  assert.match(desktopView, /version_failed_impact/);
  assert.match(desktopView, /version_failed_next_step/);
  assert.doesNotMatch(desktopView, /truncate/);

  assert.doesNotMatch(workspaceHook, /getErrorMessage/);
  assert.match(workspaceHook, /pendingMigrationTargetRef/);
  assert.match(workspaceHook, /migrationUnconfirmedRef/);
  assert.match(workspaceHook, /getDesktopStateRoot\(\)/);
  assert.match(workspaceHook, /reconcileStateRootSettingsSnapshot/);
  assert.match(workspaceHook, /busy \|\| migrationUnconfirmed/);
  assert.match(workspaceHook, /state_root_unknown_impact/);
  assert.match(workspaceHook, /state_root_unknown_next_step/);
  assert.doesNotMatch(workspaceView, /truncate/);
  assert.match(workspaceView, /FeedbackBannerViewport/);
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

test("default model, runtime, and permission failures use domain recovery instead of raw one-line errors", async () => {
  const [modelHook, behaviorView, modelRow, runtimeHook, runtimeView, permissionsView] = await Promise.all([
    read("src/features/settings/general/use-default-model-preferences.ts"),
    read("src/features/settings/general/sections/settings-general-behavior-section.tsx"),
    read("src/features/settings/general/components/settings-default-model-row.tsx"),
    read("src/features/settings/runtime/use-runtime-settings-controller.ts"),
    read("src/features/settings/runtime/settings-runtime-section.tsx"),
    read("src/features/settings/general/sections/settings-permissions-section.tsx"),
  ]);

  assert.doesNotMatch(modelHook, /getErrorMessage/);
  assert.match(
    modelHook,
    /current\.runtimeKind === agentRuntimeKind[\s\S]*\.\.\.current/,
  );
  assert.match(modelHook, /retryCatalog/);
  assert.match(behaviorView, /default_model_catalog_failed_impact/);
  assert.match(behaviorView, /default_model_catalog_failed_next_step/);
  assert.doesNotMatch(modelRow, /feedbackMessage|truncate/);

  assert.doesNotMatch(runtimeHook, /getErrorMessage|status\.message/);
  assert.match(runtimeHook, /kernel_check_not_changed_impact/);
  assert.match(runtimeHook, /kernel_check_next_step/);
  assert.match(runtimeView, /web_search_anysearch_params_invalid_impact/);
  assert.match(runtimeView, /web_search_anysearch_params_invalid_next_step/);
  assert.match(permissionsView, /PreferencesReliabilityNotice/);
  assert.doesNotMatch(permissionsView, /feedbackMessage/);
});

test("personal unknown mutations lock repeat submission until an explicit new intent", async () => {
  const [controller, passwordView] = await Promise.all([
    read("src/features/settings/personal/use-personal-settings-controller.ts"),
    read("src/features/settings/personal/personal-password-section.tsx"),
  ]);

  assert.match(controller, /setAvatarMutationBlocked\(blocked\)/);
  assert.match(controller, /setPasswordMutationBlocked\(blocked\)/);
  assert.match(controller, /avatarMutationBlocked/);
  assert.match(controller, /passwordMutationBlocked/);
  assert.match(controller, /startNewAvatarIntent/);
  assert.match(controller, /startNewPasswordIntent/);
  assert.doesNotMatch(controller, /getErrorMessage|failure\.message/);
  assert.match(passwordView, /disabled=\{isSubmitting \|\| mutationBlocked\}/);
  assert.match(passwordView, /state\.validation_failure_impact/);
  assert.match(passwordView, /state\.validation_failure_next_step/);
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

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

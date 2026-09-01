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

test("preference draft rebase preserves unrelated newer server fields", async () => {
  const {
    buildPreferencesUpdatePayload,
    equivalentPreferences,
    normalizePreferences,
    rebasePreferenceDraft,
  } = await server.ssrLoadModule(
    "/src/features/settings/general/model/settings-preferences-model.ts",
  );
  const base = preference({
    version: 4,
    emotion_enabled: false,
    default_agent_options: {
      permission_mode: "default",
      provider: "provider-a",
      model: "model-a",
    },
  });
  const draft = preference({
    ...base,
    default_agent_options: {
      ...base.default_agent_options,
      permission_mode: "acceptEdits",
    },
  });
  const latest = preference({
    ...base,
    version: 5,
    emotion_enabled: true,
    default_agent_options: {
      ...base.default_agent_options,
      provider: "provider-b",
      model: "model-b",
    },
  });

  const rebased = rebasePreferenceDraft(base, draft, latest);
  assert.equal(rebased.version, 5);
  assert.equal(rebased.emotion_enabled, true);
  assert.equal(rebased.default_agent_options.provider, "provider-b");
  assert.equal(rebased.default_agent_options.model, "model-b");
  assert.equal(rebased.default_agent_options.permission_mode, "acceptEdits");
  assert.equal(
    normalizePreferences({ ...latest, echo_enabled: true }).echo_enabled,
    true,
    "Preferences keeps Echo in its authoritative read snapshot",
  );

  const payload = buildPreferencesUpdatePayload({
    ...rebased,
    browser_cdp_enabled: true,
    echo_enabled: true,
  });
  assert.equal(payload.browser_cdp_enabled, true);
  assert.equal("echo_enabled" in payload, false, "Echo keeps its cleanup-aware endpoint");
  assert.equal("version" in payload, false, "server version belongs in If-Match, not the patch body");
  assert.equal(
    equivalentPreferences(latest, { ...latest, web_search_api_key: "new-secret" }),
    false,
    "a GET response cannot prove that an unresolved raw API key draft was committed",
  );
  assert.equal(
    equivalentPreferences(latest, { ...latest, web_search_api_key: "" }),
    false,
    "a GET response cannot prove that an unresolved API key clear was committed",
  );
});

test("preferences writes put the aggregate revision in If-Match", async () => {
  const { updateUserPreferencesApi } = await server.ssrLoadModule(
    "/src/lib/api/settings/preferences-api.ts",
  );
  const previousFetch = globalThis.fetch;
  let requestInit;
  globalThis.fetch = async (_input, init) => {
    requestInit = init;
    return new Response(JSON.stringify({
      code: "0000",
      message: "success",
      success: true,
      data: preference({ version: 8 }),
    }), {
      headers: { "Content-Type": "application/json" },
      status: 200,
    });
  };
  try {
    await updateUserPreferencesApi(
      { emotion_enabled: true },
      { expectedVersion: 7 },
    );
  } finally {
    globalThis.fetch = previousFetch;
  }
  assert.equal(new Headers(requestInit.headers).get("If-Match"), '"preferences-7"');
});

test("preferences writes use CAS while reconciliation remains read-only", async () => {
  const [api, hook] = await Promise.all([
    read("src/lib/api/settings/preferences-api.ts"),
    read("src/features/settings/general/use-user-preferences.ts"),
  ]);

  assert.match(api, /"If-Match": `"preferences-\$\{expectedVersion\}"`/);
  assert.match(hook, /setWritable\(false\)[\s\S]*getUserPreferencesApi\(\)/);
  assert.match(hook, /projectMutationFailure\(/);
  assert.match(
    hook,
    /pendingRef\.current = \{[\s\S]*base,[\s\S]*draft: optimistic,[\s\S]*latest: null,[\s\S]*projectionRepairRequired:/,
  );
  assert.match(hook, /showDraft\(optimistic\)[\s\S]*setWritable\(true\)/);

  const checkLatestBody = hook.slice(
    hook.indexOf("const checkLatest"),
    hook.indexOf("const repairProjectionSnapshot"),
  );
  assert.match(checkLatestBody, /getUserPreferencesApi\(\)/);
  assert.doesNotMatch(checkLatestBody, /updateUserPreferencesApi\(/);
  const reapplyBody = hook.slice(
    hook.indexOf("const reapplyDraft"),
    hook.indexOf("const updatePreferences"),
  );
  assert.match(reapplyBody, /rebasePreferenceDraft\(/);
  assert.match(reapplyBody, /persistAtVersion\(rebased, pending\.latest\)/);
  assert.match(hook, /preferences\.projection_result_unknown/);
  assert.match(hook, /const repairProjectionSnapshot/);
  assert.match(
    hook,
    /buildPreferencesUpdatePayload\(latest\)[\s\S]*expectedVersion: latest\.version/,
  );
  assert.match(hook, /projectionRepairRequiredFeedback/);
  assert.match(hook, /projectionRepairCompletedFeedback/);
  assert.match(
    hook,
    /failure\.code === "preferences\.projection_result_unknown"/,
  );
});

test("preference recovery copy answers problem, impact, and next step", async () => {
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  for (const prefix of [
    "settings.general.preferences_load",
    "settings.general.preferences_conflict",
    "settings.general.preferences_unknown",
    "settings.general.preferences_not_applied",
  ]) {
    assert.ok(zhMessages[`${prefix}_title`]);
    assert.ok(zhMessages[`${prefix}_impact`]);
    assert.ok(zhMessages[`${prefix}_next_step`]);
  }
  assert.ok(zhMessages["settings.general.preferences_projection_repair_impact"]);
  assert.ok(zhMessages["settings.general.preferences_projection_repair_next_step"]);
  assert.doesNotMatch(
    zhMessages["settings.general.preferences_reconciled_not_applied_title"],
    /确认.*未保存/,
  );
});

test("runtime availability failures cannot trigger preference reconciliation", async () => {
  const [controller, section] = await Promise.all([
    read("src/features/settings/runtime/use-runtime-settings-controller.ts"),
    read("src/features/settings/runtime/settings-runtime-section.tsx"),
  ]);

  assert.match(controller, /const \[runtimeFeedback, setRuntimeFeedback\]/);
  assert.match(controller, /setRuntimeFeedback\(\{/);
  assert.doesNotMatch(controller, /setFeedback\(/);
  assert.match(
    section,
    /feedback=\{settings\.preferencesFeedback \?\? settings\.runtimeFeedback\}/,
  );
  assert.match(
    section,
    /recovery=\{settings\.preferencesFeedback[\s\S]*\? settings\.preferencesRecovery[\s\S]*: undefined\}/,
  );
});

test("late preference responses are fenced to the current authenticated owner", async () => {
  const hook = await read(
    "src/features/settings/general/use-user-preferences.ts",
  );

  assert.match(hook, /captureAuthOwnerScopeGeneration\(\)/);
  assert.match(hook, /isAuthOwnerScopeGenerationCurrent\(ownerGeneration\)/);
  assert.match(hook, /authOwnerReloadKey/);
  assert.match(hook, /saveRequestRef/);
  assert.match(hook, /checkRequestRef/);
});

function preference(overrides = {}) {
  return {
    version: 1,
    chat_default_delivery_policy: "queue",
    agent_runtime_kind: "nxs",
    default_agent_options: {
      permission_mode: "default",
      allowed_tools: [],
      disallowed_tools: [],
      setting_sources: ["project"],
    },
    ...overrides,
  };
}

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

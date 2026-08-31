import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const read = (path) => readFile(new URL(`../${path}`, import.meta.url), "utf8");

test("Echo writes use the shared Preferences revision and never blind-retry", async () => {
  const [api, hook] = await Promise.all([
    read("src/lib/api/settings/echo-api.ts"),
    read("src/features/settings/general/use-echo-settings.ts"),
  ]);

  assert.match(api, /"If-Match": `"echo-\$\{options\.expectedVersion\}"`/);
  assert.match(hook, /projectMutationFailure\(/);
  assert.match(hook, /pendingRef\.current = \{/);
  assert.doesNotMatch(hook, /previousPending\?\.desired \?\? desired/);
  assert.match(hook, /failure\.code === "echo\.cleanup_incomplete"/);
  assert.match(hook, /failure\.effect === "committed"/);
  assert.match(hook, /failure\.effect === "not_applied"/);
  assert.match(hook, /!pending\.desired && !settings\.enabled/);
  assert.match(hook, /captureAuthOwnerScopeGeneration\(\)/);
  assert.match(hook, /isAuthOwnerScopeGenerationCurrent\(ownerGeneration\)/);
  assert.match(hook, /aggregateVersion/);
  assert.match(hook, /authoritativeRef\.current = echoSettingsAtAggregateVersion\(current, aggregateVersion\)/);
  assert.match(hook, /\|\| pendingRef\.current/);
  assert.match(hook, /submitAtVersion\([\s\S]*echoSettingsAtAggregateVersion\(base, aggregateVersion\)/);
  assert.doesNotMatch(hook, /setTimeout|setInterval/);
});

test("Echo adopts proven revisions from other writes in the Preferences aggregate", async () => {
  const [controller, preferences] = await Promise.all([
    read("src/features/settings/general/use-general-settings-controller.ts"),
    read("src/features/settings/general/use-user-preferences.ts"),
  ]);
  assert.match(controller, /aggregateVersion: preferences\.version/);
  assert.match(preferences, /hasUnresolvedMutation: pendingRef\.current !== null/);
});

test("a failed General read does not block an independently loaded Echo setting", async () => {
  const controller = await read("src/features/settings/general/use-general-settings-controller.ts");
  assert.match(controller, /blocked: loading \|\| saving \|\| recovery\.checking \|\| hasUnresolvedMutation/);
  assert.doesNotMatch(controller, /blocked: preferencesBusy/);
  assert.doesNotMatch(controller, /blocked: [^\n]*!writable/);
});

test("late Echo responses cannot publish into a new owner scope", async () => {
  const hook = await read("src/features/settings/general/use-echo-settings.ts");
  const generationGuards = hook.match(/isAuthOwnerScopeGenerationCurrent\(ownerGeneration\)/g) ?? [];
  assert.ok(generationGuards.length >= 6, `owner guards=${generationGuards.length}`);
  assert.match(hook, /saveRequestRef\.current \+= 1/);
  assert.match(hook, /checkRequestRef\.current \+= 1/);
  assert.match(hook, /saveRequestRef\.current !== requestId/);
  assert.match(hook, /checkRequestRef\.current !== requestId/);
});

test("Echo recovery separates read-only checking from explicit writes", async () => {
  const hook = await read("src/features/settings/general/use-echo-settings.ts");
  const checkBody = hook.slice(
    hook.indexOf("const checkLatest"),
    hook.indexOf("const useLatest"),
  );
  assert.match(checkBody, /getEchoApi\(\)/);
  assert.doesNotMatch(checkBody, /updateEchoApi\(/);
  assert.match(hook, /submitAtVersion\(pending\.latest\.enabled, pending\.latest, "use-latest"\)/);
  assert.match(hook, /submitAtVersion\(pending\.desired, pending\.latest, "reapply"\)/);
  assert.match(hook, /submitAtVersion\(false, pending\.latest, "finish-disabling"\)/);
});

test("Echo visible recovery copy includes problem, impact, and next step", async () => {
  const [zh, en, notice] = await Promise.all([
    read("src/shared/i18n/catalog/zh/settings.ts"),
    read("src/shared/i18n/catalog/en/settings.ts"),
    read("src/features/settings/general/components/echo-settings-reliability-notice.tsx"),
  ]);
  for (const prefix of [
    "echo_load_failure",
    "echo_conflict",
    "echo_unknown",
    "echo_not_applied",
    "echo_cleanup_incomplete",
    "echo_cleanup_repair",
    "echo_committed_needs_check",
    "echo_recovery_not_applied",
    "echo_difference",
    "echo_check_failure_pending",
  ]) {
    for (const suffix of ["title", "message", "impact", "next_step"]) {
      const key = `settings.general.${prefix}_${suffix}`;
      assert.ok(zh.includes(`"${key}"`), `missing zh ${key}`);
      assert.ok(en.includes(`"${key}"`), `missing en ${key}`);
    }
  }
  assert.match(notice, /feedback\.title/);
  assert.match(notice, /feedback\.message/);
  assert.match(notice, /feedback\.impact/);
  assert.match(notice, /feedback\.nextStep/);
});

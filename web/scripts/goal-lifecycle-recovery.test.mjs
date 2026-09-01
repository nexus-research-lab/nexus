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

test("Goal lifecycle reads only unlock outcomes proven by authoritative state", async () => {
  const {
    createGoalLifecycleIntent,
    reconcileGoalLifecycleIntent,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-lifecycle-recovery.ts",
  );
  const base = goal();

  const clear = createGoalLifecycleIntent(base, base.session_key, {
    operation: "clear",
  });
  assert.equal(clear.targetGoalId, base.id);
  assert.equal(clear.sessionKey, base.session_key);
  assert.equal(clear.baseVersion, base.version);
  assert.equal(reconcileGoalLifecycleIntent(clear, base), "unproven");
  assert.equal(
    reconcileGoalLifecycleIntent(clear, { ...base, version: base.version + 1 }),
    "unproven",
  );
  assert.equal(reconcileGoalLifecycleIntent(clear, null), "target_not_current");

  const pause = createGoalLifecycleIntent(base, base.session_key, {
    operation: "pause",
  });
  assert.equal(reconcileGoalLifecycleIntent(pause, base), "unproven");
  assert.equal(
    reconcileGoalLifecycleIntent(pause, {
      ...base,
      status: "paused",
      version: base.version + 1,
    }),
    "applied",
  );

  const resumeBase = goal({ status: "paused" });
  const resume = createGoalLifecycleIntent(resumeBase, resumeBase.session_key, {
    operation: "resume",
  });
  assert.equal(reconcileGoalLifecycleIntent(resume, resumeBase), "unproven");
  assert.equal(
    reconcileGoalLifecycleIntent(resume, {
      ...resumeBase,
      continuation_state: "ready",
      status: "active",
      version: resumeBase.version + 1,
    }),
    "applied",
  );

  const suppressedBase = goal({ continuation_state: "suspended" });
  const suppressedResume = createGoalLifecycleIntent(
    suppressedBase,
    suppressedBase.session_key,
    { operation: "resume" },
  );
  assert.equal(
    reconcileGoalLifecycleIntent(suppressedResume, suppressedBase),
    "unproven",
  );
  assert.equal(
    reconcileGoalLifecycleIntent(suppressedResume, {
      ...suppressedBase,
      continuation_state: "ready",
      version: suppressedBase.version + 1,
    }),
    "applied",
  );
});

test("objective rewrite or unrelated Goal progress never proves an unknown update", async () => {
  const {
    createGoalLifecycleIntent,
    reconcileGoalLifecycleIntent,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-lifecycle-recovery.ts",
  );
  const base = goal();
  const update = createGoalLifecycleIntent(base, base.session_key, {
    objective: "Prepare the release",
    operation: "update",
    tokenBudget: 1200,
  });

  assert.equal(reconcileGoalLifecycleIntent(update, base), "unproven");
  assert.equal(
    reconcileGoalLifecycleIntent(update, {
      ...base,
      objective: "Prepare a production-ready release plan",
      token_budget: 1200,
      version: base.version + 1,
    }),
    "unproven",
    "a server-rewritten objective has no exact mutation identity in the current read model",
  );
  assert.equal(
    reconcileGoalLifecycleIntent(update, {
      ...base,
      objective: "Prepare the release",
      token_budget: 1200,
      version: base.version + 1,
    }),
    "applied",
  );
  assert.equal(
    reconcileGoalLifecycleIntent(update, goal({
      id: "goal-new",
      session_key: base.session_key,
    })),
    "target_not_current",
  );
  assert.equal(
    reconcileGoalLifecycleIntent(update, {
      ...base,
      objective: "Prepare the release",
      session_key: "room:other",
      token_budget: 1200,
      version: base.version + 1,
    }),
    "unproven",
  );
  assert.equal(
    reconcileGoalLifecycleIntent(update, goal({
      id: "goal-other-scope",
      session_key: "room:other",
    })),
    "unproven",
    "a cross-session response must fail closed even when the Goal ID differs",
  );
});

test("Goal lifecycle recovery is wired to the panel with PIR copy and read-only recovery", async () => {
  const [resource, controller, panel, notice, zhMessages] = await Promise.all([
    read("src/features/conversation/shared/goal/use-goal-resource.ts"),
    read("src/features/conversation/shared/goal/use-goal-controller.ts"),
    read("src/features/conversation/shared/goal/goal-panel.tsx"),
    read("src/features/conversation/shared/goal/goal-reliability-notice.tsx"),
    server.ssrLoadModule("/src/shared/i18n/catalog/zh/index.ts")
      .then((module) => module.zhMessages),
  ]);

  assert.match(resource, /projectMutationFailure\(error/);
  assert.match(resource, /mutationFailure\.effect === "not_applied"[\s\S]*\? null/);
  assert.match(resource, /createGoalLifecycleIntent\(goal, sessionKey, input\)/);
  assert.match(resource, /reconcileGoalLifecycleIntent\(locked\.intent, currentGoal\)/);
  assert.match(resource, /mutation_committed_refresh_failed/);
  assert.match(resource, /subscribeAuthOwnerScopeGeneration/);
  assert.match(resource, /isAuthOwnerScopeGenerationCurrent/);
  assert.match(resource, /previousOwnerScopeGenerationRef/);
  assert.match(resource, /goal\.session_key !== sessionKey/);
  assert.match(resource, /validGoalCommandResult\(transaction, updated\)/);
  assert.match(resource, /blocksMutations: result\.stage !== "binding"/);
  assert.match(
    resource,
    /kind: locked\.effect === "committed"[\s\S]*\? "mutation_committed_refresh_failed"/,
    "a committed write must not become an unknown write when the follow-up Goal read fails",
  );
  assert.match(
    controller,
    /await clearGoalApi\(goalId\);[\s\S]*return null;/,
    "a successful concurrent-already-absent clear must not restore the stale Goal",
  );
  assert.match(
    controller,
    /setDraft\(null\);[\s\S]*setDialog\(EMPTY_GOAL_DIALOG\);[\s\S]*ownerScopeGeneration, sessionKey/,
    "drafts and dialogs must not cross Session or owner scope",
  );
  assert.match(
    controller,
    /reliability\?\.operation === "update"[\s\S]*mutation_committed_refresh_failed[\s\S]*setDraft\(null\)/,
    "a confirmed update must close the submitted draft instead of inviting a duplicate save",
  );
  assert.doesNotMatch(resource, /pauseGoalApi|resumeGoalApi|updateGoalApi|clearGoalApi/);
  assert.match(panel, /<GoalReliabilityNotice/);
  assert.match(panel, /mutationBlocked=\{controller\.mutationsBlocked\}/);
  assert.match(panel, /mutationBlockReason=\{controller\.mutationBlockReason\}/);
  assert.match(notice, /copy\.problem/);
  assert.match(notice, /copy\.impact/);
  assert.match(notice, /copy\.nextStep/);
  assert.match(notice, /aria-live="polite"/);
  assert.match(notice, /role="status"/);
  assert.doesNotMatch(notice, /copy\.tone === "error" \? "alert"/);
  assert.match(notice, /state\.reload_check/);

  for (const keys of [
    ["read_problem", "read_stale_impact", "read_next_step"],
    ["binding_problem", "binding_impact", "binding_next_step"],
    ["not_applied_problem", "not_applied_impact", "not_applied_next_step"],
    ["accepted_problem", "accepted_impact", "accepted_next_step"],
    ["unknown_problem", "unknown_impact", "unknown_next_step"],
    ["reconcile_failed_problem", "reconcile_failed_impact", "reconcile_failed_next_step"],
    ["unproven_problem", "unproven_impact", "unproven_next_step"],
    ["applied_problem", "applied_impact", "applied_partial_impact", "applied_next_step"],
    ["committed_problem", "committed_impact", "committed_next_step"],
    ["target_not_current_problem", "target_not_current_impact", "target_not_current_partial_impact", "target_not_current_next_step"],
    ["committed_refresh_problem", "committed_refresh_impact", "committed_refresh_next_step"],
    ["runtime_problem", "runtime_impact", "runtime_next_step"],
    ["runtime_budget_problem", "runtime_budget_impact", "runtime_budget_next_step"],
    ["runtime_usage_problem", "runtime_usage_impact", "runtime_usage_next_step"],
  ]) {
    for (const key of keys) {
      assert.ok(zhMessages[`goal.reliability.${key}`]);
    }
  }
  assert.ok(zhMessages["goal.reliability.action_locked"]);
  assert.ok(zhMessages["goal.reliability.action_stale"]);
});

function goal(overrides = {}) {
  return {
    id: "goal-current",
    session_key: "agent:nexus:ws:dm:goal-recovery",
    objective: "Ship safely",
    status: "active",
    token_budget: 800,
    continuation_count: 1,
    empty_progress_count: 0,
    continuation_state: "ready",
    version: 7,
    created_at: "2026-08-28T08:00:00Z",
    updated_at: "2026-08-28T08:05:00Z",
    usage_finalized: false,
    ...overrides,
  };
}

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

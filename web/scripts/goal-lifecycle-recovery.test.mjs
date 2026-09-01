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

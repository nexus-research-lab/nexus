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

const translate = (key, params = {}) => {
  const operation = params.operation ? `:${params.operation}` : "";
  return `${key}${operation}`;
};

test("subscription mutation feedback blocks an unknown repeat and keeps raw exceptions private", async () => {
  const { buildSubscriptionMutationFailure } = await server.ssrLoadModule(
    "/src/features/settings/operations/subscription-admin/subscription-admin-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );

  const transportFailure = buildSubscriptionMutationFailure(
    translate,
    "plan-create",
    new ApiTransportError("private upstream body", "network", "unknown"),
  );
  assert.equal(transportFailure.blocksMutation, true);
  assert.equal(transportFailure.recoveryAction, "refresh");
  assert.equal(transportFailure.tone, "warning");
  assert.equal(
    transportFailure.message,
    "settings.subscription.plan_create_failed_message",
  );
  assert.ok(transportFailure.impact);
  assert.ok(transportFailure.nextStep);

  const rejected = buildSubscriptionMutationFailure(
    translate,
    "plan-save",
    new ApiRequestError("权限不足", 403, {
      version: 1,
      category: "authorization",
      code: "subscription.forbidden",
      effect: "not_applied",
    }),
  );
  assert.equal(rejected.blocksMutation, false);
  assert.equal(rejected.recoveryAction, undefined);
  assert.equal(rejected.tone, "error");
  assert.equal(rejected.message, "settings.subscription.plan_save_failed_message");
});

test("project reads never expose ordinary Error details", async () => {
  const { buildProjectFeedback } = await server.ssrLoadModule(
    "/src/features/settings/operations/project-admin/project-admin-model.ts",
  );
  const feedback = buildProjectFeedback(
    translate,
    "load-failed",
    new Error("/private/path provider-secret SQL table"),
  );

  assert.equal(feedback.message, "settings.projects.load_failed_message");
  assert.ok(feedback.impact);
  assert.ok(feedback.nextStep);
  assert.equal(feedback.recoveryAction, "refresh");
});

test("OAuth callback and Goal Composer do not render raw exception text", async () => {
  const oauthCallback = await read(
    "src/pages/connectors/connector-oauth-callback-page.tsx",
  );
  const oauthPoller = await read(
    "src/features/capability/connectors/auth/device-flow/connector-device-auth-poller.ts",
  );
  const goalActions = await read(
    "src/features/conversation/shared/composer/controller/use-composer-goal-actions.ts",
  );

  assert.doesNotMatch(oauthCallback, /error_description/);
  assert.doesNotMatch(oauthCallback, /err instanceof Error/);
  assert.doesNotMatch(oauthPoller, /error\.message/);
  assert.doesNotMatch(goalActions, /error instanceof Error \? error\.message/);
  assert.match(goalActions, /requiresGoalReconciliation/);
  assert.match(goalActions, /failureImpact/);
  assert.match(goalActions, /failureNextStep/);
});

test("workspace command feedback uses the safe error projection", async () => {
  const workspaceCommands = await read(
    "src/features/conversation/room/workspace/controller/use-workspace-commands.ts",
  );

  assert.match(workspaceCommands, /getErrorMessage\(/);
  assert.doesNotMatch(workspaceCommands, /return error\.message/);
  assert.match(workspaceCommands, /workspace_read_action_failed_impact/);
  assert.match(workspaceCommands, /workspace_read_action_failed_next/);
});

test("OAuth pending connector correlation stores only a validated connector ID", async () => {
  const storage = new Map();
  globalThis.window = {
    opener: null,
    sessionStorage: {
      getItem: (key) => storage.get(key) ?? null,
      removeItem: (key) => storage.delete(key),
      setItem: (key, value) => storage.set(key, value),
    },
  };
  const {
    clearPendingConnectorOauth,
    readPendingConnectorOauth,
    rememberPendingConnectorOauth,
  } = await server.ssrLoadModule(
    "/src/features/capability/connectors/auth/connector-oauth-events.ts",
  );

  rememberPendingConnectorOauth("github");
  assert.equal(readPendingConnectorOauth(), "github");
  clearPendingConnectorOauth("other");
  assert.equal(readPendingConnectorOauth(), "github");
  clearPendingConnectorOauth("github");
  assert.equal(readPendingConnectorOauth(), null);

  rememberPendingConnectorOauth("github<script>");
  assert.equal(readPendingConnectorOauth(), null);
  delete globalThis.window;
});

async function read(relativePath) {
  return readFile(new URL(relativePath, `file://${webRoot}/`), "utf8");
}

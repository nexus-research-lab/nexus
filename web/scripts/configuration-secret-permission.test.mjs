import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

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

async function renderWithI18n(element, locale = "en") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      {
        value: {
          locale,
          setLocale: () => {},
          t: (key, params = {}) => Object.entries(params).reduce(
            (message, [name, value]) => message.replaceAll(
              `{${name}}`,
              String(value),
            ),
            MESSAGES[locale][key] ?? key,
          ),
        },
      },
      element,
    ),
  );
}

test("permission request decodes only valid unique configuration secret slots", async () => {
  const { decodePermissionRequest } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/permission/permission-event-data.ts",
  );
  const permission = decodePermissionRequest({
    data: {
      configuration_secret_slots: [
        { id: "token", path: "channels.telegram.token" },
        { id: "token", path: "duplicate.must.not.win" },
        { id: "", path: "missing.id" },
        { id: "missing-path" },
      ],
      request_id: "permission-secret",
      tool_input: { visible: "ordinary input" },
      tool_name: "ConfigureChannel",
    },
    session_key: "agent:agent-a:conversation-a",
    type: "permission_request",
  });

  assert.deepEqual(permission?.configuration_secret_slots, [{
    id: "token",
    path: "channels.telegram.token",
  }]);
  assert.deepEqual(permission?.tool_input, { visible: "ordinary input" });
});

test("scheduled-task permissions decode their recipient Session interaction context", async () => {
  const { decodePermissionRequest } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/permission/permission-event-data.ts",
  );
  const permission = decodePermissionRequest({
    agent_id: "agent-recipient",
    data: {
      automation_allow_task: true,
      automation_job_id: "task-1",
      automation_policy_revision: 4,
      automation_request_kind: "tool",
      automation_run_id: "run-1",
      automation_task_name: "Daily report",
      request_id: "permission-automation",
      request_source: "automation",
      summary: "Scheduled task Daily report needs WebSearch",
      tool_input: { query: "latest research" },
      tool_name: "WebSearch",
    },
    session_key: "agent:agent-recipient:ws:dm:conversation-a",
    type: "permission_request",
  });

  assert.equal(permission?.source, "automation");
  assert.equal(permission?.agent_id, "agent-recipient");
  assert.deepEqual(permission?.automation, {
    allow_task: true,
    job_id: "task-1",
    kind: "tool",
    policy_revision: 4,
    run_id: "run-1",
    task_name: "Daily report",
  });
});

test("secret drafts are request-scoped and never leak values across request ids", async () => {
  const {
    createConfigurationSecretDraft,
    getConfigurationSecretDraftValues,
    updateConfigurationSecretDraft,
  } = await server.ssrLoadModule(
    "/src/lib/conversation/configuration-secret-permission.ts",
  );
  const first = updateConfigurationSecretDraft(
    createConfigurationSecretDraft("request-a"),
    "request-a",
    "token",
    "secret-a",
  );
  assert.deepEqual(
    getConfigurationSecretDraftValues(first, "request-a"),
    { token: "secret-a" },
  );
  assert.deepEqual(
    getConfigurationSecretDraftValues(first, "request-b"),
    {},
    "switching request_id must synchronously expose blank inputs",
  );

  const second = updateConfigurationSecretDraft(
    first,
    "request-b",
    "password",
    "secret-b",
  );
  assert.deepEqual(second, {
    requestId: "request-b",
    values: { password: "secret-b" },
  });
});

test("secret selection requires every slot and emits only declared ids", async () => {
  const {
    hasCompleteConfigurationSecrets,
    selectConfigurationSecrets,
  } = await server.ssrLoadModule(
    "/src/lib/conversation/configuration-secret-permission.ts",
  );
  const slots = [
    { id: "token", path: "service.token" },
    { id: "password", path: "service.password" },
  ];
  assert.equal(
    hasCompleteConfigurationSecrets(slots, { token: "present" }),
    false,
  );
  assert.equal(
    selectConfigurationSecrets(slots, { token: "present" }),
    undefined,
  );
  assert.equal(
    selectConfigurationSecrets(slots, {
      extra: "must-not-send",
      password: " ",
      token: "present",
    }),
    undefined,
    "whitespace-only values must not pass the human secret boundary",
  );
  assert.deepEqual(
    selectConfigurationSecrets(slots, {
      extra: "must-not-send",
      password: " exact value ",
      token: "present",
    }),
    { password: " exact value ", token: "present" },
    "nonempty values are preserved exactly and undeclared keys are dropped",
  );
});

test("Composer masks requested secrets and blocks allow while fields are empty", async () => {
  const { ComposerPermissionSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-permission-surface.tsx",
  );
  const html = await renderWithI18n(React.createElement(
    ComposerPermissionSurface,
    {
      interactionDisabled: false,
      kind: "permission",
      onResponse: () => true,
      permission: {
        configuration_secret_slots: [
          { id: "token", path: "service.token" },
          { id: "password", path: "service.password" },
        ],
        request_id: "permission-ui",
        tool_input: {},
        tool_name: "ConfigureService",
      },
      total: 1,
    },
  ));

  assert.equal((html.match(/type="password"/g) ?? []).length, 2);
  assert.equal((html.match(/autoComplete="new-password"/g) ?? []).length, 2);
  assert.match(html, /service\.token/);
  assert.match(html, /service\.password/);
  assert.match(
    html,
    /data-composer-permission-decision="allow"[^>]*disabled=""/,
  );
  assert.doesNotMatch(
    html,
    /data-composer-permission-decision="deny"[^>]*disabled=""/,
  );
});

test("scheduled-task Composer offers a task-scoped approval choice", async () => {
  const { ComposerPermissionSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-permission-surface.tsx",
  );
  const { buildComposerPermissionScopeItems } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-permission-scope-items.tsx",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const permission = {
    automation: {
      allow_task: true,
      job_id: "task-1",
      kind: "tool",
      policy_revision: 4,
      run_id: "run-1",
      task_name: "Daily report",
    },
    request_id: "permission-automation",
    source: "automation",
    summary: "Scheduled task needs WebSearch",
    tool_input: { query: "latest research" },
    tool_name: "WebSearch",
  };
  const html = await renderWithI18n(React.createElement(
    ComposerPermissionSurface,
    {
      interactionDisabled: false,
      kind: "permission",
      onResponse: () => true,
      permission,
      total: 1,
    },
  ));

  assert.match(html, /data-composer-permission-action="allow"/);
  assert.match(html, /aria-label="Choose permission scope"/);
  const t = (key, params = {}) => Object.entries(params).reduce(
    (message, [name, value]) => message.replaceAll(`{${name}}`, String(value)),
    MESSAGES.en[key] ?? key,
  );
  const items = buildComposerPermissionScopeItems(
    permission,
    [],
    undefined,
    t,
  );
  assert.equal(items.length, 2);
  assert.equal(items[1].value, "allow-task");
  assert.equal(items[1].description, "Adds this capability only to scheduled task Daily report");
  assert.match(renderToStaticMarkup(items[1].label), /Always allow for this task/);
});

function createPermissionContext(decisionPermission) {
  let error = null;
  let sentMessage = null;
  let permissions = [decisionPermission];
  return {
    context: {
      acknowledgePermissionRequest: () => {},
      activeSessionKeyRef: {
        current: "agent:agent-a:web:dm:conversation-a",
      },
      identity: {
        agent_id: "agent-a",
        chat_type: "dm",
        conversation_id: "conversation-a",
        room_id: null,
      },
      messages: [],
      pendingPermissions: permissions,
      reliability: {
        observeRecovery: () => {
          error = null;
        },
        reportFailure: (failure) => {
          error = failure.code;
        },
      },
      sessionKey: "agent:agent-a:web:dm:conversation-a",
      setMessages: () => {},
      setPendingPermissions: (next) => {
        permissions = typeof next === "function"
          ? next(permissions)
          : next;
      },
      wsSend: (message) => {
        sentMessage = message;
        return { disposition: "sent" };
      },
      wsState: "connected",
    },
    read: () => ({ error, permissions, sentMessage }),
  };
}

test("permission responses send secrets only for complete allow decisions", async () => {
  const { sendSessionPermissionResponse } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/conversation-control-actions.ts",
  );
  const pendingPermission = {
    configuration_secret_slots: [
      { id: "token", path: "service.token" },
      { id: "password", path: "service.password" },
    ],
    request_id: "permission-secret",
    tool_input: {},
    tool_name: "ConfigureService",
  };
  const incomplete = createPermissionContext(pendingPermission);
  assert.equal(sendSessionPermissionResponse({
    configuration_secrets: { token: "only-one" },
    decision: "allow",
    request_id: pendingPermission.request_id,
  }, incomplete.context), false);
  assert.equal(incomplete.read().sentMessage, null);
  assert.equal(incomplete.read().permissions.length, 1);

  const allowed = createPermissionContext(pendingPermission);
  assert.equal(sendSessionPermissionResponse({
    configuration_secrets: {
      extra: "must-not-send",
      password: "password-value",
      token: "token-value",
    },
    decision: "allow",
    request_id: pendingPermission.request_id,
  }, allowed.context), true);
  assert.deepEqual(allowed.read().sentMessage.configuration_secrets, {
    password: "password-value",
    token: "token-value",
  });
  assert.equal(allowed.read().sentMessage.tool_input, undefined);

  const denied = createPermissionContext(pendingPermission);
  assert.equal(sendSessionPermissionResponse({
    configuration_secrets: {
      password: "must-not-send",
      token: "must-not-send",
    },
    decision: "deny",
    request_id: pendingPermission.request_id,
  }, denied.context), true);
  assert.equal(
    "configuration_secrets" in denied.read().sentMessage,
    false,
  );
});

test("scheduled-task permission responses preserve the selected Automation scope", async () => {
  const { sendSessionPermissionResponse } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/conversation-control-actions.ts",
  );
  const pendingPermission = {
    automation: {
      allow_task: true,
      job_id: "task-1",
      kind: "tool",
      policy_revision: 4,
      run_id: "run-1",
      task_name: "Daily report",
    },
    request_id: "permission-automation",
    source: "automation",
    tool_input: { query: "latest research" },
    tool_name: "WebSearch",
  };
  const taskScoped = createPermissionContext(pendingPermission);
  assert.equal(sendSessionPermissionResponse({
    automation_scope: "task",
    decision: "allow",
    request_id: pendingPermission.request_id,
  }, taskScoped.context), true);
  assert.equal(taskScoped.read().sentMessage.automation_scope, "task");
  assert.equal(taskScoped.read().sentMessage.updated_permissions, undefined);

  const once = createPermissionContext(pendingPermission);
  assert.equal(sendSessionPermissionResponse({
    decision: "allow",
    request_id: pendingPermission.request_id,
  }, once.context), true);
  assert.equal(once.read().sentMessage.automation_scope, "once");
});

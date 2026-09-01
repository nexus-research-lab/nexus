import assert from "node:assert/strict";
import path from "node:path";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
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

test("password unknown outcomes persist only an exact receipt pointer", async () => {
  const receipts = await server.ssrLoadModule(
    "/src/features/settings/personal/password-change-receipt.ts",
  );
  const values = new Map();
  const previousWindow = globalThis.window;
  globalThis.window = {
    localStorage: {
      getItem: (key) => values.get(key) ?? null,
      removeItem: (key) => values.delete(key),
      setItem: (key, value) => values.set(key, String(value)),
    },
  };
  try {
    const requestID = receipts.createPasswordChangeRequestID();
    receipts.rememberPendingPasswordChangeRequest("user-1", requestID);
    assert.equal(
      receipts.readPendingPasswordChangeRequest("user-1"),
      requestID,
    );
    assert.equal(values.size, 1);
    assert.doesNotMatch(JSON.stringify([...values]), /current_password|new_password/);
    receipts.forgetPendingPasswordChangeRequest("user-1", "another-request-id");
    assert.equal(
      receipts.readPendingPasswordChangeRequest("user-1"),
      requestID,
      "a stale response must not erase a newer exact request pointer",
    );
    receipts.forgetPendingPasswordChangeRequest("user-1", requestID);
    assert.equal(receipts.readPendingPasswordChangeRequest("user-1"), null);
  } finally {
    if (previousWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = previousWindow;
    }
  }
});

test("subscription mutation locking is independent from visible feedback", async () => {
  const { buildSubscriptionMutationFailure } = await server.ssrLoadModule(
    "/src/features/settings/operations/subscription-admin/subscription-admin-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key, params = {}) => Object.entries(params).reduce(
    (message, [name, value]) => message.replaceAll(`{${name}}`, String(value)),
    zhMessages[key] ?? key,
  );
  const unknown = buildSubscriptionMutationFailure(
    t,
    "account",
    new ApiTransportError("network details", "network", "unknown"),
  );
  assert.equal(unknown.effect, "unknown");
  assert.equal("blocksMutation" in unknown.feedback, false);

  const rejected = buildSubscriptionMutationFailure(
    t,
    "account",
    new ApiRequestError("details", 409, {
      category: "conflict",
      code: "subscription.not_applied",
      effect: "not_applied",
      version: 1,
    }),
  );
  assert.equal(rejected.effect, "not_applied");
});

test("project mutations distinguish not-applied from an unknown outcome", async () => {
  const { buildProjectMutationFeedback } = await server.ssrLoadModule(
    "/src/features/settings/operations/project-admin/project-admin-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];

  const rejected = buildProjectMutationFeedback(
    t,
    "grant",
    new ApiRequestError("权限不允许", 403, {
      version: 1,
      code: "project.permission_denied",
      category: "authorization",
      effect: "not_applied",
    }),
  );
  assert.equal(rejected.title, "成员权限没有更新");
  assert.match(rejected.impact, /没有改变/);
  assert.equal(rejected.recoveryAction, undefined);

  const unknown = buildProjectMutationFeedback(
    t,
    "create",
    new ApiTransportError("连接已中断", "network", "unknown"),
  );
  assert.match(unknown.title, /无法确认/);
  assert.match(unknown.impact, /重复项目/);
  assert.match(unknown.nextStep, /不要再次创建/);
  assert.equal(unknown.recoveryAction, "refresh");

  const committed = buildProjectMutationFeedback(
    t,
    "grant",
    new ApiRequestError("响应写回失败", 500, {
      version: 1,
      code: "project.access_committed",
      category: "internal",
      effect: "committed",
    }),
  );
  assert.match(committed.title, /已更新/);
  assert.match(committed.impact, /已经保存在服务端/);
  assert.doesNotMatch(committed.impact, /可能已经保存|请求中断前/);
  assert.equal(committed.recoveryAction, "refresh");

  const accepted = buildProjectMutationFeedback(
    t,
    "create",
    new ApiRequestError("仍在处理", 202, {
      version: 1,
      code: "project.create_accepted",
      category: "unavailable",
      effect: "accepted",
    }),
  );
  assert.match(accepted.title, /正在处理/);
  assert.match(accepted.impact, /已经接收/);
  assert.match(accepted.nextStep, /不要再次创建/);
});

test("Provider validation keeps the safe field-specific correction", async () => {
  const { buildProviderValidationFeedback } = await server.ssrLoadModule(
    "/src/features/settings/provider-settings/model/provider-feedback-model.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];
  const correction = t("settings.providers.check_json_format");

  const feedback = buildProviderValidationFeedback(
    t("settings.providers.model_options_save_failed_title"),
    correction,
  );

  assert.equal(feedback.impact, correction);
  assert.equal(feedback.tone, "error");
});

test("reconciled settings expose one reapply action without discarding the draft", async () => {
  const [preferencesModule, echoModule, i18nModule, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/settings/general/components/preferences-reliability-notice.tsx",
    ),
    server.ssrLoadModule(
      "/src/features/settings/general/components/echo-settings-reliability-notice.tsx",
    ),
    server.ssrLoadModule("/src/shared/i18n/i18n-context.ts"),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const t = (key, params) => {
    const template = messagesModule.MESSAGES.zh[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (match, name) => (
      params?.[name] === undefined ? match : String(params[name])
    ));
  };
  const provider = (child) => React.createElement(
    i18nModule.I18N_CONTEXT.Provider,
    { value: { locale: "zh", setLocale: () => {}, t } },
    child,
  );
  const noop = () => {};
  const preferencesHtml = renderToStaticMarkup(provider(React.createElement(
    preferencesModule.PreferencesReliabilityNotice,
    {
      feedback: { impact: "本页输入仍已保留。", title: "服务端设置不同", tone: "warning" },
      recovery: {
        canCompare: true,
        canRepairProjection: false,
        checking: false,
        checkLatest: noop,
        reapplyDraft: noop,
        repairProjection: noop,
        repairing: false,
      },
    },
  )));
  const echoHtml = renderToStaticMarkup(provider(React.createElement(
    echoModule.EchoSettingsReliabilityNotice,
    {
      feedback: { impact: "本页选择仍已保留。", title: "服务端设置不同", tone: "warning" },
      recovery: {
        canCheckLatest: false,
        canCompare: true,
        canFinishDisabling: false,
        checking: false,
        checkLatest: noop,
        finishDisabling: noop,
        reapplyChange: noop,
        repairing: false,
      },
    },
  )));

  assert.equal((preferencesHtml.match(/<button/g) ?? []).length, 1);
  assert.match(preferencesHtml, /重新应用本页修改/);
  assert.equal((echoHtml.match(/<button/g) ?? []).length, 1);
  assert.match(echoHtml, /重新应用本次更改/);
  assert.doesNotMatch(`${preferencesHtml}${echoHtml}`, /使用最新/);
});

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

function resource(items) {
  return {
    error: null,
    items,
    loading: false,
  };
}

function session({ channelType, externalIdentity, key, title }) {
  return {
    agent_id: "nexus",
    channel_type: channelType,
    chat_type: "dm",
    created_at: 0,
    last_activity_at: 0,
    message_count: 0,
    options: {},
    room_id: null,
    room_session_id: null,
    session_id: null,
    session_key: key,
    status: "active",
    title,
    external_identity: externalIdentity ?? null,
  };
}

test("scheduled task session options mark external IM channels only", async () => {
  const { buildTaskDialogSessionData } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/resources/task-dialog-resource-model.ts",
  );
  const agentSessions = resource([
    session({
      channelType: "weixin-personal",
      externalIdentity: {
        account_hint: "A1B2C3",
        can_delete: false,
        channel_type: "weixin-personal",
        current_pairing: true,
        pairing_status: "active",
      },
      key: "agent:nexus:weixin-personal:dm:wx-user",
      title: "定时任务回传测试",
    }),
    session({
      channelType: "websocket",
      key: "agent:nexus:ws:dm:web-chat",
      title: "普通对话",
    }),
    session({
      channelType: "",
      externalIdentity: {
        can_delete: false,
        channel_type: "telegram",
        current_pairing: true,
        pairing_status: "active",
      },
      key: "agent:nexus:tg:dm:telegram-chat",
      title: "Telegram 对话",
    }),
    session({
      channelType: "wechat",
      externalIdentity: {
        can_delete: false,
        channel_type: "wechat",
        current_pairing: true,
        pairing_status: "active",
      },
      key: "agent:nexus:wx:dm:wecom-chat",
      title: "企业微信对话",
    }),
    session({
      channelType: "weixin-personal",
      externalIdentity: {
        account_hint: "OLD999",
        can_delete: true,
        channel_type: "weixin-personal",
        current_pairing: false,
        pairing_status: "unpaired",
      },
      key: "agent:nexus:weixin-personal:dm:old-account",
      title: "已经解绑",
    }),
  ]);
  const result = buildTaskDialogSessionData(
    "agent",
    { agentSessions, roomContexts: resource([]) },
    new Map([["nexus", "nexus"]]),
    "未命名会话",
  );

  assert.deepEqual(
    result.options.map(({ badge, label }) => ({ badge: badge ?? null, label })),
    [
      { badge: "IM · 微信 · 账号 A1B2C3 · 当前", label: "定时任务回传测试 · nexus" },
      { badge: null, label: "普通对话 · nexus" },
      { badge: "IM · Telegram · 当前", label: "Telegram 对话 · nexus" },
      { badge: "IM · 企业微信 · 当前", label: "企业微信对话 · nexus" },
    ],
  );
});

test("scheduled task selectors hide every unpaired external IM session", async () => {
  const { buildTaskDialogSessionData } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/resources/task-dialog-resource-model.ts",
  );
  const channels = [
    "weixin-personal",
    "wechat",
    "feishu",
    "telegram",
    "discord",
    "dingtalk",
  ];
  const result = buildTaskDialogSessionData(
    "agent",
    {
      agentSessions: resource(channels.map((channelType) => session({
        channelType,
        externalIdentity: {
          can_delete: true,
          channel_type: channelType,
          current_pairing: false,
          pairing_status: "unpaired",
        },
        key: `agent:nexus:${channelType}:dm:historical-target`,
        title: `${channelType} 历史会话`,
      }))),
      roomContexts: resource([]),
    },
    new Map([["nexus", "nexus"]]),
    "未命名会话",
  );

  assert.deepEqual(result.options, []);
});

test("select presentation carries the selected session badge", async () => {
  const {
    buildSelectMenuPresentation,
    estimateSelectMenuHeight,
  } = await server.ssrLoadModule(
    "/src/shared/ui/menu/select-menu-model.ts",
  );
  const presentation = buildSelectMenuPresentation({
    allowLabelWrap: false,
    options: [
      { label: "普通对话 · nexus", value: "web" },
      { badge: "IM · 微信", label: "定时任务回传测试 · nexus", value: "wx" },
    ],
    placeholder: "请选择会话",
    size: "md",
    value: "wx",
  });

  assert.equal(presentation.activeLabel, "定时任务回传测试 · nexus");
  assert.equal(presentation.activeBadge, "IM · 微信");
  assert.equal(estimateSelectMenuHeight(2, 32), 76);
});

test("editing isolated IM task keeps its delivery session separate from execution", async () => {
  const { buildTaskDialogInitialState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-initializer.ts",
  );
  const imSessionKey = "agent:nexus:weixin-personal:dm:wx-user";
  const state = buildTaskDialogInitialState({
    agent_id: "nexus",
    delivery: { mode: "last", session_key: imSessionKey },
    enabled: true,
    execution_kind: "agent",
    expires_at: null,
    failure_streak: 0,
    instruction: "生成日报",
    job_id: "task-1",
    last_run_at: null,
    name: "日报",
    next_run_at: null,
    overlap_policy: "skip",
    permission_mode: "plan",
    running: false,
    running_started_at: null,
    schedule: {
      interval_seconds: 3600,
      kind: "every",
      timezone: "Asia/Shanghai",
    },
    session_target: { kind: "isolated" },
    source: {
      context_id: "nexus",
      context_type: "agent",
      kind: "agent",
      session_key: imSessionKey,
    },
  });

  assert.equal(state.form.executionMode, "temporary");
  assert.equal(state.form.replyMode, "selected");
  assert.equal(state.form.selectedReplySessionKey, imSessionKey);
  assert.equal(state.form.permissionMode, "plan");
});

test("editing a task that requires rebind clears the deleted session", async () => {
  const { buildTaskDialogInitialState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-initializer.ts",
  );
  const imSessionKey = "agent:nexus:weixin-personal:dm:acct:old-account:old-contact";
  const task = {
    agent_id: "nexus",
    delivery: { channel: "websocket", mode: "explicit", to: imSessionKey },
    enabled: true,
    execution_kind: "agent",
    expires_at: null,
    failure_streak: 0,
    instruction: "生成旧账号日报",
    job_id: "legacy-task",
    last_run_at: null,
    name: "旧账号日报",
    next_run_at: null,
    overlap_policy: "skip",
    permission_mode: "default",
    running: false,
    running_started_at: null,
    session_binding_issues: ["delivery"],
    session_binding_state: "rebind_required",
    schedule: {
      interval_seconds: 3600,
      kind: "every",
      timezone: "Asia/Shanghai",
    },
    session_target: { kind: "isolated" },
    source: {
      context_id: "nexus",
      context_type: "agent",
      kind: "user_page",
    },
  };
  const state = buildTaskDialogInitialState(task);

  assert.equal(state.form.replyMode, "selected");
  assert.equal(state.form.selectedReplySessionKey, "");
});

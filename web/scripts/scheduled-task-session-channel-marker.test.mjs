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

test("scheduled tasks reconcile once after socket connection without fallback polling", async () => {
  const source = await readFile(
    path.join(webRoot, "src/features/capability/scheduled/use-scheduled-task-realtime-refresh.ts"),
    "utf8",
  );

  assert.match(source, /previousWsStateRef/);
  assert.doesNotMatch(source, /setInterval|FALLBACK_POLL/);
});

function resource(items) {
  return {
    error: null,
    items,
    loading: false,
  };
}

function session({ agentId = "nexus", channelType, chatType = "dm", conversationId = null, externalIdentity, key, roomId = null, title }) {
  return {
    agent_id: agentId,
    channel_type: channelType,
    chat_type: chatType,
    created_at: 0,
    last_activity_at: 0,
    message_count: 0,
    options: {},
    room_id: roomId,
    room_session_id: null,
    session_id: null,
    session_key: key,
    status: "active",
    title,
    conversation_id: conversationId,
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
      channelType: "websocket",
      chatType: "group",
      conversationId: "room-conversation",
      key: "agent:nexus:ws:group:room-conversation",
      roomId: "room-1",
      title: "不应混入的 Room 成员会话",
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

test("scheduled task Room targets exclude every DM-backed room", async () => {
  const {
    buildExecutionRoomOptions,
    buildRoomOptions,
  } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/resources/task-dialog-resource-model.ts",
  );
  const activeMember = {
    member_type: "agent",
    participation_paused: false,
  };
  const rooms = [
    {
      members: [activeMember],
      room: { id: "dm-room", name: "Analyst", room_type: "dm" },
    },
    {
      members: [activeMember],
      room: { id: "group-room", name: "研究小组", room_type: "room" },
    },
    {
      members: [{ ...activeMember, participation_paused: true }],
      room: { id: "paused-room", name: "暂停小组", room_type: "room" },
    },
  ];

  assert.deepEqual(buildRoomOptions(rooms), [
    { label: "研究小组", value: "group-room" },
    { label: "暂停小组", value: "paused-room" },
  ]);
  assert.deepEqual(buildExecutionRoomOptions(rooms), [
    { label: "研究小组", value: "group-room" },
  ]);
});

test("Agent delivery includes Room-backed DM but excludes Room member sessions", async () => {
  const { buildTaskDialogDeliverySessionData } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/resources/task-dialog-resource-model.ts",
  );
  const recipientIM = session({
    agentId: "agent-b",
    channelType: "weixin-personal",
    externalIdentity: {
      can_delete: false,
      channel_type: "weixin-personal",
      current_pairing: true,
      pairing_status: "active",
    },
    key: "agent:agent-b:weixin-personal:dm:wx-user-b",
    title: "B 的微信",
  });
  const unavailableRecipientIM = session({
    agentId: "agent-b",
    channelType: "telegram",
    externalIdentity: {
      can_delete: true,
      channel_type: "telegram",
      current_pairing: false,
      pairing_status: "unpaired",
    },
    key: "agent:agent-b:tg:dm:old-b",
    title: "B 的旧 Telegram",
  });
  const executorIM = session({
    agentId: "agent-a",
    channelType: "weixin-personal",
    externalIdentity: {
      can_delete: false,
      channel_type: "weixin-personal",
      current_pairing: true,
      pairing_status: "active",
    },
    key: "agent:agent-a:weixin-personal:dm:wx-user-a",
    title: "A 的微信",
  });
  const recipientNexusDM = session({
    agentId: "agent-b",
    channelType: "websocket",
    conversationId: "dm-conversation-b",
    key: "agent:agent-b:ws:dm:dm-conversation-b",
    roomId: "dm-room-b",
    title: "B 的 Nexus DM",
  });
  const recipientRoomMember = session({
    agentId: "agent-b",
    channelType: "websocket",
    chatType: "group",
    conversationId: "room-conversation-b",
    key: "agent:agent-b:ws:group:room-conversation-b",
    roomId: "room-b",
    title: "B 的 Room 成员会话",
  });
  const legacyInbox = session({
    agentId: "agent-b",
    channelType: "internal",
    key: "agent:agent-b:internal:dm:automation-inbox",
    title: "定时任务收件箱",
  });
  legacyInbox.options = { created_by: "automation_delivery" };
  const result = buildTaskDialogDeliverySessionData(
    {
      deliveryTargetType: "agent",
      executionKind: "agent",
      replyMode: "selected",
      selectedDeliveryAgentId: "agent-b",
    },
    resource([
      executorIM,
      recipientNexusDM,
      recipientRoomMember,
      recipientIM,
      unavailableRecipientIM,
      legacyInbox,
    ]),
    new Map([["agent-a", "A"], ["agent-b", "B"]]),
    new Map(),
    "未命名会话",
  );

  assert.deepEqual(result.options.map(({ sessionKey }) => sessionKey), [
    recipientNexusDM.session_key,
    recipientIM.session_key,
  ]);
});

test("Room delivery exposes a shared conversation, then its exact member agents", async () => {
  const {
    buildDeliveryRoomAgentData,
    buildTaskDialogDeliverySessionData,
    resolveTaskDialogRoomId,
  } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/resources/task-dialog-resource-model.ts",
  );
  const directSession = session({
    agentId: "agent-a",
    channelType: "websocket",
    conversationId: "dm-conversation",
    key: "agent:agent-a:ws:dm:dm-conversation",
    roomId: "room-1",
    title: "不应混入的 DM",
  });
  const roomSessions = [
    directSession,
    session({
      agentId: "agent-a",
      channelType: "websocket",
      chatType: "group",
      conversationId: "conversation-1",
      key: "agent:agent-a:ws:group:conversation-1",
      roomId: "room-1",
      title: "项目周报",
    }),
    session({
      agentId: "agent-b",
      channelType: "websocket",
      chatType: "group",
      conversationId: "conversation-1",
      key: "agent:agent-b:ws:group:conversation-1",
      roomId: "room-1",
      title: "项目周报",
    }),
  ];
  const result = buildTaskDialogDeliverySessionData(
    {
      deliveryTargetType: "room",
      executionKind: "agent",
      replyMode: "selected",
      selectedDeliveryRoomId: "room-1",
    },
    resource(roomSessions),
    new Map(),
    new Map([["room-1", "研发 Room"]]),
    "未命名会话",
  );

  assert.deepEqual(result.options.map(({ sessionKey, value }) => ({ sessionKey, value })), [
    { sessionKey: "room:group:conversation-1", value: "room:group:conversation-1" },
  ]);
  assert.equal(resolveTaskDialogRoomId(roomSessions, directSession.session_key), "");
  assert.equal(resolveTaskDialogRoomId(roomSessions, "room:group:conversation-1"), "room-1");
  assert.deepEqual(buildDeliveryRoomAgentData(
    roomSessions,
    [{ room: { host_agent_id: "agent-a", id: "room-1", room_type: "room" } }],
    "room-1",
    "room:group:conversation-1",
    new Map([["agent-a", "A"], ["agent-b", "B"]]),
  ), {
    defaultAgentId: "agent-a",
    options: [
      { label: "A", value: "agent-a" },
      { label: "B", value: "agent-b" },
    ],
  });
});

test("Room execution exposes conversation before member and defaults to host", async () => {
  const {
    buildExecutionRoomAgentData,
    buildTaskDialogSessionData,
  } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/resources/task-dialog-resource-model.ts",
  );
  const context = {
    conversation: { id: "conversation-1", title: "方案讨论" },
    member_agents: [
      { agent_id: "agent-a", name: "A", room_participation_paused: false },
      { agent_id: "agent-b", name: "B", room_participation_paused: false },
      { agent_id: "agent-c", name: "C", room_participation_paused: true },
    ],
    room: { host_agent_id: "agent-a", id: "room-1", name: "研发 Room", room_type: "room" },
    sessions: [
      { agent_id: "agent-a" },
      { agent_id: "agent-b" },
      { agent_id: "agent-c" },
    ],
  };
  const sessionData = buildTaskDialogSessionData(
    "room",
    { agentSessions: resource([]), roomContexts: resource([context]) },
    new Map([["agent-a", "A"], ["agent-b", "B"]]),
    "未命名会话",
  );
  assert.deepEqual(sessionData.options.map(({ label, value }) => ({ label, value })), [{
    label: "研发 Room · 方案讨论",
    value: "room:group:conversation-1",
  }]);
  assert.deepEqual(buildExecutionRoomAgentData(
    [context],
    "room:group:conversation-1",
  ), {
    defaultAgentId: "agent-a",
    options: [
      { label: "A", value: "agent-a" },
      { label: "B", value: "agent-b" },
    ],
  });
});

test("scheduled task payload keeps executor and recipient identities independent", async () => {
  const { buildScheduledTaskPayload } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-submit.ts",
  );
  const recipientSession = "agent:agent-b:weixin-personal:dm:wx-user-b";
  const payload = buildScheduledTaskPayload({
    defaultDeliveryRoomAgentId: "",
    defaultExecutionRoomAgentId: "",
    form: {
      dedicatedSessionKey: "",
      deliveryTargetType: "agent",
      enabled: true,
      executionKind: "agent",
      executionMode: "temporary",
      expiresAt: "",
      instruction: "生成日报",
      permissionMode: "copy",
      replyMode: "selected",
      selectedAgentId: "agent-a",
      selectedDeliveryAgentId: "agent-b",
      selectedDeliveryPresenterAgentId: "",
      selectedDeliveryRoomId: "",
      selectedReplySessionKey: recipientSession,
      selectedRoomId: "",
      selectedSessionKey: "",
      targetType: "agent",
      taskName: "跨智能体日报",
    },
    schedule: {
      dailyTime: "09:00",
      everyUnit: "hours",
      everyValue: "1",
      kind: "every",
      runAt: "",
      selectedWeekdays: [],
      timezone: "Asia/Shanghai",
    },
    selectedReplySession: {
      badge: "IM · 微信 · 当前",
      label: "B 的微信 · B",
      sessionKey: recipientSession,
      value: recipientSession,
    },
    selectedSession: null,
  }, (key) => key);

  assert.equal(payload.agent_id, "agent-a");
  assert.deepEqual(payload.session_target, {
    kind: "isolated",
    wake_mode: "next-heartbeat",
  });
  assert.deepEqual(payload.delivery, {
    mode: "last",
    session_key: recipientSession,
  });
  assert.equal(payload.execution_kind, "agent");
  assert.equal(payload.permission_mode, undefined);
  assert.equal(payload.source, undefined);
});

test("Room execution payload uses explicit member after the shared session", async () => {
  const { buildScheduledTaskPayload } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-submit.ts",
  );
  const roomSession = "room:group:conversation-1";
  const payload = buildScheduledTaskPayload({
    defaultDeliveryRoomAgentId: "",
    defaultExecutionRoomAgentId: "agent-a",
    form: {
      dedicatedSessionKey: "",
      deliveryTargetType: "agent",
      enabled: false,
      executionKind: "agent",
      executionMode: "existing",
      expiresAt: "",
      instruction: "结合 Room 上下文总结",
      permissionMode: "copy",
      replyMode: "none",
      selectedAgentId: "agent-b",
      selectedDeliveryAgentId: "agent-a",
      selectedDeliveryPresenterAgentId: "",
      selectedDeliveryRoomId: "",
      selectedReplySessionKey: "",
      selectedRoomId: "room-1",
      selectedSessionKey: roomSession,
      targetType: "room",
      taskName: "Room 总结",
    },
    schedule: {
      dailyTime: "09:00",
      everyUnit: "hours",
      everyValue: "1",
      kind: "every",
      runAt: "",
      selectedWeekdays: [],
      timezone: "Asia/Shanghai",
    },
    selectedReplySession: null,
    selectedSession: {
      label: "研发 Room · 方案讨论",
      sessionKey: "room:group:conversation-1",
      value: roomSession,
    },
  }, (key) => key);

  assert.equal(payload.agent_id, "agent-b");
  assert.deepEqual(payload.session_target, {
    bound_session_key: "room:group:conversation-1",
    kind: "bound",
    wake_mode: "next-heartbeat",
  });
  assert.deepEqual(payload.delivery, { mode: "none" });
});

test("Room result delivery persists an independent replying agent", async () => {
  const { buildScheduledTaskPayload } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-submit.ts",
  );
  const roomSession = "room:group:conversation-2";
  const payload = buildScheduledTaskPayload({
    defaultDeliveryRoomAgentId: "agent-host",
    defaultExecutionRoomAgentId: "",
    form: {
      dedicatedSessionKey: "",
      deliveryTargetType: "room",
      enabled: true,
      executionKind: "agent",
      executionMode: "temporary",
      expiresAt: "",
      instruction: "发送 Room 周报",
      permissionMode: "copy",
      replyMode: "selected",
      selectedAgentId: "agent-executor",
      selectedDeliveryAgentId: "",
      selectedDeliveryPresenterAgentId: "agent-presenter",
      selectedDeliveryRoomId: "room-2",
      selectedReplySessionKey: roomSession,
      selectedRoomId: "",
      selectedSessionKey: "",
      targetType: "agent",
      taskName: "Room 周报",
    },
    schedule: {
      dailyTime: "09:00",
      everyUnit: "hours",
      everyValue: "1",
      kind: "every",
      runAt: "",
      selectedWeekdays: [],
      timezone: "Asia/Shanghai",
    },
    selectedReplySession: {
      label: "研发 Room · 周报",
      sessionKey: roomSession,
      value: roomSession,
    },
    selectedSession: null,
  }, (key) => key);

  assert.equal(payload.agent_id, "agent-executor");
  assert.deepEqual(payload.delivery, {
    agent_id: "agent-presenter",
    channel: "websocket",
    mode: "explicit",
    session_key: roomSession,
    to: roomSession,
  });
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
    configuration_version: 1,
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

test("editing keeps delivery when execution and recipient are the same real session", async () => {
  const { buildTaskDialogInitialState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-initializer.ts",
  );
  const sessionKey = "agent:nexus:ws:dm:same-session";
  const state = buildTaskDialogInitialState({
    agent_id: "nexus",
    configuration_version: 3,
    delivery: {
      channel: "websocket",
      mode: "explicit",
      session_key: sessionKey,
      to: sessionKey,
    },
    enabled: true,
    execution_kind: "agent",
    expires_at: null,
    failure_streak: 0,
    instruction: "总结当前会话",
    job_id: "same-session-task",
    last_run_at: null,
    name: "当前会话总结",
    next_run_at: null,
    overlap_policy: "skip",
    permission_mode: "default",
    running: false,
    running_started_at: null,
    schedule: { interval_seconds: 3600, kind: "every", timezone: "Asia/Shanghai" },
    session_target: { bound_session_key: sessionKey, kind: "bound" },
    source: { kind: "user_page" },
  });

  assert.equal(state.form.replyMode, "selected");
  assert.equal(state.form.selectedSessionKey, sessionKey);
  assert.equal(state.form.selectedReplySessionKey, sessionKey);
});

test("editing uses current execution and delivery identities, not immutable source provenance", async () => {
  const { buildTaskDialogInitialState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-initializer.ts",
  );
  const recipientSession = "agent:agent-b:weixin-personal:dm:wx-user-b";
  const state = buildTaskDialogInitialState({
    agent_id: "agent-a",
    configuration_version: 4,
    delivery: {
      mode: "last",
      session_key: recipientSession,
    },
    enabled: true,
    execution_kind: "agent",
    expires_at: null,
    failure_streak: 0,
    instruction: "生成日报",
    job_id: "task-cross-agent",
    last_run_at: null,
    name: "跨智能体日报",
    next_run_at: null,
    overlap_policy: "skip",
    permission_mode: "plan",
    running: false,
    running_started_at: null,
    schedule: { interval_seconds: 3600, kind: "every", timezone: "Asia/Shanghai" },
    session_target: { kind: "isolated" },
    source: {
      context_id: "old-creator",
      context_type: "agent",
      creator_agent_id: "old-creator",
      kind: "agent",
      session_key: "agent:old-creator:ws:dm:old-session",
    },
  });

  assert.equal(state.form.selectedAgentId, "agent-a");
  assert.equal(state.form.selectedDeliveryAgentId, "agent-b");
  assert.equal(state.form.selectedReplySessionKey, recipientSession);
});

test("editing a task that requires rebind clears the deleted session", async () => {
  const { buildTaskDialogInitialState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-initializer.ts",
  );
  const imSessionKey = "agent:nexus:weixin-personal:dm:acct:old-account:old-contact";
  const task = {
    agent_id: "nexus",
    configuration_version: 2,
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

test("editing a legacy scheduled-task inbox requires a real delivery session", async () => {
  const { buildTaskDialogInitialState } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/dialog/form/task-form-initializer.ts",
  );
  const legacyInbox = "agent:nexus:internal:dm:automation-inbox";
  const state = buildTaskDialogInitialState({
    agent_id: "nexus",
    configuration_version: 7,
    delivery: {
      channel: "internal",
      mode: "explicit",
      session_key: legacyInbox,
      to: legacyInbox,
    },
    enabled: true,
    execution_kind: "agent",
    expires_at: null,
    failure_streak: 0,
    instruction: "生成日报",
    job_id: "legacy-inbox-task",
    last_run_at: null,
    name: "旧版收件箱任务",
    next_run_at: null,
    overlap_policy: "skip",
    permission_mode: "default",
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
      kind: "user_page",
    },
  });

  assert.equal(state.form.replyMode, "selected");
  assert.equal(state.form.selectedDeliveryAgentId, "nexus");
  assert.equal(state.form.selectedReplySessionKey, "");
});

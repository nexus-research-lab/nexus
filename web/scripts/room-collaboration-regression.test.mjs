import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";

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

test("Goal status shows one exact token total without budget progress", async () => {
  const {
    buildGoalStatusStripModel,
    goalActualTokens,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-model.ts",
  );
  const { GoalStatusStrip } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-status-strip.tsx",
  );
  const goal = {
    id: "goal-1",
    session_key: "agent:nexus:ws:dm:chat",
    objective: "Ship exact usage",
    status: "active",
    token_budget: 200_000,
    usage: {
      input_tokens: 3_420,
      output_tokens: 206,
      cache_read_input_tokens: 59_136,
      total_tokens: 3_626,
      budget_tokens: 3_626,
      actual_tokens: 62_762,
    },
    continuation_count: 0,
    empty_progress_count: 0,
    version: 1,
    created_at: "2026-07-24T00:00:00Z",
    updated_at: "2026-07-24T00:00:00Z",
  };

  assert.equal(goalActualTokens(goal), 62_762);
  const model = buildGoalStatusStripModel({
    canResume: false,
    continuationHold: null,
    error: null,
    goal,
    isGenerating: true,
  });
  assert.equal(model.usageLabel, "62,762 tokens");
  assert.equal("usagePercent" in model, false);
  assert.equal("usageTitle" in model, false);
  assert.equal("budgetLabel" in model, false);
  assert.equal(buildGoalStatusStripModel({
    canResume: false,
    continuationHold: null,
    error: null,
    goal: { ...goal, usage: { actual_tokens: 0, budget_tokens: 0 } },
    isGenerating: false,
  }).usageLabel, null);
  assert.equal(buildGoalStatusStripModel({
    canResume: false,
    continuationHold: null,
    error: null,
    goal: { ...goal, status: "complete", usage_finalized: false },
    isGenerating: false,
  }).usageLabel, null);
  assert.equal(buildGoalStatusStripModel({
    canResume: false,
    continuationHold: null,
    error: null,
    goal: { ...goal, status: "complete", usage_finalized: true },
    isGenerating: false,
  }).usageLabel, "62,762 tokens");

  const html = renderToStaticMarkup(React.createElement(GoalStatusStrip, {
    canResume: false,
    compact: false,
    disabled: false,
    error: null,
    goal,
    isGenerating: true,
    isLoading: false,
    scopeLabel: "Goal",
    onClearRequest: () => {},
    onEdit: () => {},
    onPause: () => {},
    onRefresh: () => {},
    onResume: () => {},
  }));
  assert.match(html, />62,762 tokens</);
  assert.doesNotMatch(html, /预算|200,000|3,626|role="meter"/);
});

test("Goal status marks legacy reconstructed actual usage as estimated", async () => {
  const {
    buildGoalStatusStripModel,
    goalActualTokens,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-model.ts",
  );
  const goal = {
    id: "goal-legacy",
    session_key: "agent:nexus:ws:dm:legacy",
    objective: "Read legacy usage",
    status: "paused",
    usage: {
      input_tokens: 10,
      output_tokens: 20,
      cache_creation_input_tokens: 80,
      cache_read_input_tokens: 90,
      reasoning_tokens: 40,
      total_tokens: 30,
    },
    continuation_count: 0,
    empty_progress_count: 0,
    version: 1,
    created_at: "2026-07-24T00:00:00Z",
    updated_at: "2026-07-24T00:00:00Z",
  };

  assert.equal(goalActualTokens(goal), 220);
  const model = buildGoalStatusStripModel({
    canResume: true,
    continuationHold: null,
    error: null,
    goal,
    isGenerating: false,
  });
  assert.equal(model.usageLabel, "≈220 tokens");
});

test("会话标签只按稳定宽度约束进入溢出态", async () => {
  const {
    calculateConversationTabWidths,
    CONVERSATION_TABS_VIEWPORT_INSET,
    hasConversationTabsOverflow,
  } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model.ts",
  );

  assert.equal(
    hasConversationTabsOverflow({
      conversationCount: 2,
      hasCreateButton: true,
      hasLeadingControl: true,
      trackWidth: 400,
    }),
    false,
    "标签仍可按最小可读宽度完整排布时不应进入溢出态",
  );
  assert.equal(
    hasConversationTabsOverflow({
      conversationCount: 4,
      hasCreateButton: true,
      hasLeadingControl: true,
      trackWidth: 400,
    }),
    true,
    "只有稳定宽度约束确认放不下全部标签时才进入溢出态",
  );
  assert.equal(
    hasConversationTabsOverflow({
      conversationCount: 4,
      hasCreateButton: true,
      hasLeadingControl: true,
      trackWidth: 700,
    }),
    false,
    "轨道扩宽后应直接退出溢出态而不依赖动画中的 DOM 尺寸",
  );
  assert.equal(CONVERSATION_TABS_VIEWPORT_INSET, 4);
  assert.equal(
    calculateConversationTabWidths({
      activeConversationId: "single",
      hasCreateButton: true,
      hasLeadingControl: true,
      hasTabsOverflow: false,
      orderedConversations: [{ conversation_id: "single" }],
      trackWidth: 400,
    }).get("single"),
    328,
    "单个标签应扣除右端固定创建入口和中央留白，不再为独立动作胶囊预留间距",
  );
});

test("会话标签暴露稳定的活动与非活动状态类", async () => {
  const { resolveWorkspaceConversationTabPresentation } =
    await server.ssrLoadModule(
      "/src/shared/ui/workspace/controls/conversation-tabs/workspace-conversation-tab-model.ts",
    );
  const active = resolveWorkspaceConversationTabPresentation({
    canClose: true,
    externalSessionLabel: null,
    isActive: true,
    title: "active",
  });
  const inactive = resolveWorkspaceConversationTabPresentation({
    canClose: true,
    externalSessionLabel: null,
    isActive: false,
    title: "inactive",
  });

  assert.match(
    active.rootClassName,
    /\bworkspace-surface-header-conversation-tab\b/,
  );
  assert.match(active.rootClassName, /\bworkspace-surface-header-active-tab\b/);
  assert.doesNotMatch(
    active.rootClassName,
    /\bworkspace-surface-header-inactive-tab\b/,
  );
  assert.match(
    inactive.rootClassName,
    /\bworkspace-surface-header-conversation-tab\b/,
  );
  assert.match(
    inactive.rootClassName,
    /\bworkspace-surface-header-inactive-tab\b/,
  );
  assert.doesNotMatch(
    inactive.rootClassName,
    /\bworkspace-surface-header-active-tab\b/,
  );
});

test("会话标签显式映射滚轮与触控板并在边界放行", async () => {
  const { scrollConversationTabsByWheel } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/controls/conversation-tabs/use-conversation-tabs-scroll.ts",
  );
  const viewport = {
    clientWidth: 200,
    scrollLeft: 100,
    scrollWidth: 600,
  };

  assert.equal(
    scrollConversationTabsByWheel(
      viewport,
      { deltaMode: 0, deltaX: 0, deltaY: 40 },
    ),
    true,
  );
  assert.equal(viewport.scrollLeft, 140, "纵向鼠标滚轮应映射到横向标签轨道");

  assert.equal(
    scrollConversationTabsByWheel(
      viewport,
      { deltaMode: 0, deltaX: -30, deltaY: 5 },
    ),
    true,
  );
  assert.equal(viewport.scrollLeft, 110, "触控板主横轴应保持原始方向");

  viewport.scrollLeft = 400;
  assert.equal(
    scrollConversationTabsByWheel(
      viewport,
      { deltaMode: 0, deltaX: 0, deltaY: 40 },
    ),
    false,
    "到达右边界后应把滚动交还外层页面",
  );
  assert.equal(viewport.scrollLeft, 400);
});

test("工作区源码文件复用 Markdown 代码语义高亮语言", async () => {
  const {
    getWorkspaceFileCodeLanguage,
    getWorkspaceFilePreviewKind,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/editor/workspace-file-preview-kind.ts",
  );

  assert.equal(getWorkspaceFilePreviewKind("scripts/check.py"), "text");
  assert.equal(getWorkspaceFileCodeLanguage("scripts/check.py"), "python");
  assert.equal(getWorkspaceFileCodeLanguage("Dockerfile"), "docker");
  assert.equal(getWorkspaceFileCodeLanguage("Dockerfile.release"), "docker");
  assert.equal(getWorkspaceFileCodeLanguage(".env.local"), "bash");
  assert.equal(getWorkspaceFileCodeLanguage("notes.txt"), null);
});

test("创建 Agent 时行为模板进入独立 API 字段", async () => {
  const { buildCreateAgentMutationParams } = await server.ssrLoadModule(
    "/src/features/agents/options/agent-options-mutation.ts",
  );
  const params = buildCreateAgentMutationParams(
    "Reviewer",
    { model: "model-a", provider: "provider-a" },
    {
      avatar: "1",
      description: "",
      profile_template: "## Role\\n\\n- Review code",
      vibe_tags: ["严谨"],
    },
  );

  assert.equal(params.profile_template, "## Role\\n\\n- Review code");
  assert.equal(params.description, "");
});

test("会话标签按创建时间稳定排序并独立恢复活动项", async () => {
  const {
    getConversationIdsByCreationTime,
    getInitialOpenConversationIds,
    reconcileOpenConversationIds,
  } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model.ts",
  );
  const conversations = [
    {
      conversation_id: "third",
      created_at: 300,
      last_activity_at: 900,
    },
    {
      conversation_id: "first",
      created_at: 100,
      last_activity_at: 800,
    },
    {
      conversation_id: "second",
      created_at: 200,
      last_activity_at: 1000,
    },
  ];
  const orderedIds = getConversationIdsByCreationTime(conversations);

  assert.deepEqual(
    orderedIds,
    ["first", "second", "third"],
    "消息活动时间不得改变标签创建顺序",
  );
  assert.deepEqual(
    getInitialOpenConversationIds("third", orderedIds, orderedIds.length),
    orderedIds,
    "恢复最后活动标签时不得把该标签移动到首位",
  );
  assert.deepEqual(
    reconcileOpenConversationIds({
      conversationId: "second",
      currentIds: ["first", "third"],
      fillAvailable: false,
      maxOpenCount: orderedIds.length,
      orderedIds,
      pendingClosedId: null,
    }),
    orderedIds,
    "重新打开标签必须回到其创建时间位置",
  );
});

test("Room 无显式会话路由时优先恢复用户最后活动项", async () => {
  const { resolveSelectedConversationId } = await server.ssrLoadModule(
    "/src/pages/room/controller/model/room-conversation-model.ts",
  );
  const { buildRoomPageModel } = await server.ssrLoadModule(
    "/src/pages/room/controller/model/page/room-page-model.ts",
  );
  const conversations = [
    { conversation_id: "latest", last_activity_at: 300 },
    { conversation_id: "remembered", last_activity_at: 200 },
  ];

  assert.equal(
    resolveSelectedConversationId(null, conversations, "remembered"),
    "remembered",
    "切回 Room 时应恢复用户最后激活的标签",
  );
  assert.equal(
    resolveSelectedConversationId("latest", conversations, "remembered"),
    "latest",
    "显式 Conversation URL 仍然优先于本地恢复偏好",
  );
  assert.equal(
    resolveSelectedConversationId(null, conversations, "removed"),
    "latest",
    "已删除的恢复目标必须回退到当前有效会话",
  );

  const externalConversation = {
    conversation_id: "external:feishu",
    room_id: "room-a",
    session_key: "feishu:session",
  };
  const model = buildRoomPageModel({
    base: {
      activeRoomSession: null,
      availableRoomAgents: [],
      baseRoomConversations: conversations,
      currentAgent: null,
      currentRoom: null,
      currentRoomContext: null,
      roomMemberAgents: [],
      selectedBaseConversationId: "latest",
      workspaceAgentIds: [],
    },
    externalAgentSessions: [],
    externalRoomConversations: [externalConversation],
    isSelectionReady: true,
    preferredConversationId: externalConversation.conversation_id,
    routeRoomId: "room-a",
    routeSessionKey: null,
  });
  assert.equal(
    model.conversation.selectedId,
    externalConversation.conversation_id,
    "外部 Session 标签加载完成后也应恢复为最后活动项",
  );
});

test("聊天侧栏只按 Room 活动态显示 DM 和群组", async () => {
  const {
    getActiveRoomIds,
    pruneRoomActivity,
    replaceRoomActivitySnapshot,
    updateRoomActivity,
  } = await server.ssrLoadModule("/src/features/home/room-activity-resource.ts");

  pruneRoomActivity(new Set());
  updateRoomActivity("dm-room", "dm-round", "running");
  updateRoomActivity("group-room", "group-round", "running");
  updateRoomActivity("group-room", "group-round", "running", "agent_round", "slot-a");
  updateRoomActivity("group-room", "group-round", "running", "agent_round", "slot-b");
  updateRoomActivity("group-room", "group-round", "finished", "agent_round", "slot-a");
  assert.deepEqual(
    [...getActiveRoomIds()].sort(),
    ["dm-room", "group-room"],
    "DM 和群组必须共享同一 Room 活动态集合",
  );

  updateRoomActivity("group-room", "group-round", "finished");
  replaceRoomActivitySnapshot("dm-room", "dm-round", false);
  assert.deepEqual([...getActiveRoomIds()], [], "终态应清除 Room 活动态");
});

test("聊天行不读取持久化 Agent active 状态", async () => {
  const { buildConversationItems } = await server.ssrLoadModule(
    "/src/features/home/sidebar/sidebar-conversation-model.ts",
  );
  const agents = [{ id: "agent-a", name: "Amy", avatar: "" }];
  const rooms = [
    { id: "dm-room", room_type: "dm", dm_target_agent_id: "agent-a", members: [] },
    { id: "group-room", room_type: "room", name: "项目组", members: [{ id: "agent-a" }] },
    { id: "idle-room", room_type: "dm", dm_target_agent_id: "agent-a", members: [] },
  ];
  const conversations = rooms.map((room, index) => ({
    conversation_id: `${room.id}-conversation`,
    is_active: true,
    last_activity: `2026-07-20T0${index + 1}:00:00.000Z`,
    last_reply_preview: "preview",
    message_count: 1,
    room_id: room.id,
    room_type: room.room_type,
    session_key: `session:${room.id}`,
    status: "active",
    title: room.id,
  }));

  const items = buildConversationItems({
    activeRoomIds: new Set(["group-room"]),
    agents,
    conversations,
    rooms,
    untitledRoomLabel: "未命名 Room",
  });
  assert.deepEqual(
    Object.fromEntries(items.map((item) => [item.roomId, item.isWorking])),
    { "dm-room": false, "group-room": true, "idle-room": false },
  );
});

test("Room mention Markdown keeps the internal URL for the avatar chip", async () => {
  const { transformMarkdownUrl } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/core/markdown-renderer-shared.tsx",
  );
  const components = {
    a: ({ href, children }) => React.createElement("a", { href }, children),
  };
  const html = renderToStaticMarkup(React.createElement(
    ReactMarkdown,
    { components, urlTransform: transformMarkdownUrl },
    "[Tom](agent-mention://tom)",
  ));

  assert.match(
    html,
    /href="agent-mention:\/\/tom"/,
    "mention 协议不能被 react-markdown 默认 URL 清理器吞掉",
  );
  assert.equal(
    transformMarkdownUrl("javascript:alert(1)"),
    "",
    "危险协议仍必须被默认白名单拦截",
  );
});

test("real Room cancellation guidance is projected once into Amy Thread", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const { getRoomThreadMessages } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-thread-model.ts",
  );

  const guide = {
    agent_id: "",
    content: "@Amy 算了不用了",
    delivery_policy: "guide",
    message_id: "msg_user_1716c22bc29d6240762bcf11",
    role: "user",
    round_id: "goal_continuation_9263beccd6692dd24807",
    session_key: "room:group:91c68883cc96",
    source_round_id: "round_21eae091f80fa6a69b71ace2",
    target_agent_ids: ["367448a0264b"],
    timestamp: 1784083409342,
  };
  const amyReply = {
    agent_id: "367448a0264b",
    content: [{
      type: "text",
      text: "收到，这个任务取消了。有需要再找我。<nexus_room_no_reply/>",
    }],
    is_complete: true,
    message_id: "d71ae7953d4401554941272e",
    role: "assistant",
    round_id: "goal_continuation_9263beccd6692dd24807",
    session_key: "room:group:91c68883cc96",
    timestamp: 1784083437370,
  };
  const devinReply = {
    agent_id: "0ed5434a8c13",
    content: [{ type: "text", text: "不应进入 Amy Thread" }],
    is_complete: true,
    message_id: "devin-reply",
    role: "assistant",
    round_id: "goal_continuation_9263beccd6692dd24807",
    session_key: "room:group:91c68883cc96",
    timestamp: 1784083437371,
  };
  const messages = [guide, amyReply, devinReply];

  const mainTimeline = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {
      "0ed5434a8c13": "Devin",
      "367448a0264b": "Amy",
    },
    messages,
    pendingPermissions: [],
    pendingSlots: [],
  });
  assert.deepEqual(mainTimeline.userMessages, [], "引导不能再次出现在 Room 主时间线");
  assert.deepEqual(
    mainTimeline.completedEntries.map((entry) => entry.agent_id),
    [amyReply.agent_id],
    "本轮公区只应投影一次 Amy 回复",
  );

  const amyThread = getRoomThreadMessages(messages, "367448a0264b");
  assert.deepEqual(
    amyThread.map((message) => message.message_id),
    [guide.message_id, amyReply.message_id],
    "Amy Thread 只能接收这一条引导和 Amy 的执行链",
  );
  assert.equal(
    amyThread[1].content[0].text,
    "收到，这个任务取消了。有需要再找我。",
    "Thread 直接内容必须剥离 Room 控制标记",
  );
});

test("Room chat ACK with empty pending preserves the active slot", async () => {
  const { mergeChatAckPendingSlots } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const activeSlot = {
    agent_id: "367448a0264b",
    agent_round_id: "agent-round-active",
    msg_id: "slot-active",
    round_id: "round-active",
    status: "streaming",
    timestamp: 1784083409342,
  };
  const emptyAck = {
    client_message_id: "client-message-queued",
    client_request_id: "client-request-queued",
    pending: [],
    pending_snapshot: false,
    round_id: "round-active",
    user_message_id: "user-message-queued",
  };

  assert.deepEqual(
    mergeChatAckPendingSlots([activeSlot], emptyAck),
    [activeSlot],
    "普通 queue ACK 不能覆盖仍在运行的 Agent slot",
  );
  assert.deepEqual(
    mergeChatAckPendingSlots([activeSlot], {
      ...emptyAck,
      pending_snapshot: true,
    }),
    [],
    "权威 pending snapshot 才可以用空数组清除 slot",
  );
});

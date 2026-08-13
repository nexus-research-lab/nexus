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

test("Goal 控制记录保留 canonical /goal 命令并只隐藏展示前缀", async () => {
  const { projectUserMessagePresentation } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/user/user-message-model.ts",
  );
  const timestamp = "2026-08-11T00:00:00Z";
  const canonical = projectUserMessagePresentation(
    false,
    "/goal 修复 Goal 与 WorkGraph 状态一致性",
    { metadata: { subtype: "goal_set" }, timestamp },
  );
  assert.equal(canonical.goal, true);
  assert.equal(canonical.displayContent, "修复 Goal 与 WorkGraph 状态一致性");
  assert.equal(canonical.hasContent, true);

  const legacy = projectUserMessagePresentation(
    false,
    "兼容旧版 Goal 控制记录",
    { metadata: { subtype: "goal_set" }, timestamp },
  );
  assert.equal(legacy.displayContent, "兼容旧版 Goal 控制记录");

  const ordinary = projectUserMessagePresentation(
    false,
    "/goal 只是普通历史文本",
    { timestamp },
  );
  assert.equal(ordinary.goal, false);
  assert.equal(ordinary.displayContent, "/goal 只是普通历史文本");
});

test("手输 /goal 在 ACK 前也投影为 Goal 控制记录", async () => {
  const { sendSessionMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/actions/conversation-chat-actions.ts",
  );
  let messages = [];
  await sendSessionMessage(
    "/goal 保持 Slash 与按钮即时显示一致",
    {
      acknowledgePermissionRequest: () => {},
      activeSessionKeyRef: { current: null },
      identity: { agent_id: "nexus", chat_type: "dm" },
      messages,
      pendingPermissions: [],
      sessionKey: "agent:nexus:ws:dm:goal-optimistic",
      setError: () => {},
      setMessages: (update) => {
        messages = typeof update === "function" ? update(messages) : update;
      },
      setPendingPermissions: () => {},
      wsSend: () => ({ disposition: "sent" }),
      wsState: "connected",
    },
  );
  assert.equal(messages.length, 1);
  assert.deepEqual(messages[0].metadata, { subtype: "goal_set" });
});

async function loadI18nValue(locale = "zh") {
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  return {
    locale,
    setLocale: () => {},
    t: (key, params = {}) => Object.entries(params).reduce(
      (message, [name, value]) => message.replaceAll(
        `{${name}}`,
        String(value),
      ),
      MESSAGES[locale][key] ?? key,
    ),
  };
}

async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      { value: await loadI18nValue(locale) },
      element,
    ),
  );
}

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
  assert.equal(goalActualTokens({
    ...goal,
    usage: {
      ...goal.usage,
      actual_tokens: 0,
    },
  }), 62_762, "矛盾的 actual_tokens=0 不能覆盖正数 breakdown");
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

  const html = await renderWithI18n(React.createElement(GoalStatusStrip, {
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

test("Goal status distinguishes auto-continuation suppression from an actual pause", async () => {
  const { buildGoalStatusStripModel } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-model.ts",
  );
  const { GoalStatusStrip } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-status-strip.tsx",
  );
  const baseGoal = {
    id: "goal-continuation-state",
    session_key: "room:group:continuation-state",
    objective: "Keep Room Goal continuation observable",
    continuation_count: 1,
    empty_progress_count: 0,
    continuation_state: "ready",
    version: 1,
    created_at: "2026-08-12T00:00:00Z",
    updated_at: "2026-08-12T00:00:00Z",
  };
  const recovering = buildGoalStatusStripModel({
    canResume: false,
    continuationHold: null,
    error: null,
    goal: {
      ...baseGoal,
      status: "active",
      empty_progress_count: 1,
      continuation_state: "recovering",
    },
    isGenerating: false,
  });
  assert.equal(recovering.statusLabel, "运行中");
  assert.equal(recovering.attentionTone, null);
  assert.equal(recovering.attentionMessage, null);

  const suppressed = buildGoalStatusStripModel({
    canResume: true,
    continuationHold: null,
    error: null,
    goal: {
      ...baseGoal,
      status: "active",
      empty_progress_count: 2,
      continuation_state: "suspended",
    },
    isGenerating: false,
  });
  assert.equal(suppressed.statusLabel, "自动续跑已停止");
  assert.equal(suppressed.attentionTone, "warning");
  assert.match(suppressed.statusTitle, /不是 Agent 主动暂停/);
  assert.match(suppressed.attentionMessage, /系统已停止自动续跑/);

  const paused = buildGoalStatusStripModel({
    canResume: true,
    continuationHold: null,
    error: null,
    goal: { ...baseGoal, status: "paused", continuation_state: "inactive" },
    isGenerating: false,
  });
  assert.equal(paused.statusLabel, "已暂停");
  assert.equal(paused.attentionMessage, null);
  assert.equal(paused.attentionTone, null);

  const held = buildGoalStatusStripModel({
    canResume: false,
    continuationHold: {
      label: "等待规划完成",
      detail: "Plan 模式结束后再继续执行。",
    },
    error: null,
    goal: { ...baseGoal, status: "active" },
    isGenerating: false,
  });
  assert.equal(held.statusLabel, "等待规划完成");
  assert.equal(held.statusTitle, "Plan 模式结束后再继续执行。");

  const html = await renderWithI18n(React.createElement(GoalStatusStrip, {
    canResume: true,
    compact: false,
    disabled: false,
    error: null,
    goal: {
      ...baseGoal,
      status: "active",
      empty_progress_count: 2,
      continuation_state: "suspended",
    },
    isGenerating: false,
    isLoading: false,
    scopeLabel: "房间 Goal",
    onClearRequest: () => {},
    onEdit: () => {},
    onPause: () => {},
    onRefresh: () => {},
    onResume: () => {},
  }));
  assert.match(html, />自动续跑已停止</);
  assert.match(html, /这不是 Agent 主动暂停/);
  assert.match(html, /aria-label="继续"/);
});

test("Goal clear follows the server-derived WorkGraph binding state", async () => {
  const {
    buildGoalControllerProjection,
    resolveGoalBindingBadgeModel,
    resolveGoalClearDisabledReason,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-model.ts",
  );
  const { GoalStatusStrip } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-status-strip.tsx",
  );
  const goal = {
    id: "goal-binding",
    session_key: "agent:nexus:ws:dm:binding",
    objective: "Keep binding authority on the server",
    status: "active",
    continuation_count: 0,
    empty_progress_count: 0,
    version: 1,
    created_at: "2026-08-11T00:00:00Z",
    updated_at: "2026-08-11T00:00:00Z",
  };

  assert.match(resolveGoalClearDisabledReason(null), /正在确认/);
  for (const state of ["standalone", "reserved"]) {
    assert.equal(resolveGoalClearDisabledReason({ state }), null);
    assert.equal(resolveGoalBindingBadgeModel({ state }), null);
    assert.deepEqual(buildGoalControllerProjection({
      dialog: { goal, kind: "clear" },
      draft: null,
      executionBinding: { state },
      goal,
      phase: null,
    }).dialog, { goal, kind: "clear" });
    const html = await renderWithI18n(React.createElement(GoalStatusStrip, {
      canResume: false,
      clearDisabledReason: null,
      compact: false,
      disabled: false,
      error: null,
      executionBinding: { state },
      goal,
      isGenerating: false,
      isLoading: false,
      scopeLabel: "Goal",
      onClearRequest: () => {},
      onEdit: () => {},
      onPause: () => {},
      onRefresh: () => {},
      onResume: () => {},
    }));
    assert.doesNotMatch(html, /data-goal-binding-state=/);
    assert.doesNotMatch(html, />独立 Goal</);
    assert.doesNotMatch(html, />已关联工作图</);
  }
  for (const state of ["pending", "confirmed", "conflict"]) {
    const reason = resolveGoalClearDisabledReason({ state });
    assert.equal(typeof reason, "string");
    const projection = buildGoalControllerProjection({
      dialog: { goal, kind: "clear" },
      draft: null,
      executionBinding: { state },
      goal,
      phase: null,
    });
    assert.equal(projection.clearDisabledReason, reason);
    assert.deepEqual(projection.dialog, { kind: "none" });
  }

  const bindingCases = [
    [{ state: "pending" }, "pending", "关联确认中"],
    [{ execution_id: "execution-binding", state: "confirmed" }, "confirmed", "已关联工作图"],
    [{ state: "conflict" }, "conflict", "关联冲突"],
    [null, "unavailable", "关联状态不可用"],
  ];
  for (const [binding, displayState, label] of bindingCases) {
    const badge = resolveGoalBindingBadgeModel(binding);
    assert.equal(badge.state, displayState);
    const clearDisabledReason = resolveGoalClearDisabledReason(binding);
    const html = await renderWithI18n(React.createElement(GoalStatusStrip, {
      canResume: false,
      clearDisabledReason,
      compact: false,
      disabled: false,
      error: null,
      executionBinding: binding,
      goal,
      isGenerating: false,
      isLoading: false,
      scopeLabel: "Goal",
      onClearRequest: () => {},
      onEdit: () => {},
      onPause: () => {},
      onRefresh: () => {},
      onResume: () => {},
    }));
    assert.match(html, /data-goal-binding-state=/);
    assert.match(html, new RegExp(`data-goal-binding-state="${displayState}"`));
    assert.match(html, new RegExp(`>${label}<`));
    assert.match(html, />运行中</);
    assert.ok(
      html.indexOf("运行中") < html.indexOf(label),
      "Goal lifecycle must remain primary to the WorkGraph binding badge",
    );
    if (clearDisabledReason) {
      assert.match(html, /disabled=""/);
      assert.match(html, new RegExp(`aria-label="清除：${clearDisabledReason}`));
    }
  }
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

test("关闭最后标签直接提交正常 draft 并安全停止旧 runtime", async () => {
  const { replaceFinalConversation } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/controls/conversation-tabs/final-conversation-replacement.ts",
  );
  const calls = [];
  const internalConversation = {
    conversation_id: "old",
    created_at: 1,
    last_activity_at: 1,
    options: {},
    room_id: "room-a",
    session_id: null,
    session_key: "room:old",
    title: "Old",
    is_draft: false,
  };

  await replaceFinalConversation({
    closeConversation: async (conversationId) => {
      calls.push(`close:${conversationId}`);
    },
    commitConversation: (conversationId) => {
      calls.push(`commit:${conversationId}`);
    },
    conversation: internalConversation,
    createConversation: async () => {
      calls.push("create");
      return "fresh-draft";
    },
    isCurrent: () => true,
  });
  assert.deepEqual(
    calls,
    ["create", "commit:fresh-draft", "close:old"],
    "已开始会话的替换 ID 必然不同，应先切到输入框再后台关旧 runtime",
  );

  calls.length = 0;
  await replaceFinalConversation({
    closeConversation: async (conversationId) => {
      calls.push(`close:${conversationId}`);
    },
    commitConversation: (conversationId) => {
      calls.push(`commit:${conversationId}`);
    },
    conversation: {...internalConversation, is_draft: true},
    createConversation: async () => {
      calls.push("create");
      return "old";
    },
    isCurrent: () => true,
  });
  assert.deepEqual(
    calls,
    ["close:old", "create", "commit:old"],
    "当唯一 draft 被复用时必须先等 close，避免迟到的中断破坏新输入",
  );

  calls.length = 0;
  await replaceFinalConversation({
    closeConversation: async () => {
      calls.push("close");
      throw new Error("runtime close failed");
    },
    commitConversation: () => {
      calls.push("commit");
    },
    conversation: {...internalConversation, is_draft: true},
    createConversation: async () => {
      calls.push("create");
      return "old";
    },
    isCurrent: () => true,
  });
  assert.deepEqual(
    calls,
    ["close"],
    "唯一 draft 的 runtime 未确认停止时不得复用同一 ID",
  );

  calls.length = 0;
  await replaceFinalConversation({
    closeConversation: async () => {
      calls.push("close");
    },
    commitConversation: (conversationId) => {
      calls.push(`commit:${conversationId}`);
    },
    conversation: {
      ...internalConversation,
      conversation_id: "external-session:feishu",
      options: { external_session: true },
    },
    createConversation: async () => {
      calls.push("create");
      return "internal-draft";
    },
    isCurrent: () => true,
  });
  assert.deepEqual(
    calls,
    ["create", "commit:internal-draft"],
    "外部 Session 关闭最后标签时不得调用 Room runtime close",
  );

  calls.length = 0;
  await replaceFinalConversation({
    closeConversation: async () => {
      calls.push("close");
    },
    commitConversation: () => {
      calls.push("commit");
    },
    conversation: internalConversation,
    createConversation: async () => {
      calls.push("create");
      return null;
    },
    isCurrent: () => true,
  });
  assert.deepEqual(
    calls,
    ["create"],
    "ensure 失败必须保留旧标签与 runtime",
  );

  calls.length = 0;
  let currentCheckCount = 0;
  await replaceFinalConversation({
    closeConversation: async () => {
      calls.push("close");
    },
    commitConversation: () => {
      calls.push("commit");
    },
    conversation: internalConversation,
    createConversation: async () => {
      calls.push("create");
      return "fresh-draft";
    },
    isCurrent: () => {
      currentCheckCount += 1;
      return currentCheckCount === 1;
    },
  });
  assert.deepEqual(
    calls,
    ["create"],
    "ensure 期间选中历史会话或切换 Room 后，旧事务不得覆盖新导航",
  );
});

test("最后标签替换只能在原 Room 的原导航 revision 提交", async () => {
  const { isFinalConversationReplacementCurrent } = await server.ssrLoadModule(
    "/src/pages/room/orchestration/use-room-page-navigation.ts",
  );
  const currentScope = {
    activeConversationId: "old",
    currentEpoch: 4,
    currentRoomId: "room-a",
    expectedConversationId: "old",
    expectedEpoch: 4,
    expectedRoomId: "room-a",
    openConversationIds: ["old"],
    selectedConversationId: "old",
  };

  assert.equal(isFinalConversationReplacementCurrent(currentScope), true);
  assert.equal(isFinalConversationReplacementCurrent({
    ...currentScope,
    selectedConversationId: "history",
  }), false);
  assert.equal(isFinalConversationReplacementCurrent({
    ...currentScope,
    currentRoomId: "room-b",
  }), false);
  assert.equal(isFinalConversationReplacementCurrent({
    ...currentScope,
    currentEpoch: 6,
  }), false, "切走再切回的 ABA 也必须使旧事务过期");
  assert.equal(isFinalConversationReplacementCurrent({
    ...currentScope,
    openConversationIds: ["old", "history"],
  }), false, "等待期间打开历史标签后不得精确覆盖集合");
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

test("工作区预览 breadcrumb 投影工作区根目录与文件父目录", async () => {
  const {
    getWorkspaceFileLocationLabel,
    getWorkspaceRootLabel,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/workspace/controller/workspace-path-model.ts",
  );

  assert.equal(
    getWorkspaceRootLabel("/Users/leemysw/Projects/nexus/", "工作区"),
    "nexus",
  );
  assert.equal(
    getWorkspaceRootLabel("C:\\Users\\leemysw\\Projects\\nexus", "工作区"),
    "nexus",
  );
  assert.equal(getWorkspaceRootLabel("", "工作区"), "工作区");
  assert.equal(getWorkspaceFileLocationLabel("AGENTS.md", "nexus"), "nexus");
  assert.equal(
    getWorkspaceFileLocationLabel("memory/project_leemysw.md", "nexus"),
    "memory",
  );
  assert.equal(
    getWorkspaceFileLocationLabel("memory/output/report.md", "nexus"),
    "memory/output",
  );
});

test("工作区空预览跟随界面语言", async () => {
  const { WorkspaceFilePreviewPanel } = await server.ssrLoadModule(
    "/src/features/conversation/shared/editor/workspace-file-preview-panel.tsx",
  );
  const element = React.createElement(WorkspaceFilePreviewPanel, {
    agentId: "agent-a",
    headerLocationLabel: "nexus",
    isPreviewFocused: false,
    onTogglePreviewFocus: () => {},
    path: null,
  });
  const chinese = await renderWithI18n(element);
  const english = await renderWithI18n(element, "en");

  assert.match(chinese, /工作区预览/);
  assert.match(chinese, /从文件列表选择一个文件/);
  assert.match(english, /Workspace Preview/);
  assert.match(english, /Select a file from the list to preview it here/);
  assert.doesNotMatch(english, /工作区|从文件列表/);
});

test("工作区文件操作跟随界面语言", async () => {
  const originalWindow = globalThis.window;
  globalThis.window = {
    __NEXUS_DESKTOP_RUNTIME__: { app_mode: "desktop" },
  };
  try {
    const englishI18n = await loadI18nValue("en");
    const chineseI18n = await loadI18nValue("zh");
    const { getWorkspaceFileExternalActionCopy } = await server.ssrLoadModule(
      "/src/lib/workspace-file-action.ts",
    );
    assert.deepEqual(
      getWorkspaceFileExternalActionCopy(englishI18n.t, "MEMORY.md"),
      {
        ariaLabel: "Show MEMORY.md in folder",
        label: "Open",
        mode: "reveal",
        title: "Show MEMORY.md in folder",
      },
    );
    assert.equal(
      getWorkspaceFileExternalActionCopy(chineseI18n.t, "MEMORY.md").title,
      "在文件夹中显示 MEMORY.md",
    );

    const {
      WorkspaceFileDownloadButton,
      WorkspaceFilePreviewFocusButton,
    } = await server.ssrLoadModule(
      "/src/features/conversation/shared/editor/workspace-file-preview-chrome.tsx",
    );
    const chrome = React.createElement(React.Fragment, null,
      React.createElement(WorkspaceFileDownloadButton, {
        agentId: "agent-a",
        fileName: "MEMORY.md",
        path: "MEMORY.md",
      }),
      React.createElement(WorkspaceFilePreviewFocusButton, {
        isPreviewFocused: false,
        onTogglePreviewFocus: () => {},
      }),
      React.createElement(WorkspaceFilePreviewFocusButton, {
        isPreviewFocused: true,
        onTogglePreviewFocus: () => {},
      }),
    );
    const englishChrome = await renderWithI18n(chrome, "en");
    assert.match(englishChrome, /aria-label="Show MEMORY\.md in folder"/);
    assert.match(englishChrome, /aria-label="Focus preview"/);
    assert.match(englishChrome, /aria-label="Show file list"/);
    assert.doesNotMatch(englishChrome, /文件夹|聚焦|文件列表/);

    const { buildTextFileEditorPresentation } = await server.ssrLoadModule(
      "/src/features/conversation/shared/editor/text/text-file-editor-model.ts",
    );
    const presentation = buildTextFileEditorPresentation({
      fileType: "markdown",
      isDirty: true,
      isEditing: false,
      isExternalWriting: false,
      isSaving: false,
      liveState: undefined,
      translate: englishI18n.t,
    });
    assert.equal(presentation.editLabel, "Edit");
    assert.equal(presentation.saveLabel, "Save");
  } finally {
    globalThis.window = originalWindow;
  }
});

test("工作区文件树使用按文件名与扩展名区分的彩色图标", async () => {
  const {
    getWorkspaceDirectoryIcon,
    getWorkspaceFileVisual,
  } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/tree/workspace-file-tree-model.ts",
  );
  const iconSources = [
    getWorkspaceFileVisual("components.json").iconSrc,
    getWorkspaceFileVisual("index.html").iconSrc,
    getWorkspaceFileVisual("package.json").iconSrc,
    getWorkspaceFileVisual("pnpm-lock.yaml").iconSrc,
    getWorkspaceFileVisual("tsconfig.json").iconSrc,
    getWorkspaceFileVisual("vite.config.ts").iconSrc,
  ];

  assert.equal(new Set(iconSources).size, iconSources.length);
  assert.notEqual(getWorkspaceDirectoryIcon(false), getWorkspaceDirectoryIcon(true));
  iconSources.forEach((iconSource) => assert.match(iconSource, /svg/));
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

test("Agent 名称只做本地格式预检且允许重名语义", async () => {
  const { validateAgentNameDraft } = await server.ssrLoadModule(
    "/src/features/agents/options/editor/agent-name-validation.ts",
  );

  assert.deepEqual(await validateAgentNameDraft("  Amy   Agent  "), {
    is_available: true,
    is_valid: true,
    name: "  Amy   Agent  ",
    normalized_name: "Amy Agent",
    reason: "",
    workspace_path: null,
  });
  assert.equal((await validateAgentNameDraft("A")).is_valid, false);
  assert.equal((await validateAgentNameDraft("Amy🙂")).is_valid, false);
});

test("Agent 首次保存接受服务端来源回写但拒绝更新的用户草稿", async () => {
  const {
    buildAgentEditorCommandScopeKey,
    buildAgentEditorScopeKey,
    createAgentOptionsDraft,
    reconcileAgentOptionsDraft,
  } = await server.ssrLoadModule(
    "/src/features/agents/options/editor/agent-options-draft.ts",
  );
  const { isAgentOptionsSaveCurrent } = await server.ssrLoadModule(
    "/src/features/agents/options/editor/agent-options-save-transaction.ts",
  );
  const beforeSource = {
    agentId: "agent-1",
    initial: {
      avatar: "1",
      description: "",
      options: { permission_mode: "plan" },
      title: "Reviewer",
      vibeTags: [],
    },
    isMain: false,
    kind: "edit",
  };
  const afterSource = {
    ...beforeSource,
    initial: {
      ...beforeSource.initial,
      options: { permission_mode: "acceptEdits" },
    },
  };
  const beforeSourceScopeKey = buildAgentEditorScopeKey({
    draft: createAgentOptionsDraft({
      defaultTitle: "Agent",
      initial: beforeSource.initial,
    }),
    isActive: true,
    source: beforeSource,
  });
  const afterSourceScopeKey = buildAgentEditorScopeKey({
    draft: createAgentOptionsDraft({
      defaultTitle: "Agent",
      initial: afterSource.initial,
    }),
    isActive: true,
    source: afterSource,
  });
  const commandScopeKey = buildAgentEditorCommandScopeKey({
    isActive: true,
    source: beforeSource,
  });
  const token = {
    commandScopeKey,
    draftRevision: 1,
    id: 1,
    sourceScopeKey: beforeSourceScopeKey,
  };
  const beforeDraft = createAgentOptionsDraft({
    defaultTitle: "Agent",
    initial: beforeSource.initial,
  });
  const afterDraft = createAgentOptionsDraft({
    defaultTitle: "Agent",
    initial: afterSource.initial,
  });

  assert.equal(
    reconcileAgentOptionsDraft(beforeDraft, beforeDraft, afterDraft),
    afterDraft,
    "无本地修改时应接受服务端来源回写",
  );
  const newerLocalDraft = { ...beforeDraft, title: "Reviewer local" };
  assert.equal(
    reconcileAgentOptionsDraft(newerLocalDraft, beforeDraft, afterDraft),
    newerLocalDraft,
    "自动保存期间的新草稿不得被旧响应覆盖",
  );

  assert.notEqual(afterSourceScopeKey, beforeSourceScopeKey);
  assert.equal(
    buildAgentEditorCommandScopeKey({ isActive: true, source: afterSource }),
    commandScopeKey,
  );
  assert.equal(
    isAgentOptionsSaveCurrent(token, {
      commandScopeKey,
      draftRevision: 1,
      sourceScopeKey: afterSourceScopeKey,
      token,
    }, false),
    true,
    "PATCH 成功后的来源回写仍应归属于首次保存",
  );
  assert.equal(
    isAgentOptionsSaveCurrent(token, {
      commandScopeKey,
      draftRevision: 2,
      sourceScopeKey: afterSourceScopeKey,
      token,
    }, false),
    false,
    "保存过程中出现的新用户草稿不得收到旧成功反馈",
  );
});

test("Agent 自动保存只调度新的有效草稿版本", async () => {
  const { shouldScheduleAgentOptionsAutoSave } = await server.ssrLoadModule(
    "/src/features/agents/options/editor/use-agent-options-auto-save.ts",
  );
  const ready = {
    attempted: null,
    canSave: true,
    draftRevision: 3,
    enabled: true,
    isDirty: true,
    isSaving: false,
    scopeKey: "agent-1",
  };

  assert.equal(shouldScheduleAgentOptionsAutoSave(ready), true);
  assert.equal(
    shouldScheduleAgentOptionsAutoSave({
      ...ready,
      attempted: { draftRevision: 3, scopeKey: "agent-1" },
    }),
    false,
    "同一草稿失败后不得无休止重试",
  );
  assert.equal(
    shouldScheduleAgentOptionsAutoSave({ ...ready, draftRevision: 4 }),
    true,
    "用户继续编辑后应调度下一次保存",
  );
  assert.equal(
    shouldScheduleAgentOptionsAutoSave({ ...ready, canSave: false }),
    false,
  );
});

test("Room Agent 面板用最新目录覆盖上下文中的旧模型快照", async () => {
  const { resolveCurrentRoomMemberAgents } = await server.ssrLoadModule(
    "/src/pages/room/controller/model/room-member-model.ts",
  );
  const staleAgent = {
    agent_id: "agent-1",
    name: "Nexus",
    workspace_path: "/tmp/nexus",
    options: {
      provider: "glm-coding-plan",
      model: "glm-5.2",
    },
    created_at: 0,
    status: "active",
    avatar: null,
    description: null,
    vibe_tags: [],
    skills_count: null,
  };
  const currentAgent = {
    ...staleAgent,
    options: {
      provider: "deepseek",
      model: "deepseek-v4-flash",
    },
  };
  const contexts = [{
    member_agents: [staleAgent],
    members: [],
  }];

  assert.deepEqual(
    resolveCurrentRoomMemberAgents(contexts, [currentAgent])[0].options,
    currentAgent.options,
  );
});

test("会话标签首次只打开活动项并按创建时间插入主动选择项", async () => {
  const {
    getCloseFallbackConversationId,
    getConversationIdsByCreationTime,
    getInitialOpenConversationIds,
    reconcileOpenConversationIds,
    resolveSelectedDraftConversationId,
    shouldPersistConversationTabs,
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
  const draftConversations = [
    {
      conversation_id: "started",
      created_at: 300,
      is_draft: false,
      message_count: 0,
      options: {},
    },
    {
      conversation_id: "historical-draft",
      created_at: 100,
      is_draft: true,
      message_count: 0,
      options: {},
    },
    {
      conversation_id: "selected-draft",
      created_at: 200,
      is_draft: true,
      message_count: 0,
      options: {},
    },
    {
      conversation_id: "external-draft",
      created_at: 400,
      is_draft: true,
      message_count: 0,
      options: {external_session: true},
    },
  ];

  assert.deepEqual(
    orderedIds,
    ["first", "second", "third"],
    "消息活动时间不得改变标签创建顺序",
  );
  assert.deepEqual(
    getInitialOpenConversationIds("third", orderedIds),
    ["third"],
    "首次进入 Room 时只应打开最后活动标签",
  );
  assert.deepEqual(
    reconcileOpenConversationIds({
      conversationId: "third",
      currentIds: ["third"],
      orderedIds,
      pendingClosedId: null,
    }),
    ["third"],
    "新发现的历史会话不得自动加入标签栏",
  );
  assert.deepEqual(
    reconcileOpenConversationIds({
      conversationId: "second",
      currentIds: ["first", "third"],
      orderedIds,
      pendingClosedId: null,
    }),
    orderedIds,
    "重新打开标签必须回到其创建时间位置",
  );
  assert.equal(
    resolveSelectedDraftConversationId(
      draftConversations,
      "selected-draft",
    ),
    "selected-draft",
    "当前内部 Session 被服务端明确标记为草稿时，新建动作必须留在当前页",
  );
  assert.equal(
    resolveSelectedDraftConversationId(
      draftConversations,
      "started",
    ),
    null,
    "当前 Session 不是草稿时不得扫描并拉回其他历史草稿",
  );
  assert.equal(
    resolveSelectedDraftConversationId(
      draftConversations,
      "external-draft",
    ),
    null,
    "外部 Session 即使错误携带草稿标记也不得参与内部 Session 复用",
  );
  assert.deepEqual(
    reconcileOpenConversationIds({
      conversationId: "started",
      currentIds: ["historical-draft", "selected-draft", "started"],
      orderedIds: ["historical-draft", "selected-draft", "started"],
      pendingClosedId: null,
    }),
    ["historical-draft", "selected-draft", "started"],
    "标签视图不得自行折叠或删除历史草稿",
  );
  assert.equal(
    getCloseFallbackConversationId(
      [{ conversation_id: "only" }],
      "only",
    ),
    null,
    "关闭唯一标签没有相邻回退项，应由控制器确保新 draft 来替换",
  );
  assert.equal(
    shouldPersistConversationTabs({
      activeConversationId: "second",
      routeConversationId: "third",
    }),
    false,
    "点击事务的乐观活动项不得被尚未更新的旧路由反向持久化",
  );
  assert.equal(
    shouldPersistConversationTabs({
      activeConversationId: "second",
      routeConversationId: "second",
    }),
    true,
    "路由追上活动项后才能收敛持久化标签状态",
  );
});

test("Room 导航持久化完整标签栏并让关闭项保持关闭", async () => {
  const { useRoomNavigationStore } = await server.ssrLoadModule(
    "/src/store/room-navigation.ts",
  );
  const migrate = useRoomNavigationStore.persist.getOptions().migrate;
  assert.equal(typeof migrate, "function");
  assert.deepEqual(
    await migrate({
      last_active_conversation_by_room: {
        "legacy-room": "legacy-conversation",
      },
    }, 1),
    {
      conversation_tabs_by_room: {
        "legacy-room": {
          active_conversation_id: "legacy-conversation",
          open_conversation_ids: ["legacy-conversation"],
        },
      },
    },
    "旧版最后活动项应迁移成单标签，而不是把历史全部打开",
  );
  assert.deepEqual(
    await migrate({
      conversation_tabs_by_room: {
        "empty-room": {
          active_conversation_id: null,
          open_conversation_ids: [],
        },
      },
    }, 3),
    { conversation_tabs_by_room: {} },
    "v3 错误空快照必须在 v4 丢弃，重新进入时恢复有效会话",
  );

  useRoomNavigationStore.setState({ conversation_tabs_by_room: {} });
  const roomId = "room-tabs-persistence";

  useRoomNavigationStore.getState().save_room_conversation_tabs(
    roomId,
    ["first", "third"],
    "third",
  );
  assert.deepEqual(
    useRoomNavigationStore.getState().conversation_tabs_by_room[roomId],
    {
      active_conversation_id: "third",
      open_conversation_ids: ["first", "third"],
    },
    "离开 Room 前应保存完整标签数量、顺序和活动项",
  );

  useRoomNavigationStore.getState().remember_last_active_conversation(
    roomId,
    "second",
  );
  assert.deepEqual(
    useRoomNavigationStore.getState().conversation_tabs_by_room[roomId],
    {
      active_conversation_id: "second",
      open_conversation_ids: ["first", "third", "second"],
    },
    "从历史主动选择会话时才应把它加入打开集合",
  );

  useRoomNavigationStore.getState().save_room_conversation_tabs(
    roomId,
    ["first", "second"],
    "second",
  );
  assert.deepEqual(
    useRoomNavigationStore.getState().conversation_tabs_by_room[roomId],
    {
      active_conversation_id: "second",
      open_conversation_ids: ["first", "second"],
    },
    "关闭的标签不得在后续进入或持久化恢复时自动补回",
  );

});

test("删除外部 IM Session 会从持久化标签栏移除旧身份", async () => {
  const { useRoomNavigationStore } = await server.ssrLoadModule(
    "/src/store/room-navigation.ts",
  );
  useRoomNavigationStore.setState({conversation_tabs_by_room: {}});
  useRoomNavigationStore.getState().save_room_conversation_tabs(
    "room-im",
    ["local", "external-session:agent:a:tg:dm:old"],
    "external-session:agent:a:tg:dm:old",
  );

  useRoomNavigationStore.getState().forget_conversation(
    "room-im",
    "external-session:agent:a:tg:dm:old",
  );

  assert.deepEqual(
    useRoomNavigationStore.getState().conversation_tabs_by_room["room-im"],
    {
      active_conversation_id: "local",
      open_conversation_ids: ["local"],
    },
  );
});

test("Room 无显式会话路由时优先恢复用户最后活动项", async () => {
  const {
    buildRoomConversationViews,
    resolveCurrentRoomContext,
    resolveSelectedConversationId,
  } = await server.ssrLoadModule(
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
    resolveSelectedConversationId(null, conversations, ["remembered"]),
    "remembered",
    "切回 Room 时应恢复用户最后激活的标签",
  );
  assert.equal(
    resolveSelectedConversationId("latest", conversations, ["remembered"]),
    "latest",
    "显式 Conversation URL 仍然优先于本地恢复偏好",
  );
  assert.equal(
    resolveSelectedConversationId(null, conversations, ["removed"]),
    "latest",
    "已删除的恢复目标必须回退到当前有效会话",
  );
  assert.equal(
    resolveSelectedConversationId(
      null,
      conversations,
      ["removed", "remembered"],
    ),
    "remembered",
    "活动标签失效时应优先恢复仍然有效的已打开标签",
  );
  assert.equal(
    resolveSelectedConversationId(null, conversations, []),
    "latest",
    "没有有效恢复偏好时应进入当前有效会话，不保留无 Conversation 页面",
  );
  assert.deepEqual(
    resolveCurrentRoomContext(
      conversations.map((conversation) => ({
        conversation: { id: conversation.conversation_id },
      })),
      null,
    ),
    { conversation: { id: "latest" } },
    "页面模型必须始终回退到有效 Room 上下文",
  );
  const untitledViews = buildRoomConversationViews([{
    conversation: {
      conversation_type: "topic",
      created_at: "2026-07-27T00:00:00Z",
      id: "new-conversation",
      is_draft: true,
      last_activity_at: "2026-07-27T00:00:00Z",
      message_count: 0,
      title: "",
    },
    room: {
      id: "room-a",
      name: "Smoke",
      room_type: "room",
    },
    sessions: [],
  }]);
  assert.equal(
    untitledViews[0].title,
    "",
    "空标题必须留给界面本地化显示“新会话”，不能回退成 Room 名称",
  );
  assert.equal(
    untitledViews[0].is_draft,
    true,
    "Room 会话视图必须保留服务端显式草稿状态",
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
    preferredConversationIds: [externalConversation.conversation_id],
    routeRoomId: "room-a",
    routeSessionKey: null,
  });
  assert.equal(
    model.conversation.selectedId,
    externalConversation.conversation_id,
    "外部 Session 标签加载完成后也应恢复为最后活动项",
  );
});

test("会话快照只把可见的非 synthetic user 视为用户输入", async () => {
  const {
    doConversationMessagesBelongToScope,
    hasConversationUserInput,
    shouldReportConversationSnapshot,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/use-conversation-snapshot-reporter.ts",
  );
  assert.equal(
    hasConversationUserInput([
      { role: "assistant" },
      { role: "user", hidden_from_user: true },
      { role: "user", is_synthetic: true },
    ]),
    false,
    "助手、隐藏或 synthetic 用户消息都不能结束草稿",
  );
  assert.equal(
    hasConversationUserInput([{ role: "user" }]),
    true,
    "可见的真实用户消息必须结束草稿",
  );
  assert.equal(
    shouldReportConversationSnapshot({
      messages: [{
        conversation_id: "conversation-old",
        role: "user",
        session_key: "room:group:conversation-old",
      }],
      observed_scope_key: "conversation-old",
      scope_key: "conversation-new",
    }),
    false,
    "切换 Session 的首个 render 不能把上一会话的用户消息投影给新 draft",
  );
  assert.equal(
    shouldReportConversationSnapshot({
      messages: [{
        conversation_id: "conversation-new",
        role: "user",
        session_key: "room:group:conversation-new",
      }],
      observed_scope_key: "conversation-new",
      scope_key: "conversation-new",
    }),
    true,
    "消息集合与当前 scope 对齐后才允许提交会话快照",
  );
  assert.equal(
    doConversationMessagesBelongToScope([{
      conversation_id: "conversation-old",
      role: "user",
      session_key: "room:group:conversation-old",
    }], "conversation-new"),
    false,
    "即使过渡期发生额外 render，旧 Session 的消息也不能污染新 draft",
  );
  assert.equal(
    shouldReportConversationSnapshot({
      messages: [{
        conversation_id: "conversation-old",
        role: "user",
        session_key: "room:group:conversation-old",
      }],
      observed_scope_key: "conversation-new",
      scope_key: "conversation-new",
    }),
    false,
    "callback 引用变化导致第二次 effect 时，旧消息仍不得通过完整上报判定",
  );
  assert.equal(
    doConversationMessagesBelongToScope([{
      conversation_id: "conversation-new",
      role: "user",
      session_key: "room:group:conversation-new",
    }, {
      conversation_id: "conversation-new",
      role: "assistant",
      session_key: "agent:agent-a:ws:group:conversation-new",
    }], "conversation-new"),
    true,
    "共享用户消息与 Agent 回复都应归属于当前 Room conversation",
  );
  assert.equal(
    doConversationMessagesBelongToScope([{
      role: "assistant",
      session_key: "agent:agent-a:ws:group:conversation-new",
    }], "conversation-new"),
    true,
    "旧消息缺少 conversation_id 时必须从 Group session_key 兼容识别 scope",
  );
});

test("Room 快照只在存在用户输入时把新 Session 标记为已开始", async () => {
  const { applyConversationSnapshotToRoomContexts } = await server.ssrLoadModule(
    "/src/pages/room/controller/model/room-snapshot-model.ts",
  );
  const contexts = [{
    conversation: {
      id: "conversation-new",
      is_draft: true,
      message_count: 0,
    },
    sessions: [],
  }];

  const withAssistantOnly = applyConversationSnapshotToRoomContexts(contexts, {
    conversation_id: "conversation-new",
    has_user_input: false,
    message_count: 1,
    room_session_id: null,
  });
  assert.equal(
    withAssistantOnly[0].conversation.message_count,
    1,
    "助手消息仍应单调提升本地消息计数",
  );
  assert.equal(
    withAssistantOnly[0].conversation.is_draft,
    true,
    "仅有助手或内部消息时不能把 Session 误判为用户已开始",
  );
  const withUserInput = applyConversationSnapshotToRoomContexts(withAssistantOnly, {
    conversation_id: "conversation-new",
    has_user_input: true,
    message_count: 2,
    room_session_id: null,
  });
  assert.equal(
    withUserInput[0].conversation.is_draft,
    false,
    "出现真实用户输入后必须立即退出草稿态",
  );
  const withPartialHistory = applyConversationSnapshotToRoomContexts(withUserInput, {
    conversation_id: "conversation-new",
    has_user_input: false,
    message_count: 0,
    room_session_id: null,
  });
  assert.equal(
    withPartialHistory[0].conversation.message_count,
    2,
    "局部历史窗口不得把已开始的 Session 回退成空会话",
  );
  assert.equal(
    withPartialHistory[0].conversation.is_draft,
    false,
    "局部历史窗口不得把已开始的 Session 恢复成草稿",
  );
});

test("聊天侧栏只按 Room 活动态显示 DM 和群组", async () => {
  const {
    getRoomActivity,
    pruneRoomActivity,
    replaceRoomActivitySnapshot,
    replaceRoomInteractionSnapshot,
    updateRoomActivity,
    updateRoomInteraction,
  } = await server.ssrLoadModule("/src/features/home/room-activity-resource.ts");

  pruneRoomActivity(new Set());
  updateRoomActivity("dm-room", "dm-round", "running");
  updateRoomActivity("group-room", "group-round", "running");
  updateRoomActivity("group-room", "group-round", "running", "agent_round", "slot-a");
  updateRoomActivity("group-room", "group-round", "running", "agent_round", "slot-b");
  updateRoomActivity("group-room", "group-round", "finished", "agent_round", "slot-a");
  assert.deepEqual(
    Object.fromEntries([...getRoomActivity()].sort()),
    { "dm-room": "working", "group-room": "working" },
    "DM 和群组必须共享同一 Room 活动态集合",
  );

  updateRoomInteraction("group-room", "permission-a", true);
  updateRoomInteraction("group-room", "permission-b", true);
  assert.equal(
    getRoomActivity().get("group-room"),
    "waiting",
    "待确认状态必须覆盖同一 Room 的工作中展示",
  );
  updateRoomInteraction("group-room", "permission-a", false);
  assert.equal(
    getRoomActivity().get("group-room"),
    "waiting",
    "仍有其他人工交互时不能提前恢复工作中",
  );
  updateRoomActivity("group-room", "group-round", "finished");
  updateRoomInteraction("group-room", "permission-b", false);
  replaceRoomActivitySnapshot("dm-room", "dm-round", false, ["permission-replayed"]);
  assert.deepEqual(
    Object.fromEntries(getRoomActivity()),
    { "dm-room": "waiting" },
    "重连快照必须恢复待确认状态",
  );
  replaceRoomActivitySnapshot("dm-room", "dm-round", false, []);
  assert.deepEqual(Object.fromEntries(getRoomActivity()), {}, "空快照应清除 Room 活动态");

  updateRoomActivity("group-room", "group-round-new", "running");
  replaceRoomInteractionSnapshot("group-room", ["permission-global"]);
  assert.equal(getRoomActivity().get("group-room"), "waiting");
  replaceRoomInteractionSnapshot("group-room", []);
  assert.equal(
    getRoomActivity().get("group-room"),
    "working",
    "Room 全局交互快照不得清除 conversation 执行槽",
  );
  replaceRoomActivitySnapshot("group-room", "group-round-new", false, []);

  updateRoomActivity("group-room", "runtime-round", "running");
  updateRoomActivity(
    "group-room",
    "logical-root",
    "running",
    "agent_round",
    "continued-slot",
  );
  updateRoomActivity("group-room", "runtime-round", "finished");
  assert.deepEqual(
    Object.fromEntries(getRoomActivity()),
    {},
    "最后一个 runtime round 结束时必须清理 logical root 下的孤儿 Agent 活动态",
  );
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
    agents,
    conversations,
    roomActivity: new Map([
      ["dm-room", "waiting"],
      ["group-room", "working"],
    ]),
    rooms,
    untitledRoomLabel: "未命名 Room",
  });
  assert.deepEqual(
    Object.fromEntries(items.map((item) => [item.roomId, item.activityStatus])),
    { "dm-room": "waiting", "group-room": "working", "idle-room": null },
  );

  const { ConversationRow } = await server.ssrLoadModule(
    "/src/features/home/sidebar/sidebar-list-rows.tsx",
  );
  const waitingHtml = await renderWithI18n(React.createElement(ConversationRow, {
    isActive: false,
    item: items.find((item) => item.roomId === "dm-room"),
    onClick: () => {},
  }));
  assert.match(waitingHtml, /待确认/);

  const yesterday = new Date();
  yesterday.setDate(yesterday.getDate() - 1);
  yesterday.setHours(12, 0, 0, 0);
  const localizedConversation = {
    ...conversations[0],
    last_activity: yesterday.toISOString(),
  };
  const englishItems = buildConversationItems({
    agents,
    conversations: [localizedConversation],
    locale: "en",
    rooms: [rooms[0]],
    untitledRoomLabel: "Untitled room",
  });
  const chineseItems = buildConversationItems({
    agents,
    conversations: [localizedConversation],
    locale: "zh",
    rooms: [rooms[0]],
    untitledRoomLabel: "未命名 Room",
  });
  assert.equal(englishItems[0].timeLabel, "Yesterday");
  assert.equal(chineseItems[0].timeLabel, "昨天");
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

test("Room memory saved status is projected only into its Agent Thread", async () => {
  const { projectGroupAgentTimeline } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const { getRoomThreadMessages } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-thread-model.ts",
  );
  const { buildSystemEventBlocks } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-system-events.ts",
  );
  const saved = {
    agent_id: "367448a0264b",
    content: "长期记忆已保存",
    message_id: "memory-saved-amy",
    metadata: { subtype: "memory_saved" },
    role: "system",
    timestamp: 1784083437370,
  };

  assert.deepEqual(
    getRoomThreadMessages([saved], "367448a0264b")
      .map((message) => message.message_id),
    [saved.message_id],
  );
  assert.deepEqual(
    getRoomThreadMessages([saved], "0ed5434a8c13"),
    [],
  );
  assert.deepEqual(
    buildSystemEventBlocks([saved], false).map((block) => ({
      content: block.content,
      label: block.label,
    })),
    [{ content: saved.content, label: "长期记忆" }],
  );
  const publicTimeline = projectGroupAgentTimeline({
    messageGroups: new Map([["round-memory", [saved]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["round-memory"],
  });
  assert.deepEqual(
    publicTimeline.roundIds.flatMap(
      (roundId) => publicTimeline.messageGroups.get(roundId) ?? [],
    ),
    [],
    "长期记忆过程不能在 Room 公区生成独立消息节点",
  );
});

test("Assistant memory references are deduplicated for the footer", async () => {
  const { collectRecalledMemoryReferences } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/use-message-item-projection.ts",
  );
  const reference = {
    description: "发布前检查签名清单",
    name: "release checklist",
  };
  assert.deepEqual(
    collectRecalledMemoryReferences([
      { recalled_memories: [reference] },
      { recalled_memories: [reference, { description: "", name: "empty" }] },
    ]),
    [reference],
  );
});

test("Loaded Assistant memory references replace an equivalent live snapshot", async () => {
  const { mergeLoadedMessages } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );
  const live = {
    agent_id: "367448a0264b",
    content: [{ type: "text", text: "发布检查已完成。" }],
    is_complete: true,
    message_id: "assistant-memory-reference",
    role: "assistant",
    round_id: "round-memory-reference",
    session_key: "agent:test",
    stop_reason: "end_turn",
    timestamp: 1784083437370,
  };
  const reference = {
    description: "发布前检查签名清单",
    name: "release checklist",
  };
  const merged = mergeLoadedMessages(
    [{ ...live, recalled_memories: [reference] }],
    [live],
  );
  assert.deepEqual(merged[0].recalled_memories, [reference]);
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

test("Goal 完成收据只展示已知结算项且不泄露内部绑定 ID", async () => {
  const { AssistantMessageStats } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-message-stats.tsx",
  );
  const { formatGoalElapsed } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/goal-completion-receipt.ts",
  );
  const renderReceipt = async (receipt, locale = "zh") => renderWithI18n(
    React.createElement(AssistantMessageStats, {
      copied: false,
      goalCompletionReceipt: receipt,
      stats: null,
      streaming: false,
    }),
    locale,
  );

  const completeOnly = await renderReceipt({
    goal_id: "goal-hidden",
    round_id: "round-hidden",
  });
  assert.match(completeOnly, /Goal 已完成/);
  assert.doesNotMatch(completeOnly, /耗时|tokens|goal-hidden|round-hidden|结算中|不可用/);

  const durationOnly = await renderReceipt({
    goal_id: "goal-hidden",
    round_id: "round-hidden",
    time_used_seconds: 754,
  });
  assert.match(durationOnly, /Goal 已完成/);
  assert.match(durationOnly, /耗时 12 分 34 秒/);
  assert.doesNotMatch(durationOnly, /tokens|结算中|不可用/);

  const complete = await renderReceipt({
    goal_id: "goal-hidden",
    round_id: "round-hidden",
    time_used_seconds: 754,
    actual_tokens: 62762,
  });
  assert.match(complete, /Goal 已完成/);
  assert.match(complete, /耗时 12 分 34 秒/);
  assert.match(complete, /使用 62,762 tokens/);
  assert.doesNotMatch(complete, /goal-hidden|round-hidden|结算中|不可用/);
  assert.equal(formatGoalElapsed(3605, "zh"), "1 小时 0 分");
  assert.equal(formatGoalElapsed(86400, "zh"), "1 天 0 小时");
});

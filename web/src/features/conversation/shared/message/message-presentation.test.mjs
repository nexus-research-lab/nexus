// @vitest-environment node

import { renderWithI18n, assistantMessage } from "@/features/conversation/testing/message-fixtures.mjs";

import assert from "node:assert/strict";
import { test } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

test("recoverable malformed tool results stay out of the user timeline", async () => {
  const {
    buildVisibleOrderedAssistantEntries,
    collectHiddenToolUseIds,
  } = await import("@/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts");
  const content = [
    {
      type: "tool_use",
      id: "tool-malformed",
      name: "WebFetch",
      input: {},
      metadata: {
        _nexus_internal_kind: "malformed_tool_input",
      },
    },
    {
      type: "tool_result",
      tool_use_id: "tool-malformed",
      content: "Tool input was not valid JSON",
      is_error: true,
      metadata: {
        _nexus_internal_kind: "malformed_tool_input",
      },
    },
    { type: "text", text: "模型已自行修正并继续。" },
  ];
  const hiddenToolUseIds = collectHiddenToolUseIds(content, new Set());
  const entries = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds,
    isLoading: false,
    mergedContent: content,
    mergedContentSourceMessageIds: ["assistant-1", "assistant-2", "assistant-3"],
    sourceMessageOrderById: new Map([
      ["assistant-1", 0],
      ["assistant-2", 1],
      ["assistant-3", 2],
    ]),
    systemEventBlocks: [],
  });

  assert.deepEqual([...hiddenToolUseIds], ["tool-malformed"]);
  assert.deepEqual(
    entries.map(({ block }) => block),
    [{ type: "text", text: "模型已自行修正并继续。" }],
  );
});

test("a visible ToolBlock exclusively owns its running state", async () => {
  const { ContentRenderer } = await import("@/features/conversation/shared/message/item/view/content/content-renderer.tsx");
  const { resolveContentActivityState } = await import("@/features/conversation/shared/message/item/activity/message-content-activity.ts");
  const toolUse = {
    type: "tool_use",
    id: "tool-write-state-owner",
    name: "Write",
    input: { file_path: "PLAN.md" },
  };
  assert.equal(resolveContentActivityState({
    consumedBlockIndexes: new Set(),
    content: [toolUse],
    hiddenToolNames: new Set(),
    resolvedToolUseIds: new Set(),
  }), null);

  const html = await renderWithI18n(React.createElement(ContentRenderer, {
    content: [toolUse],
    fallbackActivityState: "executing",
    isStreaming: true,
  }));
  assert.equal(html.match(/>执行中</g)?.length, 1);
  assert.doesNotMatch(html, /正在执行/);
});

test("live conversations keep one stable tool segment across consecutive patches", async () => {
  const { projectToolRunSegments } = await import("@/features/conversation/shared/message/item/process/dm-tool-run-segments.ts");
  const toolA = {
    type: "tool_use",
    id: "tool-run-a",
    name: "Read",
    input: { file_path: "a.ts" },
  };
  const resultA = {
    type: "tool_result",
    tool_use_id: toolA.id,
    content: "a",
  };
  const toolB = {
    type: "tool_use",
    id: "tool-run-b",
    name: "Grep",
    input: { pattern: "needle" },
  };
  const resultB = {
    type: "tool_result",
    tool_use_id: toolB.id,
    content: "b",
  };
  const project = (
    content,
    { live = true, responseResumed = false } = {},
  ) => projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live,
    projection: { content, streamingIndexes: new Set() },
    responseResumed,
    toolUseSummary: null,
  });

  for (const content of [
    [toolA],
    [toolA, resultA],
    [toolA, resultA, toolB],
    [toolA, resultA, toolB, resultB],
  ]) {
    const [segment] = project(content);
    assert.equal(segment.kind, "tool_run");
    assert.equal(segment.id, "tool-run:tool-run-a");
    assert.equal(segment.phase, "active");
  }

  const [completed] = project(
    [toolA, resultA, toolB, resultB],
    { responseResumed: true },
  );
  assert.equal(completed.id, "tool-run:tool-run-a");
  assert.equal(completed.phase, "complete");
  assert.equal(completed.toolUseIds.length, 2);

  const [unresolvedDuringResponse] = project(
    [toolA],
    { responseResumed: true },
  );
  assert.equal(
    unresolvedDuringResponse.phase,
    "active",
    "a stale response boundary cannot collapse an unresolved newer tool",
  );

  const [terminal] = project(
    [toolA, resultA, toolB, resultB],
    { live: false },
  );
  assert.equal(terminal.phase, "complete");
});

test("tool segments absorb completed questions but preserve active interactions", async () => {
  const { projectToolRunSegments } = await import("@/features/conversation/shared/message/item/process/dm-tool-run-segments.ts");
  const failedTool = {
    type: "tool_use",
    id: "tool-failed",
    name: "Bash",
    input: { command: "false" },
  };
  const questionTool = {
    type: "tool_use",
    id: "tool-question-boundary",
    name: "AskUserQuestion",
    input: { questions: [] },
  };
  const permissionTool = {
    type: "tool_use",
    id: "tool-permission-boundary",
    name: "Write",
    input: { file_path: "answer.md" },
  };
  const trailingTool = {
    type: "tool_use",
    id: "tool-after-boundaries",
    name: "Read",
    input: { file_path: "answer.md" },
  };
  const content = [
    failedTool,
    {
      type: "tool_result",
      tool_use_id: failedTool.id,
      content: "exit 1",
      is_error: true,
    },
    {
      type: "task_progress",
      task_id: "task-failed",
      description: "command finished",
      tool_use_id: failedTool.id,
    },
    {
      type: "system_event",
      content: "retry exhausted",
      icon: "retry",
      label: "Retry",
      source_message_id: "system-failed",
      subtype: "api_retry",
      timestamp: 3,
      tone: "warning",
      tool_use_id: failedTool.id,
    },
    {
      type: "workspace_file_artifact",
      path: "failure.log",
      source_tool_use_id: failedTool.id,
    },
    { type: "thinking", thinking: "调整方案" },
    questionTool,
    {
      type: "tool_result",
      tool_use_id: questionTool.id,
      content: "answered",
    },
    permissionTool,
    {
      type: "tool_result",
      tool_use_id: permissionTool.id,
      content: "allowed",
    },
    trailingTool,
  ];
  const [activeFailedSegment] = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: content.slice(0, 2),
      streamingIndexes: new Set(),
    },
    responseResumed: false,
    toolUseSummary: null,
  });
  assert.equal(activeFailedSegment.phase, "active");
  assert.equal(activeFailedSegment.errorCount, 1);
  const segments = projectToolRunSegments({
    interactiveToolUseIds: new Set([permissionTool.id]),
    live: true,
    projection: {
      content,
      streamingIndexes: new Set([5]),
    },
    responseResumed: false,
    toolUseSummary: null,
  });

  assert.deepEqual(
    segments.map(({ id, kind, phase }) => ({ id, kind, phase })),
    [
      {
        id: "tool-run:tool-failed",
        kind: "tool_run",
        phase: "error",
      },
      {
        id: "interactive-tool:tool-permission-boundary",
        kind: "content",
        phase: undefined,
      },
      {
        id: "tool-run:tool-after-boundaries",
        kind: "tool_run",
        phase: "active",
      },
    ],
  );
  const failedSegment = segments[0];
  assert.equal(failedSegment.errorCount, 1);
  assert.deepEqual(
    failedSegment.projection.content.map(({ type }) => type),
    [
      "tool_use",
      "tool_result",
      "task_progress",
      "system_event",
      "workspace_file_artifact",
      "thinking",
      "tool_use",
      "tool_result",
    ],
  );
  assert.deepEqual(
    [...failedSegment.projection.streamingIndexes],
    [5],
    "folded execution preserves the remapped streaming reasoning index",
  );
  assert.deepEqual(
    segments[1].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
    "a pending permission tool stays outside the collapsible run",
  );
});

test("ToolUseSummary updates the matching uninterrupted tool row and text stays visible", async () => {
  const { projectToolRunSegments } = await import("@/features/conversation/shared/message/item/process/dm-tool-run-segments.ts");
  const toolA = {
    type: "tool_use",
    id: "tool-summary-a",
    name: "Read",
    input: { file_path: "a.ts" },
  };
  const toolB = {
    type: "tool_use",
    id: "tool-summary-b",
    name: "Grep",
    input: { pattern: "needle" },
  };
  const segments = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: [
        { type: "thinking", thinking: "先看现有实现" },
        toolA,
        { type: "tool_result", tool_use_id: toolA.id, content: "a" },
        { type: "thinking", thinking: "继续核对调用点" },
        toolB,
        { type: "tool_result", tool_use_id: toolB.id, content: "b" },
      ],
      streamingIndexes: new Set(),
    },
    responseResumed: false,
    toolUseSummary: {
      precedingToolUseIds: [toolB.id],
      text: "定位时间线折叠入口",
    },
  });

  assert.deepEqual(
    segments.map(({ phase, summaryText, toolUseIds }) => ({
      phase,
      summaryText,
      toolUseIds,
    })),
    [
      {
        phase: "active",
        summaryText: "定位时间线折叠入口",
        toolUseIds: [toolA.id, toolB.id],
      },
    ],
  );
  assert.deepEqual(
    segments[0].projection.content.map(({ type }) => type),
    [
      "thinking",
      "tool_use",
      "tool_result",
      "thinking",
      "tool_use",
      "tool_result",
    ],
  );

  const commentarySegments = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: {
      content: [
        toolA,
        { type: "tool_result", tool_use_id: toolA.id, content: "a" },
        { type: "text", text: "这里是给用户看的阶段结论" },
        toolB,
        { type: "tool_result", tool_use_id: toolB.id, content: "b" },
      ],
      streamingIndexes: new Set(),
    },
    responseResumed: false,
    toolUseSummary: {
      precedingToolUseIds: [toolB.id],
      text: "定位时间线折叠入口",
    },
  });
  assert.deepEqual(
    commentarySegments.map((segment) => ({
      kind: segment.kind,
      summaryText: segment.kind === "tool_run" ? segment.summaryText : null,
      toolUseIds: segment.kind === "tool_run" ? segment.toolUseIds : [],
    })),
    [
      {
        kind: "tool_run",
        summaryText: null,
        toolUseIds: [toolA.id],
      },
      {
        kind: "content",
        summaryText: null,
        toolUseIds: [],
      },
      {
        kind: "tool_run",
        summaryText: "定位时间线折叠入口",
        toolUseIds: [toolB.id],
      },
    ],
  );
  assert.deepEqual(
    commentarySegments[0].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
  );
  assert.deepEqual(
    commentarySegments[1].projection.content.map(({ type }) => type),
    ["text"],
    "visible narration remains in the reading flow",
  );
  assert.deepEqual(
    commentarySegments[2].projection.content.map(({ type }) => type),
    ["tool_use", "tool_result"],
  );
});

test("live commentary separates completed and active tool groups", async () => {
  const { AssistantMessageContent } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-message-content.tsx");
  const { resolveMessageItemFinalProjection } = await import("@/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts");
  const {
    projectionFromOrderedEntries,
  } = await import("@/features/conversation/shared/message/item/message-item-projection.ts");
  const { projectToolRunSegments } = await import("@/features/conversation/shared/message/item/process/dm-tool-run-segments.ts");
  const initialThinking = { type: "thinking", thinking: "先查基础数据" };
  const initialText = { type: "text", text: "EARLIER_REPLY_MARKER" };
  const oldTool = {
    type: "tool_use",
    id: "tool-before-reply",
    name: "WebSearch",
    input: { url: "https://old.example.test" },
  };
  const oldResult = {
    type: "tool_result",
    tool_use_id: oldTool.id,
    content: "ok",
  };
  const nextThinking = { type: "thinking", thinking: "整理后补官方口径" };
  const commentary = { type: "text", text: "LATEST_REPLY_MARKER" };
  const activeTool = {
    type: "tool_use",
    id: "tool-after-reply",
    name: "WebFetch",
    input: { url: "https://latest.example.test" },
  };
  const firstMessage = {
    role: "assistant",
    message_id: "assistant-before-reply",
    parent_id: "user-boundary",
    content: [initialThinking, initialText, oldTool],
  };
  const activeMessage = {
    role: "assistant",
    message_id: "assistant-after-reply",
    parent_id: "user-boundary",
    content: [nextThinking, commentary, activeTool],
  };
  const orderedEntries = [
    ...firstMessage.content.map((block) => ({
      block,
      sourceMessageId: firstMessage.message_id,
      sourceOrder: 0,
    })),
    {
      block: oldResult,
      sourceMessageId: firstMessage.message_id,
      sourceOrder: 0,
    },
    ...activeMessage.content.map((block) => ({
      block,
      sourceMessageId: activeMessage.message_id,
      sourceOrder: 1,
    })),
  ].map((entry, mergedIndex) => ({ ...entry, mergedIndex }));
  const streamingBlockIndexes = new Set([orderedEntries.length - 1]);
  const projection = resolveMessageItemFinalProjection({
    assistantContentMode: "dm_live",
    assistantMessages: [firstMessage, activeMessage],
    orderedProjection: projectionFromOrderedEntries(
      orderedEntries,
      streamingBlockIndexes,
    ),
    resultSummary: undefined,
    roundId: "round-boundary",
    userMessageId: "user-boundary",
    streamingBlockIndexes,
    visibleAssistantTurns: [
      {
        content: [...firstMessage.content, oldResult],
        messageId: firstMessage.message_id,
        streamingIndexes: new Set(),
        textContent: [initialText],
        textStreamingIndexes: new Set(),
      },
      {
        content: activeMessage.content,
        messageId: activeMessage.message_id,
        streamingIndexes: new Set([2]),
        textContent: [commentary],
        textStreamingIndexes: new Set(),
      },
    ],
    visibleOrderedAssistantEntries: orderedEntries,
  });
  assert.equal(
    projection.finalAssistantContent,
    null,
    "commentary followed by a normal tool remains in the direct timeline",
  );
  const segments = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: projection.directOrderedProjection,
    responseResumed: false,
    toolUseSummary: null,
  });
  assert.deepEqual(
    segments.flatMap((segment) => (
      segment.kind === "tool_run" ? [segment.toolUseIds] : []
    )),
    [[oldTool.id], [activeTool.id]],
  );
  const html = await renderWithI18n(React.createElement(
    AssistantMessageContent,
    {
      activity: {
        emptyStreamStatus: null,
        label: null,
        showCursor: true,
        standalone: false,
        state: "browsing",
        toolUseSummary: null,
      },
      direct: {
        projection: projection.directOrderedProjection,
        visible: true,
      },
      environment: {
        canRespondToPermissions: true,
        hiddenToolNames: [],
        mode: "dm_live",
      },
      final: {
        content: projection.finalAssistantContent,
        mentions: [],
        isStreaming: false,
        streamingIndexes: projection.finalAssistantStreamingIndexes,
        visible: false,
      },
      permissions: {
        all: [],
        matchedByToolUseId: new Map(),
        owner: "composer",
        unmatched: [],
      },
      process: {
        anchorRef: { current: null },
        expanded: false,
        projection: { content: [], streamingIndexes: new Set() },
        summary: { kind: "details", latestDetail: null, metrics: [] },
        toggle: () => {},
        visible: false,
      },
      showMaxTokensWarning: false,
    },
  ));

  assert.ok(
    html.indexOf('data-tool-run-id="tool-run:tool-before-reply"')
      < html.indexOf("LATEST_REPLY_MARKER"),
  );
  assert.ok(html.indexOf("LATEST_REPLY_MARKER") < html.indexOf("latest.example.test"));
  assert.equal(html.match(/latest\.example\.test/g)?.length, 1);
  assert.match(html, /data-live-tool-text="true"/);
});

test("DM and Room Thread activity groups default to collapsed", async () => {
  const { AssistantProcessCallchain } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-process-callchain.tsx");
  const { AssistantToolRuns } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx");
  const content = Array.from({ length: 4 }, (_, index) => [
    { type: "thinking", thinking: `Thought ${index}` },
    {
      type: "tool_use",
      id: `tool-icon-${index}`,
      name: index % 2 === 0 ? "Bash" : "Read",
      input: {},
    },
    {
      type: "tool_result",
      tool_use_id: `tool-icon-${index}`,
      content: "ok",
    },
  ]).flat();
  const toolRunProps = {
    activity: {
      emptyStreamStatus: null,
      label: null,
      showCursor: false,
      standalone: false,
      state: "executing",
      toolUseSummary: null,
    },
    environment: {
      canRespondToPermissions: true,
      hiddenToolNames: [],
      mode: "dm_live",
    },
    generatedFilesLabel: "生成文件",
    permissions: {
      all: [],
      matchedByToolUseId: new Map(),
      owner: "content",
      unmatched: [],
    },
    projection: { content, streamingIndexes: new Set() },
    responseResumed: true,
  };
  const toolRunHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    toolRunProps,
  ));

  assert.match(toolRunHtml, /aria-expanded="false"/);
  assert.match(toolRunHtml, /data-process-activity-icon-count="8"/);
  assert.match(toolRunHtml, />\+2<\/span>/);
  assert.equal(
    toolRunHtml.match(/data-process-activity-icon=/g)?.length,
    6,
  );
  assert.match(toolRunHtml, /data-process-activity-icon="thinking"/);
  assert.match(toolRunHtml, /data-process-activity-icon="terminal"/);
  assert.match(toolRunHtml, /data-process-activity-icon="inspect"/);
  assert.ok(
    toolRunHtml.lastIndexOf("+2")
      > toolRunHtml.lastIndexOf("data-process-activity-icon="),
    "the overflow count follows the visible activity icons",
  );

  assert.doesNotMatch(toolRunHtml, /Thought 0|data-tool-run-detail-list/);
  assert.match(toolRunHtml, /before:bottom-0/);
  assert.match(toolRunHtml, /data-timeline-dot/);
  assert.match(toolRunHtml, /nexus-chat-timeline-block/);
  assert.doesNotMatch(toolRunHtml, /正在执行/);

  const liveToolRunHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    {
      ...toolRunProps,
      activity: {
        ...toolRunProps.activity,
        showCursor: true,
      },
      responseResumed: false,
    },
  ));
  assert.match(liveToolRunHtml, /aria-expanded="false"/);
  assert.match(liveToolRunHtml, /正在执行/);
  assert.match(liveToolRunHtml, /data-loading-orb="active"/);

  const threadHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    {
      ...toolRunProps,
      environment: {
        ...toolRunProps.environment,
        mode: "room_thread_process",
      },
    },
  ));
  assert.doesNotMatch(threadHtml, /aria-expanded="true"/);
  assert.match(threadHtml, /aria-expanded="false"/);
  assert.doesNotMatch(threadHtml, /data-tool-run-detail-list/);
  assert.doesNotMatch(threadHtml, /Thought 0/);
  assert.doesNotMatch(threadHtml, /before:bottom-0/);
  assert.doesNotMatch(threadHtml, /data-timeline-dot/);
  assert.doesNotMatch(threadHtml, /nexus-chat-timeline-block/);

  const liveThreadHtml = await renderWithI18n(React.createElement(
    AssistantToolRuns,
    {
      ...toolRunProps,
      activity: {
        ...toolRunProps.activity,
        showCursor: true,
      },
      environment: {
        ...toolRunProps.environment,
        mode: "room_thread_process",
      },
      responseResumed: false,
    },
  ));
  assert.doesNotMatch(liveThreadHtml, /data-tool-run-detail-list/);

  const archivedHtml = await renderWithI18n(React.createElement(
    AssistantProcessCallchain,
    {
      activity: toolRunProps.activity,
      environment: {
        ...toolRunProps.environment,
        mode: "dm_archived",
      },
      generatedFilesLabel: toolRunProps.generatedFilesLabel,
      permissions: toolRunProps.permissions,
      process: {
        anchorRef: { current: null },
        expanded: true,
        projection: toolRunProps.projection,
        summary: {
          kind: "details",
          latestDetail: null,
          metrics: [
            { count: 4, kind: "thinking" },
            { count: 5, kind: "action" },
          ],
        },
        toggle: () => {},
        visible: true,
      },
    },
  ));
  assert.match(archivedHtml, /data-tool-run-id=/);
  assert.match(archivedHtml, /aria-expanded="false"/);
  assert.match(archivedHtml, /4 段思路 · 5 次动作/);
  assert.doesNotMatch(archivedHtml, /Thought 0/);
});

test("detail scroll fade follows the remaining scroll directions", async () => {
  const { MessageDetailScroll } = await import("@/features/conversation/shared/message/ui/message-rail.tsx");
  const { isAtScrollBottom, resolveScrollFade } = await import("@/features/conversation/shared/timeline/scroll/follow-scroll-model.ts");
  const position = (scrollTop) => ({
    clientHeight: 280,
    scrollHeight: 560,
    scrollTop,
  });

  assert.equal(resolveScrollFade({
    clientHeight: 280,
    scrollHeight: 280,
    scrollTop: 0,
  }), "none");
  assert.equal(resolveScrollFade(position(0)), "bottom");
  assert.equal(resolveScrollFade(position(140)), "both");
  assert.equal(resolveScrollFade(position(280)), "top");
  assert.equal(isAtScrollBottom(position(140)), false);
  assert.equal(isAtScrollBottom(position(280)), true);

  const nestedHtml = renderToStaticMarkup(React.createElement(
    MessageDetailScroll,
    null,
    React.createElement(
      MessageDetailScroll,
      null,
      "Nested detail",
    ),
  ));
  assert.equal(
    nestedHtml.match(/data-message-detail-scroll/g)?.length,
    1,
    "nested details share the outer scroll owner",
  );
});

test("Thought expansion shows content once and collapse restores its preview", async () => {
  const { ThinkingBlock } = await import("@/features/conversation/shared/message/blocks/thinking-block.tsx");
  const html = await renderWithI18n(React.createElement(ThinkingBlock, {
    isStreaming: true,
    defaultExpanded: true,
    thinking: "Compact detail",
  }));

  assert.match(html, /data-message-detail-sticky-header="true"/);
  assert.match(html, /data-message-detail-follow="true"/);
  assert.match(html, /data-markdown-streaming="true"/);
  assert.doesNotMatch(
    html,
    /data-thinking-block-preview/,
    "expanded Thought keeps its complete content only in the detail body",
  );
  assert.equal(html.match(/Compact detail/g)?.length, 1);

  const collapsedHtml = await renderWithI18n(React.createElement(ThinkingBlock, {
    thinking: "Collapsed preview",
  }));
  assert.match(collapsedHtml, /data-thinking-block-preview/);
  assert.match(collapsedHtml, /Collapsed preview/);
});

test("expanded tool headers do not repeat summaries from their detail body", async () => {
  const { buildToolBlockViewModel } = await import("@/features/conversation/shared/message/blocks/tool/tool-block-model.ts");
  const { ToolBlockHeader } = await import("@/features/conversation/shared/message/blocks/tool/header/tool-block-header.tsx");
  const localization = {
    locale: "en",
    t: (key) => key,
  };
  const launchResult = "Launching skill: execution-orchestrator";
  const skillModel = buildToolBlockViewModel({
    localization,
    status: "success",
    toolResult: {
      content: launchResult,
      tool_use_id: "tool-skill",
      type: "tool_result",
    },
    toolUse: {
      id: "tool-skill",
      input: { skill: "execution-orchestrator" },
      name: "Skill",
      type: "tool_use",
    },
  });
  assert.equal(skillModel.collapsedDetailText, launchResult);
  assert.equal(skillModel.expandedInputText, null);

  const renderHeader = (isExpanded, model = skillModel) => (
    renderWithI18n(React.createElement(ToolBlockHeader, {
      copied: false,
      interactionDisabled: false,
      isExpanded,
      model,
      onCopyResult: () => {},
      onToggle: () => {},
    }))
  );
  const collapsedHtml = await renderHeader(false);
  const expandedHtml = await renderHeader(true);
  assert.match(collapsedHtml, /data-tool-block-detail="inline"/);
  assert.match(collapsedHtml, /Launching skill: execution-orchestrator/);
  assert.doesNotMatch(expandedHtml, /data-tool-block-detail="inline"/);
  assert.doesNotMatch(expandedHtml, /Launching skill: execution-orchestrator/);

  const queryModel = buildToolBlockViewModel({
    localization,
    status: "success",
    toolResult: {
      content: "Search complete",
      tool_use_id: "tool-search",
      type: "tool_result",
    },
    toolUse: {
      id: "tool-search",
      input: { query: "A2A protocol" },
      name: "WebSearch",
      type: "tool_use",
    },
  });
  assert.equal(queryModel.collapsedDetailText, "A2A protocol");
  assert.equal(queryModel.expandedInputText, "A2A protocol");
  assert.doesNotMatch(await renderHeader(true, queryModel), /A2A protocol/);

  const duplicateModel = buildToolBlockViewModel({
    localization,
    status: "success",
    toolResult: {
      content: "Same input and result",
      tool_use_id: "tool-echo",
      type: "tool_result",
    },
    toolUse: {
      id: "tool-echo",
      input: { query: "Same input and result" },
      name: "WebSearch",
      type: "tool_use",
    },
  });
  assert.equal(duplicateModel.expandedInputText, null);

  const operationIdentityModel = buildToolBlockViewModel({
    localization,
    status: "success",
    toolUse: {
      id: "tool-workgraph-preview",
      input: {
        action: "invoke",
        domain: "execution",
        operation: "extract_workgraph_preview",
      },
      name: "mcp__nexus__command",
      type: "tool_use",
    },
  });
  assert.equal(operationIdentityModel.toolTitle, "extract_workgraph_preview");
  assert.equal(operationIdentityModel.collapsedDetailText, null);
  assert.equal(operationIdentityModel.expandedInputText, null);
});

test("a newer semantic block closes the preceding smooth stream", async () => {
  const { findLastStreamableBlockIndex } = await import("@/hooks/conversation/use-assistant-content-merge.ts");
  const thinking = { type: "thinking", thinking: "先确认范围" };

  assert.equal(findLastStreamableBlockIndex([thinking]), 0);
  assert.equal(
    findLastStreamableBlockIndex([
      thinking,
      { type: "tool_use", id: "tool-after-thinking", name: "Bash", input: {} },
    ]),
    -1,
  );
});

test("semantic tool rejection stays distinct from transport completion in DM and Room", async () => {
  const { AssistantToolRuns } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx");
  const { ContentRenderer } = await import("@/features/conversation/shared/message/item/view/content/content-renderer.tsx");
  const { resolveToolBlockStatus } = await import("@/features/conversation/shared/message/item/view/content/content-renderer-model.ts");
  const { ToolBlockResult } = await import("@/features/conversation/shared/message/blocks/tool/tool-block-detail.tsx");
  const { projectToolRunSegments } = await import("@/features/conversation/shared/message/item/process/dm-tool-run-segments.ts");
  const { buildProcessSummary } = await import("@/features/conversation/shared/message/item/process/message-process-summary.ts");
  const interrupted = {type: "tool_result", tool_use_id: "cancel-tool", is_error: true, content: "Interrupted by user"};
  const cancelledContent = [{type: "tool_use", id: "cancel-tool", name: "Bash", input: {}}, interrupted];
  const [cancelledSegment] = projectToolRunSegments({interactiveToolUseIds: new Set(), live: false, projection: {content: cancelledContent, streamingIndexes: new Set()}, responseResumed: true, toolUseSummary: null});
  assert.equal(cancelledSegment.phase, "stopped");
  assert.equal(cancelledSegment.errorCount, 0);
  assert.deepEqual(buildProcessSummary({pendingPermissionCount: 0, processContent: cancelledContent}).metrics, [{kind: "action", count: 1}]);
  const { I18nProvider } = await import("@/shared/i18n/i18n-provider.tsx");
  const tool = {
    type: "tool_use",
    id: "tool-plan-rejected",
    name: "Bash",
    input: {
      command: '"${NEXUS_COMMAND_PATH}" --json execution invoke --operation prepare_plan_execution --request-id plan-rejected-1',
    },
  };
  const result = {
    type: "tool_result",
    tool_use_id: tool.id,
    is_error: false,
    content: JSON.stringify({
      domain: "execution",
      action: "invoke",
      operation: "prepare_plan_execution",
      result: { data: {
        message: "Plan Document items must contain at least one complete Work Item",
        next_actions: [{
          domain: "execution",
          operation: "prepare_plan_execution",
          reason: "submit one complete Nexus Plan Document with every intended Work Item",
        }],
        outcome: "rejected",
        reason_code: "plan_items_empty",
      } },
    }),
  };
  const projection = {
    content: [tool, result],
    streamingIndexes: new Set(),
  };
  const [segment] = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection,
    responseResumed: true,
    toolUseSummary: null,
  });
  assert.equal(segment.phase, "rejected");
  assert.equal(segment.rejectedCount, 1);
  assert.equal(segment.errorCount, 0);
  assert.equal(
    resolveToolBlockStatus({ result }, false),
    "rejected",
    "a completed transport must not turn a rejected mutation green",
  );
  assert.deepEqual(
    buildProcessSummary({
      pendingPermissionCount: 0,
      processContent: [tool, result],
    }).metrics,
    [
      { count: 1, kind: "action" },
      { count: 1, kind: "error" },
    ],
  );

  const provider = (child) => React.createElement(I18nProvider, null, child);
  const dmHtml = renderToStaticMarkup(provider(React.createElement(
    AssistantToolRuns,
    {
      activity: {
        emptyStreamStatus: null,
        label: null,
        showCursor: true,
        standalone: false,
        state: "executing",
        toolUseSummary: null,
      },
      environment: {
        canRespondToPermissions: true,
        hiddenToolNames: [],
        mode: "dm_live",
      },
      generatedFilesLabel: "生成文件",
      permissions: {
        all: [],
        matchedByToolUseId: new Map(),
        owner: "content",
        unmatched: [],
      },
      projection,
      responseResumed: true,
    },
  )));
  assert.match(dmHtml, /已拒绝/);
  assert.doesNotMatch(dmHtml, />完成</);

  const roomHtml = renderToStaticMarkup(provider(React.createElement(
    ContentRenderer,
    { content: [tool, result] },
  )));
  assert.match(roomHtml, /已拒绝/);
  assert.match(roomHtml, /Plan Document items/);
  assert.doesNotMatch(roomHtml, /next_actions/);

  const detailHtml = renderToStaticMarkup(provider(React.createElement(
    ToolBlockResult,
    { toolResult: result },
  )));
  assert.match(detailHtml, /data-tool-result-semantic-outcome="rejected"/);
  assert.match(detailHtml, /data-inline-notice-tone="danger"/);
  assert.match(detailHtml, /data-inline-notice-variant="contained"/);
  assert.match(detailHtml, /data-inline-notice-width="compact"/);
  assert.match(detailHtml, /max-w-sm/);
  assert.match(detailHtml, /Plan Document items/);
  assert.match(detailHtml, /plan_items_empty/);
  assert.doesNotMatch(detailHtml, /next_actions/);
});

test("superseded WorkGraph result is muted and does not count as failure", async () => {
  const { AssistantToolRuns } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx");
  const { ContentRenderer } = await import("@/features/conversation/shared/message/item/view/content/content-renderer.tsx");
  const { resolveToolBlockStatus } = await import("@/features/conversation/shared/message/item/view/content/content-renderer-model.ts");
  const { ToolBlockResult } = await import("@/features/conversation/shared/message/blocks/tool/tool-block-detail.tsx");
  const { projectToolRunSegments } = await import("@/features/conversation/shared/message/item/process/dm-tool-run-segments.ts");
  const { buildProcessSummary } = await import("@/features/conversation/shared/message/item/process/message-process-summary.ts");
  const { I18nProvider } = await import("@/shared/i18n/i18n-provider.tsx");
  const tool = {
    type: "tool_use",
    id: "tool-submit-superseded",
    name: "Bash",
    input: {
      command: '"${NEXUS_COMMAND_PATH}" --json execution invoke --operation submit_work --request-id submit-superseded-1',
    },
  };
  const result = {
    type: "tool_result",
    tool_use_id: tool.id,
    is_error: false,
    content: JSON.stringify({
      domain: "execution",
      action: "invoke",
      operation: "submit_work",
      result: { data: {
        message: "旧工作已被新目标替换；请停止当前轮次并等待新指派",
        outcome: "superseded",
        reason_code: "execution_terminal",
      } },
    }),
  };
  const projection = {
    content: [tool, result],
    streamingIndexes: new Set(),
  };
  const [segment] = projectToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection,
    responseResumed: true,
    toolUseSummary: null,
  });
  assert.equal(segment.phase, "superseded");
  assert.equal(segment.supersededCount, 1);
  assert.equal(segment.rejectedCount, 0);
  assert.equal(segment.errorCount, 0);
  assert.equal(resolveToolBlockStatus({ result }, false), "superseded");
  assert.deepEqual(
    buildProcessSummary({
      pendingPermissionCount: 0,
      processContent: [tool, result],
    }).metrics,
    [{ count: 1, kind: "action" }],
  );

  const provider = (child) => React.createElement(I18nProvider, null, child);
  const dmHtml = renderToStaticMarkup(provider(React.createElement(
    AssistantToolRuns,
    {
      activity: {
        emptyStreamStatus: null,
        label: null,
        showCursor: true,
        standalone: false,
        state: "executing",
        toolUseSummary: null,
      },
      environment: {
        canRespondToPermissions: true,
        hiddenToolNames: [],
        mode: "dm_live",
      },
      generatedFilesLabel: "生成文件",
      permissions: {
        all: [],
        matchedByToolUseId: new Map(),
        owner: "content",
        unmatched: [],
      },
      projection,
      responseResumed: true,
    },
  )));
  assert.match(dmHtml, /已被替换/);
  assert.doesNotMatch(dmHtml, /执行失败|已拒绝/);

  const roomHtml = renderToStaticMarkup(provider(React.createElement(
    ContentRenderer,
    { content: [tool, result] },
  )));
  assert.match(roomHtml, /已被替换/);
  assert.match(roomHtml, /旧工作已被新目标替换/);
  assert.doesNotMatch(roomHtml, /失败|已拒绝/);

  const detailHtml = renderToStaticMarkup(provider(React.createElement(
    ToolBlockResult,
    { toolResult: result },
  )));
  assert.match(detailHtml, /data-tool-result-semantic-outcome="superseded"/);
  assert.match(detailHtml, /data-inline-notice-tone="neutral"/);
  assert.match(detailHtml, /data-inline-notice-variant="contained"/);
  assert.match(detailHtml, /data-inline-notice-width="compact"/);
  assert.match(detailHtml, /max-w-sm/);
  assert.match(detailHtml, /execution_terminal/);
});

test("DM live and terminal keep the final response on one content surface", async () => {
  const {
    projectionFromOrderedEntries,
    resolveAssistantResponseSurface,
  } = await import("@/features/conversation/shared/message/item/message-item-projection.ts");
  const { resolveMessageItemFinalProjection } = await import("@/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts");
  const { AssistantMessageContent } = await import("@/features/conversation/shared/message/item/view/assistant/assistant-message-content.tsx");
  const message = assistantMessage({
    messageId: "assistant-dm-surface",
    roundId: "round-dm-surface",
    status: "streaming",
    text: "稳定逐字正文",
    timestamp: 2,
  });
  const thinking = { type: "thinking", thinking: "整理答案" };
  const textBlock = message.content[0];
  const orderedEntries = [
    {
      block: thinking,
      mergedIndex: 0,
      sourceMessageId: message.message_id,
      sourceOrder: 0,
    },
    {
      block: textBlock,
      mergedIndex: 1,
      sourceMessageId: message.message_id,
      sourceOrder: 0,
    },
  ];
  const visibleTurns = [{
    content: [thinking, textBlock],
    messageId: message.message_id,
    streamingIndexes: new Set([1]),
    textContent: [textBlock],
    textStreamingIndexes: new Set([0]),
  }];
  const project = (assistantContentMode, streamingBlockIndexes) => (
    resolveMessageItemFinalProjection({
      assistantContentMode,
      assistantMessages: [message],
      orderedProjection: projectionFromOrderedEntries(
        orderedEntries,
        streamingBlockIndexes,
      ),
      resultSummary: undefined,
      roundId: message.round_id,
      streamingBlockIndexes,
      visibleAssistantTurns: visibleTurns,
      visibleOrderedAssistantEntries: orderedEntries,
    })
  );

  assert.equal(resolveAssistantResponseSurface("dm_live"), "final");
  assert.equal(resolveAssistantResponseSurface("dm_archived"), "final");
  const live = project("dm_live", new Set([1]));
  const terminal = project("dm_archived", new Set());
  assert.deepEqual(live.finalAssistantContent, terminal.finalAssistantContent);
  assert.deepEqual(live.directOrderedProjection.content, [thinking]);
  assert.deepEqual(terminal.processProjection.content, [thinking]);
  assert.equal(
    live.directOrderedProjection.content.some((block) => block.type === "text"),
    false,
    "the live process track must not duplicate or own the final response text",
  );

  const messageContentProps = {
    activity: {
      emptyStreamStatus: null,
      label: null,
      showCursor: true,
      standalone: false,
      state: "replying",
      toolUseSummary: null,
    },
    direct: {
      projection: live.directOrderedProjection,
      visible: true,
    },
    environment: {
      canRespondToPermissions: true,
      hiddenToolNames: [],
      mode: "dm_live",
    },
    final: {
      content: live.finalAssistantContent,
      mentions: [],
      isStreaming: true,
      streamingIndexes: live.finalAssistantStreamingIndexes,
      visible: true,
    },
    permissions: {
      all: [],
      matchedByToolUseId: new Map(),
      owner: "composer",
      unmatched: [],
    },
    process: {
      anchorRef: { current: null },
      expanded: false,
      projection: { content: [], streamingIndexes: new Set() },
      summary: { kind: "details", latestDetail: null, metrics: [] },
      toggle: () => {},
      visible: false,
    },
    showMaxTokensWarning: false,
  };
  const html = await renderWithI18n(React.createElement(
    AssistantMessageContent,
    messageContentProps,
  ));
  assert.doesNotMatch(
    html,
    /nexus-chat-final-content[^\"]*before:left-\[5\.5px\]/,
    "the final reply must stay outside the process rail",
  );
  assert.match(
    html,
    /nexus-chat-final-content mt-3\.5 first:mt-0/,
    "a final reply after process content matches the footer spacing",
  );
  assert.equal(
    html.match(/data-timeline-dot/g)?.length,
    1,
    "only the process Thought keeps a timeline node",
  );

  const roomHtml = await renderWithI18n(React.createElement(
    AssistantMessageContent,
    {
      ...messageContentProps,
      environment: {
        ...messageContentProps.environment,
        mode: "room_result",
      },
    },
  ));
  assert.doesNotMatch(roomHtml, /before:left-\[5\.5px\]|data-timeline-dot/);
});

test("failed Assistant result does not expose the fork action", async () => {
  const { resolveAssistantDisplayState } = await import("@/features/conversation/shared/message/item/controller/display/message-item-display-model.ts");
  const projection = {
    assistantMessages: [{}],
    directOrderedProjection: { content: [] },
    finalAssistantContent: "最终回复",
    finalAssistantStreamingIndexes: new Set(),
    finalAssistantText: "最终回复",
    goalCompletionReceipt: null,
    liveActivityState: null,
    mergedContent: [{ type: "text", text: "最终回复" }],
    model: "model",
    pendingInteractionPermissions: [],
    processProjection: { content: [] },
    resultSummary: {
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: true,
      num_turns: 1,
      subtype: "error",
    },
    stats: null,
    streamStatus: "done",
    streamingBlockIndexes: new Set(),
  };
  const options = {
    assistantContentMode: "dm_archived",
    hasForkHandler: true,
    hasStopHandler: false,
    isLastRound: true,
    isLoading: false,
    pendingPermissionCount: 0,
    projection,
  };

  assert.equal(resolveAssistantDisplayState(options).canFork, false);
  assert.equal(resolveAssistantDisplayState({
    ...options,
    projection: {
      ...projection,
      resultSummary: {
        ...projection.resultSummary,
        is_error: false,
        subtype: "success",
      },
    },
  }).canFork, true);
});

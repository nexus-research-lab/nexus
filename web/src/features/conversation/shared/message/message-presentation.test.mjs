// @vitest-environment node

import { renderWithI18n } from "@/features/conversation/testing/message-fixtures.mjs";

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
  assert.equal(buildProcessSummary({pendingPermissionCount: 0, processContent: cancelledContent}).errorCount, 0);
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
    }).errorCount,
    1,
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

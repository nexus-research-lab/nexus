/**
 * INPUT: DM/Room live 过程内容、流式索引、ToolUseSummary 与当前人工交互工具身份。
 * OUTPUT: 以独立交互为边界、首个 tool_use.id 稳定标识且按最后动作判定终态的连续执行段，以及独立内容段。
 * POS: 会话 live 过程压缩的纯投影边界；摘要持续更新包含其 preceding_tool_use_ids 的执行段，权限动作保持独立。
 */
import type {
  ContentBlock,
  ToolUseContent,
} from "@/types/conversation/message/content";

import { ASK_USER_QUESTION_TOOL_NAME } from "../../message-tool-names";
import {
  isRejectedToolResult,
  isSupersededToolResult,
} from "../../tool-result-semantic-model";
import type {
  ContentProjection,
  ToolUseSummaryProjection,
} from "../message-item-projection";

export type ToolRunPhase =
  | "active"
  | "complete"
  | "error"
  | "rejected"
  | "superseded";

export interface ToolRunSegment {
  errorCount: number;
  id: string;
  kind: "tool_run";
  phase: ToolRunPhase;
  projection: ContentProjection;
  rejectedCount: number;
  supersededCount: number;
  summaryText: string | null;
  toolUseIds: string[];
  unresolvedToolUseCount: number;
}

export interface ToolProcessContentSegment {
  id: string;
  kind: "content";
  projection: ContentProjection;
}

export type ToolProcessSegment =
  | ToolProcessContentSegment
  | ToolRunSegment;

interface ToolRunSegmentOptions {
  interactiveToolUseIds: ReadonlySet<string>;
  live: boolean;
  projection: ContentProjection;
  responseResumed: boolean;
  toolUseSummary: ToolUseSummaryProjection | null;
}

interface PendingToolRun {
  indexes: Set<number>;
  toolUses: ToolUseContent[];
}

const EMPTY_INDEXES: ReadonlySet<number> = new Set<number>();

export function projectToolRunSegments({
  interactiveToolUseIds,
  live,
  projection,
  responseResumed,
  toolUseSummary,
}: ToolRunSegmentOptions): ToolProcessSegment[] {
  const supportIndexesByToolUseId = collectToolSupportIndexes(
    projection.content,
  );
  const claimedSupportIndexes = collectClaimedSupportIndexes(
    supportIndexesByToolUseId,
  );
  const segments: ToolProcessSegment[] = [];
  let pendingProcessIndexes: number[] = [];
  let pendingToolRun: PendingToolRun | null = null;

  const flushPendingProcess = () => {
    pendingProcessIndexes.forEach((index) => {
      const block = projection.content[index];
      if (block) {
        segments.push(buildContentSegment(block, index, projection));
      }
    });
    pendingProcessIndexes = [];
  };

  const flushToolRun = (closed: boolean) => {
    if (!pendingToolRun) {
      return;
    }
    segments.push(buildToolRunSegment({
      closed,
      live,
      projection,
      run: pendingToolRun,
    }));
    pendingToolRun = null;
  };

  projection.content.forEach((block, index) => {
    if (claimedSupportIndexes.has(index)) {
      return;
    }

    if (block.type === "tool_use") {
      if (isInteractiveToolUse(block, interactiveToolUseIds)) {
        flushToolRun(true);
        flushPendingProcess();
        segments.push(buildInteractiveToolSegment(
          block,
          index,
          projection,
          supportIndexesByToolUseId.get(block.id) ?? EMPTY_INDEXES,
        ));
        return;
      }

      pendingToolRun ??= {
        indexes: new Set<number>(pendingProcessIndexes),
        toolUses: [],
      };
      pendingProcessIndexes = [];
      pendingToolRun.indexes.add(index);
      pendingToolRun.toolUses.push(block);
      addIndexes(
        pendingToolRun.indexes,
        supportIndexesByToolUseId.get(block.id),
      );
      return;
    }

    if (pendingToolRun && isTrailingToolSupport(block)) {
      pendingToolRun.indexes.add(index);
      return;
    }

    if (isCollapsibleProcessBlock(block)) {
      if (pendingToolRun) {
        pendingToolRun.indexes.add(index);
      } else {
        pendingProcessIndexes.push(index);
      }
      return;
    }

    flushToolRun(true);
    flushPendingProcess();
    segments.push(buildContentSegment(block, index, projection));
  });

  flushToolRun(responseResumed || !live);
  flushPendingProcess();
  return attachToolUseSummary(segments, toolUseSummary);
}

function buildToolRunSegment({
  closed,
  live,
  projection,
  run,
}: {
  closed: boolean;
  live: boolean;
  projection: ContentProjection;
  run: PendingToolRun;
}): ToolRunSegment {
  const toolUseIds = run.toolUses.map((block) => block.id);
  const errorCount = countToolRunErrors(run.indexes, projection.content);
  const rejectedCount = countToolRunRejections(
    run.indexes,
    projection.content,
  );
  const supersededCount = countToolRunSupersessions(
    run.indexes,
    projection.content,
  );
  const unresolvedToolUseCount = countUnresolvedToolUses(
    toolUseIds,
    run.indexes,
    projection.content,
  );
  const terminal = (
    closed
    && (!live || unresolvedToolUseCount === 0)
  );
  const phase: ToolRunPhase = !terminal
    ? "active"
    : resolveTerminalToolRunPhase(
        toolUseIds[toolUseIds.length - 1],
        run.indexes,
        projection.content,
      );
  return {
    errorCount,
    id: `tool-run:${toolUseIds[0]}`,
    kind: "tool_run",
    phase,
    projection: selectProjectionIndexes(projection, run.indexes),
    rejectedCount,
    summaryText: null,
    supersededCount,
    toolUseIds,
    unresolvedToolUseCount,
  };
}

function buildInteractiveToolSegment(
  block: ToolUseContent,
  index: number,
  projection: ContentProjection,
  supportIndexes: ReadonlySet<number>,
): ToolProcessContentSegment {
  const indexes = new Set<number>([index]);
  addIndexes(indexes, supportIndexes);
  return {
    id: `interactive-tool:${block.id}`,
    kind: "content",
    projection: selectProjectionIndexes(projection, indexes),
  };
}

function buildContentSegment(
  block: ContentBlock,
  index: number,
  projection: ContentProjection,
): ToolProcessContentSegment {
  return {
    id: contentSegmentId(block, index),
    kind: "content",
    projection: selectProjectionIndexes(projection, new Set([index])),
  };
}

function collectToolSupportIndexes(
  content: readonly ContentBlock[],
): Map<string, Set<number>> {
  const toolUseIds = new Set(
    content.flatMap((block) => block.type === "tool_use" ? [block.id] : []),
  );
  const supportIndexesByToolUseId = new Map<string, Set<number>>();
  content.forEach((block, index) => {
    const toolUseId = toolSupportOwnerId(block);
    if (!toolUseId || !toolUseIds.has(toolUseId)) {
      return;
    }
    const indexes = supportIndexesByToolUseId.get(toolUseId)
      ?? new Set<number>();
    indexes.add(index);
    supportIndexesByToolUseId.set(toolUseId, indexes);
  });
  return supportIndexesByToolUseId;
}

function collectClaimedSupportIndexes(
  supportIndexesByToolUseId: ReadonlyMap<string, ReadonlySet<number>>,
): Set<number> {
  const indexes = new Set<number>();
  supportIndexesByToolUseId.forEach((supportIndexes) => {
    addIndexes(indexes, supportIndexes);
  });
  return indexes;
}

function toolSupportOwnerId(block: ContentBlock): string | null {
  if (block.type === "tool_result") {
    return block.tool_use_id;
  }
  if (block.type === "task_progress") {
    return block.tool_use_id ?? null;
  }
  if (block.type === "system_event") {
    return block.tool_use_id ?? null;
  }
  if (block.type === "workspace_file_artifact") {
    return block.source_tool_use_id ?? null;
  }
  return null;
}

function isInteractiveToolUse(
  block: ToolUseContent,
  interactiveToolUseIds: ReadonlySet<string>,
): boolean {
  return (
    block.name === ASK_USER_QUESTION_TOOL_NAME
    || interactiveToolUseIds.has(block.id)
  );
}

function isTrailingToolSupport(block: ContentBlock): boolean {
  return (
    block.type === "tool_result"
    || block.type === "task_progress"
    || block.type === "tool_use_error"
  );
}

function isCollapsibleProcessBlock(block: ContentBlock): boolean {
  return (
    block.type === "text"
    || block.type === "thinking"
    || block.type === "redacted_thinking"
    || block.type === "progress_update"
  );
}

function selectProjectionIndexes(
  projection: ContentProjection,
  selectedIndexes: ReadonlySet<number>,
): ContentProjection {
  const content: ContentBlock[] = [];
  const streamingIndexes = new Set<number>();
  [...selectedIndexes]
    .sort((left, right) => left - right)
    .forEach((sourceIndex) => {
      const block = projection.content[sourceIndex];
      if (!block) {
        return;
      }
      const nextIndex = content.length;
      content.push(block);
      if (projection.streamingIndexes.has(sourceIndex)) {
        streamingIndexes.add(nextIndex);
      }
    });
  return { content, streamingIndexes };
}

function countToolRunErrors(
  indexes: ReadonlySet<number>,
  content: readonly ContentBlock[],
): number {
  let count = 0;
  indexes.forEach((index) => {
    const block = content[index];
    if (
      block?.type === "tool_use_error"
      || (block?.type === "tool_result" && block.is_error)
    ) {
      count += 1;
    }
  });
  return count;
}

function resolveTerminalToolRunPhase(
  lastToolUseId: string | undefined,
  indexes: ReadonlySet<number>,
  content: readonly ContentBlock[],
): Exclude<ToolRunPhase, "active"> {
  if (!lastToolUseId) {
    return "complete";
  }
  let lastToolUseIndex = -1;
  indexes.forEach((index) => {
    const block = content[index];
    if (
      block?.type === "tool_use"
      && block.id === lastToolUseId
      && index > lastToolUseIndex
    ) {
      lastToolUseIndex = index;
    }
  });
  let phase: Exclude<ToolRunPhase, "active"> = "complete";
  [...indexes]
    .sort((left, right) => left - right)
    .forEach((index) => {
      if (index <= lastToolUseIndex) {
        return;
      }
      const block = content[index];
      if (block?.type === "tool_use_error") {
        phase = "error";
        return;
      }
      if (
        block?.type !== "tool_result"
        || block.tool_use_id !== lastToolUseId
      ) {
        return;
      }
      phase = block.is_error
        ? "error"
        : isRejectedToolResult(block)
        ? "rejected"
        : isSupersededToolResult(block)
        ? "superseded"
        : "complete";
    });
  return phase;
}

function countUnresolvedToolUses(
  toolUseIds: readonly string[],
  indexes: ReadonlySet<number>,
  content: readonly ContentBlock[],
): number {
  const resolvedToolUseIds = new Set<string>();
  indexes.forEach((index) => {
    const block = content[index];
    if (block?.type === "tool_result") {
      resolvedToolUseIds.add(block.tool_use_id);
    }
  });
  return toolUseIds.filter(
    (toolUseId) => !resolvedToolUseIds.has(toolUseId),
  ).length;
}

function countToolRunRejections(
  indexes: ReadonlySet<number>,
  content: readonly ContentBlock[],
): number {
  let count = 0;
  indexes.forEach((index) => {
    const block = content[index];
    if (block?.type === "tool_result" && isRejectedToolResult(block)) {
      count += 1;
    }
  });
  return count;
}

function countToolRunSupersessions(
  indexes: ReadonlySet<number>,
  content: readonly ContentBlock[],
): number {
  let count = 0;
  indexes.forEach((index) => {
    const block = content[index];
    if (block?.type === "tool_result" && isSupersededToolResult(block)) {
      count += 1;
    }
  });
  return count;
}

function contentSegmentId(block: ContentBlock, index: number): string {
  if (block.type === "system_event") {
    return `content:system:${block.source_message_id}:${block.subtype ?? index}`;
  }
  if (block.type === "workspace_file_artifact") {
    return `content:artifact:${block.id ?? block.path}`;
  }
  if (block.type === "tool_result") {
    return `content:result:${block.tool_use_id}`;
  }
  return `content:${block.type}:${index}`;
}

function addIndexes(
  target: Set<number>,
  source: ReadonlySet<number> | undefined,
): void {
  source?.forEach((index) => target.add(index));
}

function attachToolUseSummary(
  segments: ToolProcessSegment[],
  summary: ToolUseSummaryProjection | null,
): ToolProcessSegment[] {
  if (!summary?.text.trim()) {
    return segments;
  }
  const summaryToolUseIds = new Set(summary.precedingToolUseIds);
  let targetIndex = -1;
  if (summaryToolUseIds.size > 0) {
    targetIndex = segments.findIndex((segment) => (
      segment.kind === "tool_run"
      && segment.toolUseIds.length > 0
      && [...summaryToolUseIds].every((toolUseId) => (
        segment.toolUseIds.includes(toolUseId)
      ))
    ));
  } else {
    targetIndex = segments.findLastIndex(
      (segment) => segment.kind === "tool_run",
    );
  }
  if (targetIndex < 0) {
    return segments;
  }
  return segments.map((segment, index) => (
    index === targetIndex && segment.kind === "tool_run"
      ? { ...segment, summaryText: summary.text.trim() }
      : segment
  ));
}

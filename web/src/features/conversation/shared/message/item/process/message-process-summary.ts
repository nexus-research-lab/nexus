import type { ContentBlock } from "@/types/conversation/message/content";

import {
  isRecoverableToolResult,
  isRecoverableToolUse,
} from "../../message-content-model";
import {
  getCompactToolInputSummary,
  getSemanticToolName,
} from "../../tool-activity";
import { isRejectedToolResult } from "../../tool-result-semantic-model";

const PROCESS_SUMMARY_DETAIL_LIMIT = 72;

interface ProcessMetric {
  kind: ProcessSummaryMetricKind;
  matches: (block: ContentBlock) => boolean;
}

const PROCESS_METRICS: ProcessMetric[] = [
  { kind: "thinking", matches: (block) => block.type === "thinking" },
  {
    kind: "action",
    matches: (block) => block.type === "tool_use"
      && !isRecoverableToolUse(block),
  },
  {
    kind: "error",
    matches: (block) => block.type === "tool_result"
      && (Boolean(block.is_error) || isRejectedToolResult(block))
      && !isRecoverableToolResult(block),
  },
  {
    kind: "guidance",
    matches: (block) => block.type === "system_event"
      && block.subtype === "guided_input",
  },
];

const PROCESS_DETAIL_RESOLVERS: ReadonlyArray<
  (block: ContentBlock) => ProcessSummaryDetail | null
> = [
  (block) => block.type === "task_progress"
    ? projectTaskProgressDetail(block.description || block.last_tool_name)
    : null,
  (block) => {
    if (block.type !== "tool_use" || isRecoverableToolUse(block)) {
      return null;
    }
    const detail = getCompactToolInputSummary(block.input);
    return {
      detail: detail ? compactProcessDetail(detail) : null,
      kind: "tool",
      toolName: getSemanticToolName(block.name, block.input),
    };
  },
  (block) => block.type === "system_event"
    ? projectTextDetail(block.content || block.label)
    : null,
  (block) => block.type === "tool_use_error"
    ? projectTextDetail(block.content)
    : null,
];

export type ProcessSummaryMetricKind =
  | "action"
  | "error"
  | "guidance"
  | "thinking";

export interface ProcessSummaryMetric {
  count: number;
  kind: ProcessSummaryMetricKind;
}

export type ProcessSummaryDetail =
  | { kind: "background_task" }
  | { kind: "text"; text: string }
  | { detail: string | null; kind: "tool"; toolName: string };

export type ProcessSummaryProjection =
  | { kind: "waiting_permission" }
  | {
      kind: "details";
      latestDetail: ProcessSummaryDetail | null;
      metrics: ProcessSummaryMetric[];
    };

export function buildProcessSummary({
  pendingPermissionCount,
  processContent,
}: {
  pendingPermissionCount: number;
  processContent: readonly ContentBlock[];
}): ProcessSummaryProjection {
  if (pendingPermissionCount > 0) {
    return { kind: "waiting_permission" };
  }

  const metrics = PROCESS_METRICS.flatMap(({ kind, matches }) => {
    const count = processContent.filter(matches).length;
    return count > 0 ? [{ count, kind }] : [];
  });
  return {
    kind: "details",
    latestDetail: latestProcessDetail(processContent),
    metrics,
  };
}

function latestProcessDetail(
  processContent: readonly ContentBlock[],
): ProcessSummaryDetail | null {
  for (let index = processContent.length - 1; index >= 0; index -= 1) {
    const block = processContent[index];
    for (const resolveDetail of PROCESS_DETAIL_RESOLVERS) {
      const detail = resolveDetail(block);
      if (detail) {
        return detail;
      }
    }
  }
  return null;
}

function projectTaskProgressDetail(
  value: string | null | undefined,
): ProcessSummaryDetail {
  const text = value ? compactProcessDetail(value) : null;
  return text ? { kind: "text", text } : { kind: "background_task" };
}

function projectTextDetail(value: string): ProcessSummaryDetail | null {
  const text = compactProcessDetail(value);
  return text ? { kind: "text", text } : null;
}

function compactProcessDetail(value: string): string | null {
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) {
    return null;
  }
  return text.length <= PROCESS_SUMMARY_DETAIL_LIMIT
    ? text
    : `${text.slice(0, PROCESS_SUMMARY_DETAIL_LIMIT - 1)}…`;
}

import type { ContentBlock } from "@/types/conversation/message/content";

import {
  getToolInputSummary,
  getToolTitle,
} from "../../tool-activity";

const PROCESS_SUMMARY_DETAIL_LIMIT = 72;

interface ProcessMetric {
  label: string;
  matches: (block: ContentBlock) => boolean;
}

const PROCESS_METRICS: ProcessMetric[] = [
  { label: "段思路", matches: (block) => block.type === "thinking" },
  { label: "次动作", matches: (block) => block.type === "tool_use" },
  {
    label: "个异常",
    matches: (block) => block.type === "tool_result" && Boolean(block.is_error),
  },
  {
    label: "次引导",
    matches: (block) => block.type === "system_event"
      && block.subtype === "guided_input",
  },
];

const PROCESS_DETAIL_RESOLVERS: ReadonlyArray<
  (block: ContentBlock) => string | null
> = [
  (block) => block.type === "task_progress"
    ? compactProcessDetail(
      block.description || block.last_tool_name || "后台任务正在执行",
    )
    : null,
  (block) => {
    if (block.type !== "tool_use") {
      return null;
    }
    const detail = getToolInputSummary(block.input);
    return compactProcessDetail(
      detail ? `${getToolTitle(block.name)}：${detail}` : getToolTitle(block.name),
    );
  },
  (block) => block.type === "system_event"
    ? compactProcessDetail(block.content || block.label)
    : null,
  (block) => block.type === "tool_use_error"
    ? compactProcessDetail(block.content)
    : null,
];

export function buildProcessSummary({
  pendingPermissionCount,
  processContent,
}: {
  pendingPermissionCount: number;
  processContent: readonly ContentBlock[];
}): string {
  if (pendingPermissionCount > 0) {
    return "等待你的确认后继续";
  }

  const summaryParts = PROCESS_METRICS.flatMap(({ label, matches }) => {
    const count = processContent.filter(matches).length;
    return count > 0 ? [`${count} ${label}`] : [];
  });
  const summary = summaryParts.length > 0 ? summaryParts.join(" · ") : "查看过程";
  const latestDetail = latestProcessDetail(processContent);
  return latestDetail ? `${summary} · 最近：${latestDetail}` : summary;
}

function latestProcessDetail(
  processContent: readonly ContentBlock[],
): string | null {
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

function compactProcessDetail(value: string): string | null {
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) {
    return null;
  }
  return text.length <= PROCESS_SUMMARY_DETAIL_LIMIT
    ? text
    : `${text.slice(0, PROCESS_SUMMARY_DETAIL_LIMIT - 1)}…`;
}

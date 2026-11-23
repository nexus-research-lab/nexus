/**
 * INPUT: Thought 正文、流式状态与展开默认值。
 * OUTPUT: 默认收起的单行正文预览；用户展开后保留选择，流式阶段切换不重置明细。
 * POS: Assistant 执行过程中的 Thought 二级明细入口；只是内容，不表达运行态——“正在思考”由消息活动行独占。
 */
"use client";

import { Brain } from "lucide-react";
import type { RefObject } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useI18n } from "@/shared/i18n/i18n-context";

import { MarkdownRenderer } from "../markdown-renderer";
import {
  MessageDetailFrame,
  MessageDetailScroll,
} from "../ui/message-rail";
import { MessageDetailToggle } from "../ui/message-detail-toggle";

interface ThinkingBlockProps {
  defaultExpanded?: boolean;
  thinking: string;
  initialRevealFromEmpty?: boolean;
  isStreaming?: boolean;
  workspaceAgentId?: string | null;
}

export function ThinkingBlock({
  defaultExpanded,
  thinking,
  initialRevealFromEmpty = false,
  isStreaming = false,
  workspaceAgentId,
}: ThinkingBlockProps) {
  const { t } = useI18n();
  // 展开只由用户选择；流式开始或结束不得自动打开或关闭详情。
  const expansion = useScrollAnchoredState(defaultExpanded ?? false);
  const isExpanded = expansion.isOpen;
  const preview = thinking.replace(/\s+/g, " ").trim();
  if (!thinking) {
    return null;
  }

  return (
    <div
      className="min-w-0"
      ref={expansion.anchorRef as RefObject<HTMLDivElement>}
    >
      <MessageDetailToggle
        contentClassName="flex items-baseline gap-1.5"
        data-activity-row="thinking"
        data-message-detail-sticky-header={isExpanded || undefined}
        expanded={isExpanded}
        leading={(
          <span
            className="flex h-5 w-5 shrink-0 items-center justify-center text-(--icon-muted)"
            data-thinking-block-icon="thinking"
          >
            <Brain aria-hidden className="h-3.5 w-3.5" strokeWidth={1.5} />
          </span>
        )}
        onClick={expansion.toggle}
      >
        <span className="shrink-0">{t("message.thought")}</span>
        {!isExpanded ? (
          <span
            className="min-w-0 flex-1 truncate text-(--text-soft)"
            data-thinking-block-preview
          >
            {preview}
          </span>
        ) : null}
      </MessageDetailToggle>
      {isExpanded ? (
        <MessageDetailFrame>
          <MessageDetailScroll followContent={isStreaming}>
            <MarkdownRenderer
              key={isStreaming ? "streaming" : "complete"}
              className="nexus-message-detail-markdown min-w-0 max-w-full overflow-hidden break-all text-(--text-muted)"
              content={thinking}
              initialRevealFromEmpty={initialRevealFromEmpty}
              isStreaming={isStreaming}
              workspaceAgentId={workspaceAgentId}
            />
          </MessageDetailScroll>
        </MessageDetailFrame>
      ) : null}
    </div>
  );
}

"use client";

/**
 * INPUT: 待发送消息、折叠状态与排序/引导/删除命令。
 * OUTPUT: 可拖动、可收起且使用共享微型动作的 Composer 队列。
 * POS: Composer 输入队列装配层；队列事务由 controller 持有。
 */

import { ChevronDown, ChevronUp } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import type { InputQueueItem } from "@/types/agent/agent-conversation";

import {
  getPendingQueuePaddingClassName,
  projectPendingQueueItem,
} from "./pending-queue-model";
import { PendingQueueItem } from "./pending-queue-item";
import { usePendingQueueController } from "./use-pending-queue-controller";

interface ComposerPendingQueueProps {
  compact: boolean;
  inputQueueItems: InputQueueItem[];
  onDeleteQueuedMessage: (itemId: string) => void | Promise<void>;
  onGuideQueuedMessage: (itemId: string) => void | Promise<void>;
  onReorderQueueMessages: (orderedIds: string[]) => void | Promise<void>;
}

export function ComposerPendingQueue({
  compact,
  inputQueueItems,
  onDeleteQueuedMessage,
  onGuideQueuedMessage,
  onReorderQueueMessages,
}: ComposerPendingQueueProps) {
  const controller = usePendingQueueController({
    commands: {
      deleteMessage: onDeleteQueuedMessage,
      guideMessage: onGuideQueuedMessage,
      reorderMessages: onReorderQueueMessages,
    },
    items: inputQueueItems,
  });
  if (inputQueueItems.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "border-b border-(--surface-canvas-border)",
        getPendingQueuePaddingClassName(compact),
      )}
    >
      <PendingQueueHeader
        collapsed={controller.state.isCollapsed}
        count={inputQueueItems.length}
        onToggle={controller.actions.toggleCollapsed}
      />
      <div
        className={cn(
          "soft-scrollbar flex max-h-[112px] flex-col divide-y divide-(--divider-subtle-color) overflow-y-auto pr-1",
          controller.state.isCollapsed ? "hidden" : "mt-0.5",
        )}
        onDragOver={(event) => {
          event.preventDefault();
          controller.actions.startAutoScroll(event.clientY);
        }}
        ref={controller.refs.scrollRef}
      >
        {inputQueueItems.map((item) => (
          <PendingQueueItem
            key={item.id}
            isActionRunning={controller.state.isActionRunning}
            item={item}
            onDelete={controller.actions.deleteMessage}
            onDragEnd={controller.actions.finishDrag}
            onDragOver={controller.actions.dragOver}
            onDragStart={controller.actions.startDrag}
            onDrop={controller.actions.dropOnMessage}
            onGuide={(messageId) => {
              void controller.actions.guideMessage(messageId);
            }}
            projection={projectPendingQueueItem(
              item,
              controller.state.dragState,
            )}
          />
        ))}
      </div>
    </div>
  );
}

function PendingQueueHeader({
  collapsed,
  count,
  onToggle,
}: {
  collapsed: boolean;
  count: number;
  onToggle: () => void;
}) {
  const { t } = useI18n();
  const CollapseIcon = collapsed ? ChevronDown : ChevronUp;
  const label = collapsed
    ? t("composer.expand_pending_queue")
    : t("composer.collapse_pending_queue");
  return (
    <div className="flex items-center justify-between gap-2 text-2xs font-medium text-(--text-soft)">
      <span className="inline-flex items-center gap-1.5">
        {t("composer.pending_queue")}
        <span className="tabular-nums">{count}</span>
      </span>
      <UiIconButton
        aria-label={label}
        className="shrink-0"
        onClick={onToggle}
        size="2xs"
        tooltip={label}
        variant="ghost"
      >
        <CollapseIcon className="h-3 w-3" />
      </UiIconButton>
    </div>
  );
}

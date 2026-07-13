"use client";

import { ChevronDown, ChevronUp } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
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
    <div className="flex items-center justify-between gap-2 text-[10px] font-medium text-(--text-soft)">
      <span className="inline-flex items-center gap-1.5">
        {t("composer.pending_queue")}
        <span className="tabular-nums">{count}</span>
      </span>
      <button
        aria-label={label}
        className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={onToggle}
        type="button"
      >
        <CollapseIcon className="h-3 w-3" />
      </button>
    </div>
  );
}

"use client";

/**
 * INPUT: 待发送消息与排序/引导/删除命令。
 * OUTPUT: 共享 Disclosure 内的有序队列；支持拖动和键盘排序，命令只按当前条目派发。
 * POS: Composer 输入队列装配层；队列事务由 controller 持有。
 */

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import type { InputQueueItem } from "@/types/agent/agent-conversation";

import { getPendingQueuePaddingClassName } from "../../composer-styles";
import { projectPendingQueueItem } from "./pending-queue-model";
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
  const { t } = useI18n();
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
    <UiDisclosure
      defaultOpen
      density="compact"
      label={t("composer.pending_queue")}
      meta={<span className="tabular-nums">{inputQueueItems.length}</span>}
      onToggle={(event) => { if (!event.currentTarget.open) controller.actions.finishDrag(); }}
      summaryRole="caption"
      summaryTone="muted"
      className={cn(
        "border-b border-(--surface-canvas-border)",
        getPendingQueuePaddingClassName(compact),
      )}
    >
      <ol
        aria-label={t("composer.pending_queue")}
        className="soft-scrollbar flex max-h-28 flex-col divide-y divide-(--divider-subtle-color) overflow-y-auto overscroll-contain pr-1"
        onDragOver={(event) => {
          event.preventDefault();
          controller.actions.startAutoScroll(event.clientY);
        }}
        ref={controller.refs.scrollRef}
      >
        {inputQueueItems.map((item, index) => (
          <PendingQueueItem
            key={item.id}
            canMoveUp={index > 0}
            canMoveDown={index < inputQueueItems.length - 1}
            isActionRunning={controller.state.isActionRunning}
            item={item}
            onDelete={controller.actions.deleteMessage}
            onDragEnd={controller.actions.finishDrag}
            onDragOver={controller.actions.dragOver}
            onDragStart={controller.actions.startDrag}
            onDrop={controller.actions.dropOnMessage}
            onMove={controller.actions.moveMessage}
            onGuide={(messageId) => {
              void controller.actions.guideMessage(messageId);
            }}
            projection={projectPendingQueueItem(
              item,
              controller.state.dragState,
            )}
          />
        ))}
      </ol>
    </UiDisclosure>
  );
}

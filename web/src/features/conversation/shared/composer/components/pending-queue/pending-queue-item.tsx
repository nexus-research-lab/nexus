/**
 * INPUT: 单条排队消息、拖动投影与引导/删除动作。
 * OUTPUT: 保持原生拖动语义并复用共享微型动作的队列行。
 * POS: Composer Pending Queue 行视图；不拥有队列顺序或命令状态。
 */
import { CornerDownRight, GripVertical, Paperclip, Trash2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import type { InputQueueItem } from "@/types/agent/agent-conversation";

import type {
  PendingQueueItemContent,
  PendingQueueItemProjection,
} from "./pending-queue-model";

interface PendingQueueItemProps {
  isActionRunning: boolean;
  item: InputQueueItem;
  onDelete: (messageId: string) => void;
  onDragEnd: () => void;
  onDragOver: (messageId: string, clientY: number) => void;
  onDragStart: (messageId: string) => void;
  onDrop: (messageId: string) => void;
  onGuide: (messageId: string) => void;
  projection: PendingQueueItemProjection;
}

export function PendingQueueItem({
  isActionRunning,
  item,
  onDelete,
  onDragEnd,
  onDragOver,
  onDragStart,
  onDrop,
  onGuide,
  projection,
}: PendingQueueItemProps) {
  const { t } = useI18n();
  const guideAriaLabel = projection.isGuidanceWaiting
    ? t("composer.cancel_guidance")
    : t("composer.mark_guidance");
  const guideActionLabel = projection.isGuidanceWaiting
    ? t("composer.cancel_guide_action")
    : t("composer.guide_action");

  return (
    <div
      draggable
      className={cn(
        "group -mx-1 flex min-h-7 items-center gap-2 border-l-[3px] border-l-transparent px-1 py-0.5 text-(--text-default) transition-[background,border-color,opacity]",
        projection.isDragging && "opacity-60",
        projection.isDragTarget
          && "border-l-(--primary) bg-(--surface-interactive-hover-background)",
      )}
      onDragEnd={onDragEnd}
      onDragOver={(event) => {
        event.preventDefault();
        onDragOver(item.id, event.clientY);
      }}
      onDragStart={() => onDragStart(item.id)}
      onDrop={(event) => {
        event.preventDefault();
        onDrop(item.id);
      }}
    >
      <span
        aria-label={t("composer.drag_to_reorder")}
        className="inline-flex h-5 w-3.5 shrink-0 cursor-grab items-center justify-center text-(--text-soft) active:cursor-grabbing"
      >
        <GripVertical className="h-3.5 w-3.5" />
      </span>
      <PendingQueueItemContentView content={projection.content} />
      <UiButton
        aria-label={guideAriaLabel}
        className="shrink-0"
        disabled={isActionRunning}
        onClick={() => onGuide(item.id)}
        size="2xs"
        variant="text"
      >
        <CornerDownRight className="h-3 w-3" />
        {guideActionLabel}
      </UiButton>
      <UiIconButton
        aria-label={t("composer.delete_pending")}
        className="shrink-0"
        onClick={() => onDelete(item.id)}
        size="xs"
        tone="danger"
        tooltip={t("composer.delete_pending")}
        variant="ghost"
      >
        <Trash2 className="h-3 w-3" />
      </UiIconButton>
    </div>
  );
}

function PendingQueueItemContentView({
  content,
}: {
  content: PendingQueueItemContent | null;
}) {
  if (!content) {
    return <p className="min-w-0 flex-1" />;
  }
  if (content.kind === "attachments") {
    return (
      <p className="line-clamp-1 inline-flex min-w-0 flex-1 items-center gap-1 text-compact leading-5 text-(--text-muted)">
        <Paperclip className="h-3 w-3 shrink-0" />
        {content.text}
      </p>
    );
  }
  return (
    <p className="line-clamp-1 min-w-0 flex-1 text-compact leading-5 text-(--text-strong)">
      {content.text}
    </p>
  );
}

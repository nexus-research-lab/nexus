/**
 * INPUT: 单条排队消息、拖动投影与引导/删除动作。
 * OUTPUT: 有序队列行、完整内容提示与共享拖动/排序入口；引导和删除保持独立原生命中。
 * POS: Composer Pending Queue 行视图；不拥有队列顺序或命令状态。
 */
import { useId, useRef } from "react";
import { ArrowDown, ArrowUp, CornerDownRight, GripVertical, Paperclip, Trash2 } from "lucide-react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import type { InputQueueItem } from "@/types/agent/agent-conversation";

import type {
  PendingQueueItemContent,
  PendingQueueItemProjection,
} from "./pending-queue-model";

interface PendingQueueItemProps {
  canMoveUp: boolean;
  canMoveDown: boolean;
  isActionRunning: boolean;
  item: InputQueueItem;
  onDelete: (messageId: string) => void;
  onDragEnd: () => void;
  onDragOver: (messageId: string, clientY: number) => void;
  onDragStart: (messageId: string) => void;
  onDrop: (messageId: string) => void;
  onGuide: (messageId: string) => void;
  onMove: (messageId: string, direction: -1 | 1) => void;
  projection: PendingQueueItemProjection;
}

export function PendingQueueItem({
  canMoveUp,
  canMoveDown,
  isActionRunning,
  item,
  onDelete,
  onDragEnd,
  onDragOver,
  onDragStart,
  onDrop,
  onGuide,
  onMove,
  projection,
}: PendingQueueItemProps) {
  const { t } = useI18n();
  const contentId = useId();
  const moveAnchorRef = useRef<HTMLButtonElement>(null);
  const canReorder = !isActionRunning && (canMoveUp || canMoveDown);
  const [isMoveMenuOpen, setMoveMenuOpen] = useResettableState(false, canReorder);
  const guideAriaLabel = projection.isGuidanceWaiting
    ? t("composer.cancel_guidance")
    : t("composer.mark_guidance");
  const guideActionLabel = projection.isGuidanceWaiting
    ? t("composer.cancel_guide_action")
    : t("composer.guide_action");

  return (
    <li
      className={cn(
        "group -mx-1 flex min-h-7 items-center gap-2 border-l-[3px] border-l-transparent px-1 py-0.5 text-(--text-default) transition-[background,border-color,opacity]",
        projection.isDragging && "opacity-60",
        projection.isDragTarget
          && "border-l-(--primary) bg-(--surface-interactive-hover-background)",
      )}
      onDragOver={(event) => {
        event.preventDefault();
        onDragOver(item.id, event.clientY);
      }}
      onDrop={(event) => {
        event.preventDefault();
        onDrop(item.id);
      }}
    >
      <UiIconButton
        ref={moveAnchorRef}
        aria-describedby={contentId}
        aria-expanded={canReorder && isMoveMenuOpen}
        aria-haspopup="menu"
        aria-label={t("composer.reorder_pending")}
        className="shrink-0 cursor-grab active:cursor-grabbing"
        disabled={!canReorder}
        draggable={canReorder}
        onClick={() => setMoveMenuOpen((open) => !open)}
        onDragEnd={onDragEnd}
        onDragStart={(event) => {
          setMoveMenuOpen(false);
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("application/x-nexus-input-queue", item.id);
          onDragStart(item.id);
        }}
        size="2xs"
        tooltip={t("composer.drag_to_reorder")}
        variant="ghost"
      >
        <GripVertical className="h-3.5 w-3.5" />
      </UiIconButton>
      <UiActionMenu
        anchorRef={moveAnchorRef}
        ariaLabel={t("composer.reorder_pending")}
        density="compact"
        isOpen={canReorder && isMoveMenuOpen}
        items={[
          { value: "up", label: t("composer.move_pending_up"), icon: <ArrowUp className="h-4 w-4" />, disabled: !canMoveUp },
          { value: "down", label: t("composer.move_pending_down"), icon: <ArrowDown className="h-4 w-4" />, disabled: !canMoveDown },
        ]}
        onClose={() => setMoveMenuOpen(false)}
        onSelect={(value) => onMove(item.id, value === "up" ? -1 : 1)}
      />
      <PendingQueueItemContentView content={projection.content} id={contentId} />
      <UiButton
        aria-describedby={contentId}
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
        aria-describedby={contentId}
        aria-label={t("composer.delete_pending")}
        disabled={isActionRunning}
        className="shrink-0"
        onClick={() => onDelete(item.id)}
        size="xs"
        tone="danger"
        tooltip={t("composer.delete_pending")}
        variant="ghost"
      >
        <Trash2 className="h-3 w-3" />
      </UiIconButton>
    </li>
  );
}

function PendingQueueItemContentView({
  content,
  id,
}: {
  content: PendingQueueItemContent | null;
  id: string;
}) {
  const { t } = useI18n();
  return (
    <p className={`flex min-w-0 flex-1 items-center gap-1 ${getUiTypographyClassName({
      role: "supporting", tone: content?.kind === "text" ? "strong" : "muted",
    })}`} id={id} title={content?.text}>
      {content?.kind === "attachments" ? <Paperclip aria-hidden="true" className="h-3 w-3 shrink-0" /> : null}
      <span className="min-w-0 truncate">{content?.text || t("composer.pending_message")}</span>
    </p>
  );
}

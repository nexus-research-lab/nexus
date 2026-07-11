"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronUp,
  CornerDownRight,
  GripVertical,
  Paperclip,
  Trash2,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { InputQueueItem } from "@/types/agent/agent-conversation";

const PENDING_QUEUE_AUTO_SCROLL_ZONE_PX = 28;
const PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX = 10;

function reorderPendingMessages(
  messages: InputQueueItem[],
  sourceId: string,
  targetId: string,
): InputQueueItem[] {
  const sourceIndex = messages.findIndex((item) => item.id === sourceId);
  const targetIndex = messages.findIndex((item) => item.id === targetId);
  if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) {
    return messages;
  }
  const next = [...messages];
  const [source] = next.splice(sourceIndex, 1);
  next.splice(targetIndex, 0, source);
  return next;
}

export function ComposerPendingQueue({
  compact,
  inputQueueItems,
  onDeleteQueuedMessage,
  onGuideQueuedMessage,
  onReorderQueueMessages,
}: {
  compact: boolean;
  inputQueueItems: InputQueueItem[];
  onDeleteQueuedMessage?: (itemId: string) => void | Promise<void>;
  onGuideQueuedMessage?: (itemId: string) => void | Promise<void>;
  onReorderQueueMessages?: (orderedIds: string[]) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const [draggingMessageId, setDraggingMessageId] = useState<string | null>(null);
  const [dragOverMessageId, setDragOverMessageId] = useState<string | null>(null);
  const [isPendingQueueCollapsed, setIsPendingQueueCollapsed] = useState(false);
  const [isQueueActionRunning, setIsQueueActionRunning] = useState(false);
  const pendingQueueScrollRef = useRef<HTMLDivElement>(null);
  const pendingQueueDragYRef = useRef<number | null>(null);
  const pendingQueueScrollFrameRef = useRef<number | null>(null);
  const draggingMessageIdRef = useRef<string | null>(null);

  const stopPendingQueueAutoScroll = useCallback(() => {
    if (pendingQueueScrollFrameRef.current !== null) {
      cancelAnimationFrame(pendingQueueScrollFrameRef.current);
      pendingQueueScrollFrameRef.current = null;
    }
    pendingQueueDragYRef.current = null;
  }, []);

  const runPendingQueueAutoScroll = useCallback(() => {
    const container = pendingQueueScrollRef.current;
    const pointerY = pendingQueueDragYRef.current;
    if (!container || pointerY === null || !draggingMessageIdRef.current) {
      pendingQueueScrollFrameRef.current = null;
      return;
    }

    const rect = container.getBoundingClientRect();
    const distanceToTop = pointerY - rect.top;
    const distanceToBottom = rect.bottom - pointerY;
    let delta = 0;

    if (distanceToTop < PENDING_QUEUE_AUTO_SCROLL_ZONE_PX) {
      const ratio =
        (PENDING_QUEUE_AUTO_SCROLL_ZONE_PX - Math.max(distanceToTop, 0)) /
        PENDING_QUEUE_AUTO_SCROLL_ZONE_PX;
      delta = -Math.ceil(ratio * PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX);
    } else if (distanceToBottom < PENDING_QUEUE_AUTO_SCROLL_ZONE_PX) {
      const ratio =
        (PENDING_QUEUE_AUTO_SCROLL_ZONE_PX - Math.max(distanceToBottom, 0)) /
        PENDING_QUEUE_AUTO_SCROLL_ZONE_PX;
      delta = Math.ceil(ratio * PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX);
    }

    if (delta !== 0) {
      container.scrollTop += delta;
    }
    pendingQueueScrollFrameRef.current = requestAnimationFrame(
      runPendingQueueAutoScroll,
    );
  }, []);

  const startPendingQueueAutoScroll = useCallback((clientY: number) => {
    pendingQueueDragYRef.current = clientY;
    if (pendingQueueScrollFrameRef.current === null) {
      pendingQueueScrollFrameRef.current = requestAnimationFrame(
        runPendingQueueAutoScroll,
      );
    }
  }, [runPendingQueueAutoScroll]);

  useEffect(() => stopPendingQueueAutoScroll, [stopPendingQueueAutoScroll]);

  const removePendingMessage = useCallback(async (id: string) => {
    await onDeleteQueuedMessage?.(id);
  }, [onDeleteQueuedMessage]);

  const guidePendingMessage = useCallback(async (message: InputQueueItem) => {
    if (isQueueActionRunning) {
      return;
    }
    try {
      setIsQueueActionRunning(true);
      await onGuideQueuedMessage?.(message.id);
    } catch (error) {
      console.error("引导队列消息失败:", error);
    } finally {
      setIsQueueActionRunning(false);
    }
  }, [
    isQueueActionRunning,
    onGuideQueuedMessage,
  ]);

  if (inputQueueItems.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "border-b border-(--surface-canvas-border)",
        compact ? "px-2 pb-0.5 pt-1" : "px-3 pb-1 pt-1",
      )}
    >
      <div className="flex items-center justify-between gap-2 text-[10px] font-medium text-(--text-soft)">
        <span className="inline-flex items-center gap-1.5">
          {t("composer.pending_queue")}
          <span className="tabular-nums">{inputQueueItems.length}</span>
        </span>
        <button
          aria-label={isPendingQueueCollapsed ? t("composer.expand_pending_queue") : t("composer.collapse_pending_queue")}
          className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
          onClick={() => setIsPendingQueueCollapsed((current) => !current)}
          type="button"
        >
          {isPendingQueueCollapsed ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronUp className="h-3 w-3" />
          )}
        </button>
      </div>
      <div
        className={cn(
          "soft-scrollbar flex max-h-[112px] flex-col divide-y divide-(--divider-subtle-color) overflow-y-auto pr-1",
          isPendingQueueCollapsed ? "hidden" : "mt-0.5",
        )}
        onDragOver={(event) => {
          event.preventDefault();
          startPendingQueueAutoScroll(event.clientY);
        }}
        ref={pendingQueueScrollRef}
      >
        {inputQueueItems.map((message) => {
          const isDragging = draggingMessageId === message.id;
          const isGuidanceWaiting = message.delivery_policy === "guide";
          const isDragTarget = Boolean(
            draggingMessageId &&
            draggingMessageId !== message.id &&
            dragOverMessageId === message.id,
          );
          return (
            <div
              key={message.id}
              draggable
              className={cn(
                "group -mx-1 flex min-h-7 items-center gap-2 px-1 py-0.5 text-(--text-default) transition-[background,box-shadow,opacity]",
                isDragging && "opacity-60",
                isDragTarget && "bg-(--surface-interactive-hover-background) shadow-[inset_3px_0_0_var(--primary)]",
              )}
              onDragOver={(event) => {
                event.preventDefault();
                startPendingQueueAutoScroll(event.clientY);
                if (dragOverMessageId !== message.id) {
                  setDragOverMessageId(message.id);
                }
              }}
              onDragStart={() => {
                draggingMessageIdRef.current = message.id;
                setDraggingMessageId(message.id);
              }}
              onDragEnd={() => {
                draggingMessageIdRef.current = null;
                stopPendingQueueAutoScroll();
                setDraggingMessageId(null);
                setDragOverMessageId(null);
              }}
              onDrop={(event) => {
                event.preventDefault();
                if (!draggingMessageId) {
                  return;
                }
                const nextItems = reorderPendingMessages(
                  inputQueueItems,
                  draggingMessageId,
                  message.id,
                );
                void onReorderQueueMessages?.(nextItems.map((item) => item.id));
                draggingMessageIdRef.current = null;
                stopPendingQueueAutoScroll();
                setDraggingMessageId(null);
                setDragOverMessageId(null);
              }}
            >
              <span
                aria-label={t("composer.drag_to_reorder")}
                className="inline-flex h-5 w-3.5 shrink-0 cursor-grab items-center justify-center text-(--text-soft) active:cursor-grabbing"
              >
                <GripVertical className="h-3.5 w-3.5" />
              </span>
              <p className="line-clamp-1 min-w-0 flex-1 text-[12px] leading-5 text-(--text-strong)">
                {message.content.trim() ? (
                  message.content
                ) : message.attachments && message.attachments.length > 0 ? (
                  <span className="inline-flex items-center gap-1 text-(--text-muted)">
                    <Paperclip className="h-3 w-3 shrink-0" />
                    {message.attachments.map((attachment) => attachment.file_name || attachment.workspace_path).join("、")}
                  </span>
                ) : null}
              </p>
              <button
                aria-label={isGuidanceWaiting ? t("composer.cancel_guidance") : t("composer.mark_guidance")}
                className="inline-flex h-6 shrink-0 items-center justify-center gap-1 px-1 text-[11px] font-semibold text-(--text-soft) transition-colors hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                disabled={isQueueActionRunning}
                onClick={() => {
                  void guidePendingMessage(message);
                }}
                type="button"
              >
                <CornerDownRight className="h-3 w-3" />
                {isGuidanceWaiting ? t("composer.cancel_guide_action") : t("composer.guide_action")}
              </button>
              <button
                aria-label={t("composer.delete_pending")}
                className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:text-(--destructive)"
                onClick={() => {
                  void removePendingMessage(message.id);
                }}
                type="button"
              >
                <Trash2 className="h-3 w-3" />
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

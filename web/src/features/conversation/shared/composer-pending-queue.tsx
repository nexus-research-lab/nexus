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

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { InputQueueItem } from "@/types/agent/agent-conversation";

const PENDING_QUEUE_AUTO_SCROLL_ZONE_PX = 28;
const PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX = 10;

function reorder_pending_messages(
  messages: InputQueueItem[],
  source_id: string,
  target_id: string,
): InputQueueItem[] {
  const source_index = messages.findIndex((item) => item.id === source_id);
  const target_index = messages.findIndex((item) => item.id === target_id);
  if (source_index < 0 || target_index < 0 || source_index === target_index) {
    return messages;
  }
  const next = [...messages];
  const [source] = next.splice(source_index, 1);
  next.splice(target_index, 0, source);
  return next;
}

export function ComposerPendingQueue({
  compact,
  disabled,
  input_queue_items,
  on_delete_queued_message,
  on_guide_queued_message,
  on_reorder_queue_messages,
}: {
  compact: boolean;
  disabled: boolean;
  input_queue_items: InputQueueItem[];
  on_delete_queued_message?: (item_id: string) => void | Promise<void>;
  on_guide_queued_message?: (item_id: string) => void | Promise<void>;
  on_reorder_queue_messages?: (ordered_ids: string[]) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const [dragging_message_id, set_dragging_message_id] = useState<string | null>(null);
  const [drag_over_message_id, set_drag_over_message_id] = useState<string | null>(null);
  const [is_pending_queue_collapsed, set_is_pending_queue_collapsed] = useState(false);
  const [is_queue_action_running, set_is_queue_action_running] = useState(false);
  const pending_queue_scroll_ref = useRef<HTMLDivElement>(null);
  const pending_queue_drag_y_ref = useRef<number | null>(null);
  const pending_queue_scroll_frame_ref = useRef<number | null>(null);
  const dragging_message_id_ref = useRef<string | null>(null);

  const stop_pending_queue_auto_scroll = useCallback(() => {
    if (pending_queue_scroll_frame_ref.current !== null) {
      cancelAnimationFrame(pending_queue_scroll_frame_ref.current);
      pending_queue_scroll_frame_ref.current = null;
    }
    pending_queue_drag_y_ref.current = null;
  }, []);

  const run_pending_queue_auto_scroll = useCallback(() => {
    const container = pending_queue_scroll_ref.current;
    const pointer_y = pending_queue_drag_y_ref.current;
    if (!container || pointer_y === null || !dragging_message_id_ref.current) {
      pending_queue_scroll_frame_ref.current = null;
      return;
    }

    const rect = container.getBoundingClientRect();
    const distance_to_top = pointer_y - rect.top;
    const distance_to_bottom = rect.bottom - pointer_y;
    let delta = 0;

    if (distance_to_top < PENDING_QUEUE_AUTO_SCROLL_ZONE_PX) {
      const ratio =
        (PENDING_QUEUE_AUTO_SCROLL_ZONE_PX - Math.max(distance_to_top, 0)) /
        PENDING_QUEUE_AUTO_SCROLL_ZONE_PX;
      delta = -Math.ceil(ratio * PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX);
    } else if (distance_to_bottom < PENDING_QUEUE_AUTO_SCROLL_ZONE_PX) {
      const ratio =
        (PENDING_QUEUE_AUTO_SCROLL_ZONE_PX - Math.max(distance_to_bottom, 0)) /
        PENDING_QUEUE_AUTO_SCROLL_ZONE_PX;
      delta = Math.ceil(ratio * PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX);
    }

    if (delta !== 0) {
      container.scrollTop += delta;
    }
    pending_queue_scroll_frame_ref.current = requestAnimationFrame(
      run_pending_queue_auto_scroll,
    );
  }, []);

  const start_pending_queue_auto_scroll = useCallback((client_y: number) => {
    pending_queue_drag_y_ref.current = client_y;
    if (pending_queue_scroll_frame_ref.current === null) {
      pending_queue_scroll_frame_ref.current = requestAnimationFrame(
        run_pending_queue_auto_scroll,
      );
    }
  }, [run_pending_queue_auto_scroll]);

  useEffect(() => stop_pending_queue_auto_scroll, [stop_pending_queue_auto_scroll]);

  const remove_pending_message = useCallback(async (id: string) => {
    await on_delete_queued_message?.(id);
  }, [on_delete_queued_message]);

  const guide_pending_message = useCallback(async (message: InputQueueItem) => {
    if (disabled || is_queue_action_running) {
      return;
    }
    try {
      set_is_queue_action_running(true);
      await on_guide_queued_message?.(message.id);
    } catch (error) {
      console.error("引导队列消息失败:", error);
    } finally {
      set_is_queue_action_running(false);
    }
  }, [
    disabled,
    is_queue_action_running,
    on_guide_queued_message,
  ]);

  if (input_queue_items.length === 0) {
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
          <span className="tabular-nums">{input_queue_items.length}</span>
        </span>
        <button
          aria-label={is_pending_queue_collapsed ? t("composer.expand_pending_queue") : t("composer.collapse_pending_queue")}
          className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
          onClick={() => set_is_pending_queue_collapsed((current) => !current)}
          type="button"
        >
          {is_pending_queue_collapsed ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronUp className="h-3 w-3" />
          )}
        </button>
      </div>
      <div
        className={cn(
          "soft-scrollbar flex max-h-[112px] flex-col divide-y divide-(--divider-subtle-color) overflow-y-auto pr-1",
          is_pending_queue_collapsed ? "hidden" : "mt-0.5",
        )}
        onDragOver={(event) => {
          event.preventDefault();
          start_pending_queue_auto_scroll(event.clientY);
        }}
        ref={pending_queue_scroll_ref}
      >
        {input_queue_items.map((message) => {
          const is_dragging = dragging_message_id === message.id;
          const is_guidance_waiting = message.delivery_policy === "guide";
          const is_drag_target = Boolean(
            dragging_message_id &&
            dragging_message_id !== message.id &&
            drag_over_message_id === message.id,
          );
          return (
            <div
              key={message.id}
              draggable
              className={cn(
                "group -mx-1 flex min-h-7 items-center gap-2 px-1 py-0.5 text-(--text-default) transition-[background,box-shadow,opacity]",
                is_dragging && "opacity-60",
                is_drag_target && "bg-(--surface-interactive-hover-background) shadow-[inset_3px_0_0_var(--primary)]",
              )}
              onDragOver={(event) => {
                event.preventDefault();
                start_pending_queue_auto_scroll(event.clientY);
                if (drag_over_message_id !== message.id) {
                  set_drag_over_message_id(message.id);
                }
              }}
              onDragStart={() => {
                dragging_message_id_ref.current = message.id;
                set_dragging_message_id(message.id);
              }}
              onDragEnd={() => {
                dragging_message_id_ref.current = null;
                stop_pending_queue_auto_scroll();
                set_dragging_message_id(null);
                set_drag_over_message_id(null);
              }}
              onDrop={(event) => {
                event.preventDefault();
                if (!dragging_message_id) {
                  return;
                }
                const next_items = reorder_pending_messages(
                  input_queue_items,
                  dragging_message_id,
                  message.id,
                );
                void on_reorder_queue_messages?.(next_items.map((item) => item.id));
                dragging_message_id_ref.current = null;
                stop_pending_queue_auto_scroll();
                set_dragging_message_id(null);
                set_drag_over_message_id(null);
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
                aria-label={is_guidance_waiting ? t("composer.cancel_guidance") : t("composer.mark_guidance")}
                className="inline-flex h-6 shrink-0 items-center justify-center gap-1 px-1 text-[11px] font-semibold text-(--text-soft) transition-colors hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                disabled={disabled || is_queue_action_running}
                onClick={() => {
                  void guide_pending_message(message);
                }}
                type="button"
              >
                <CornerDownRight className="h-3 w-3" />
                {is_guidance_waiting ? t("composer.cancel_guide_action") : t("composer.guide_action")}
              </button>
              <button
                aria-label={t("composer.delete_pending")}
                className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:text-(--destructive)"
                onClick={() => {
                  void remove_pending_message(message.id);
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

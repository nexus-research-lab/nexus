"use client";

import { MouseEvent, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { Plus, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { RoomConversationView } from "@/types/conversation/conversation";

interface WorkspaceConversationTabsProps {
  conversations: RoomConversationView[];
  conversation_id: string | null;
  tour_anchor?: string;
  on_select_conversation: (conversation_id: string) => void;
  on_close_conversation?: (conversation_id: string) => Promise<void>;
  on_create_conversation?: (title?: string) => Promise<string | null>;
}

const CONVERSATION_TAB_BASE_CLASS_NAME =
  "group relative inline-flex h-6.5 flex-none items-center overflow-hidden rounded-[13px] border text-[11px] font-semibold transition-[width,background-color,border-color,color,box-shadow] duration-[145ms] ease-[cubic-bezier(0.25,0.1,0.25,1)]";

const CONVERSATION_TAB_TRACK_CLASS_NAME =
  "soft-scrollbar scrollbar-hide flex h-[30px] w-full min-w-0 items-center gap-0 overflow-x-auto rounded-[15px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_52%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_86%,rgba(117,131,149,0.32))] px-px py-px shadow-[inset_0_1px_1px_rgba(15,23,42,0.05),inset_0_-1px_0_rgba(255,255,255,0.62)]";

const CREATE_CONVERSATION_BUTTON_SPACE = 88;
const TRACK_HORIZONTAL_PADDING = 2;
const ACTIVE_TAB_MIN_WIDTH = 142;
const INACTIVE_TAB_MIN_WIDTH = 92;
const ACTIVE_TAB_WIDTH_WEIGHT = 1.32;

function get_conversation_ids_by_activity(conversations: RoomConversationView[]): string[] {
  return [...conversations]
    .sort((left, right) => {
      if (left.last_activity_at !== right.last_activity_at) {
        return right.last_activity_at - left.last_activity_at;
      }
      return left.conversation_id.localeCompare(right.conversation_id);
    })
    .map((conversation) => conversation.conversation_id);
}

function are_conversation_ids_equal(left_ids: string[], right_ids: string[]): boolean {
  if (left_ids.length !== right_ids.length) {
    return false;
  }
  return left_ids.every((id, index) => id === right_ids[index]);
}

function get_initial_open_conversation_ids(
  conversation_id: string | null,
  recent_conversation_ids: string[],
): string[] {
  if (conversation_id && recent_conversation_ids.includes(conversation_id)) {
    return [conversation_id];
  }
  return recent_conversation_ids[0] ? [recent_conversation_ids[0]] : [];
}

function calculate_filled_tab_widths({
  active_conversation_id,
  has_create_button,
  ordered_conversations,
  track_width,
}: {
  active_conversation_id: string | null;
  has_create_button: boolean;
  ordered_conversations: RoomConversationView[];
  track_width: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  if (!track_width || ordered_conversations.length === 0) {
    return widths;
  }

  const available_width = Math.max(
    0,
    track_width -
      TRACK_HORIZONTAL_PADDING -
      (has_create_button ? CREATE_CONVERSATION_BUTTON_SPACE : 0),
  );

  if (ordered_conversations.length === 1) {
    widths.set(
      ordered_conversations[0].conversation_id,
      Math.max(ACTIVE_TAB_MIN_WIDTH, available_width),
    );
    return widths;
  }

  const inactive_count = ordered_conversations.length - 1;
  const minimum_total_width = ACTIVE_TAB_MIN_WIDTH + INACTIVE_TAB_MIN_WIDTH * inactive_count;
  let active_width = ACTIVE_TAB_MIN_WIDTH;
  let inactive_width = INACTIVE_TAB_MIN_WIDTH;

  if (available_width > minimum_total_width) {
    const weighted_unit_width = available_width / (inactive_count + ACTIVE_TAB_WIDTH_WEIGHT);
    const maximum_active_width = available_width - INACTIVE_TAB_MIN_WIDTH * inactive_count;
    active_width = Math.min(
      maximum_active_width,
      Math.max(ACTIVE_TAB_MIN_WIDTH, weighted_unit_width * ACTIVE_TAB_WIDTH_WEIGHT),
    );
    inactive_width = (available_width - active_width) / inactive_count;
  }

  ordered_conversations.forEach((conversation) => {
    widths.set(
      conversation.conversation_id,
      conversation.conversation_id === active_conversation_id ? active_width : inactive_width,
    );
  });

  return widths;
}

export function WorkspaceConversationTabs({
  conversations,
  conversation_id,
  tour_anchor,
  on_select_conversation,
  on_close_conversation,
  on_create_conversation,
}: WorkspaceConversationTabsProps) {
  const { t } = useI18n();
  const track_ref = useRef<HTMLElement | null>(null);
  const [track_width, set_track_width] = useState(0);
  const [is_creating, set_is_creating] = useState(false);
  const [hovered_conversation_id, set_hovered_conversation_id] = useState<string | null>(null);
  const [optimistic_active_conversation_id, set_optimistic_active_conversation_id] = useState<string | null>(null);
  const [pending_closed_active_conversation_id, set_pending_closed_active_conversation_id] = useState<string | null>(null);
  const recent_conversation_ids = useMemo(
    () => get_conversation_ids_by_activity(conversations),
    [conversations],
  );
  const [open_conversation_ids, set_open_conversation_ids] = useState<string[]>(() => (
    get_initial_open_conversation_ids(conversation_id, recent_conversation_ids)
  ));
  const conversations_by_id = useMemo(
    () => new Map(conversations.map((conversation) => [conversation.conversation_id, conversation])),
    [conversations],
  );
  const ordered_conversations = useMemo(
    () => open_conversation_ids
      .map((id) => conversations_by_id.get(id))
      .filter((conversation): conversation is RoomConversationView => Boolean(conversation)),
    [conversations_by_id, open_conversation_ids],
  );
  const optimistic_active_conversation_exists = Boolean(
    optimistic_active_conversation_id &&
      ordered_conversations.some((conversation) => (
        conversation.conversation_id === optimistic_active_conversation_id
      )),
  );
  const active_conversation_id = optimistic_active_conversation_exists
    ? optimistic_active_conversation_id
    : conversation_id;
  const tab_widths = useMemo(() => (
    calculate_filled_tab_widths({
      active_conversation_id,
      has_create_button: Boolean(on_create_conversation),
      ordered_conversations,
      track_width,
    })
  ), [
    active_conversation_id,
    on_create_conversation,
    ordered_conversations,
    track_width,
  ]);

  useLayoutEffect(() => {
    const track_element = track_ref.current;
    if (!track_element) {
      return undefined;
    }

    const update_track_width = () => {
      set_track_width((current_width) => {
        const next_width = track_element.clientWidth;
        return current_width === next_width ? current_width : next_width;
      });
    };

    update_track_width();
    const resize_observer = new ResizeObserver(update_track_width);
    resize_observer.observe(track_element);
    return () => resize_observer.disconnect();
  }, []);

  useEffect(() => {
    // 会话标签页采用浏览器模型：进入默认只打开当前会话，历史列表点击后再显式加入。
    const live_ids = new Set(recent_conversation_ids);
    const active_id = conversation_id && live_ids.has(conversation_id)
      ? conversation_id
      : null;
    const fallback_id = active_id ?? recent_conversation_ids[0] ?? null;

    set_open_conversation_ids((current_ids) => {
      let next_ids = current_ids.filter((id) => live_ids.has(id));
      if (
        active_id &&
        active_id !== pending_closed_active_conversation_id &&
        !next_ids.includes(active_id)
      ) {
        next_ids = [...next_ids, active_id];
      }
      if (next_ids.length === 0 && fallback_id) {
        next_ids = [fallback_id];
      }
      return are_conversation_ids_equal(current_ids, next_ids) ? current_ids : next_ids;
    });
  }, [
    conversation_id,
    pending_closed_active_conversation_id,
    recent_conversation_ids,
  ]);

  useEffect(() => {
    set_pending_closed_active_conversation_id((current_id) => (
      current_id && current_id !== conversation_id ? null : current_id
    ));
  }, [conversation_id]);

  useEffect(() => {
    set_optimistic_active_conversation_id((current_id) => {
      if (!current_id || current_id === conversation_id || !conversations_by_id.has(current_id)) {
        return null;
      }
      return current_id;
    });
  }, [conversation_id, conversations_by_id]);

  const handle_create_conversation = async () => {
    if (!on_create_conversation || is_creating) {
      return;
    }

    set_is_creating(true);
    try {
      await on_create_conversation();
    } finally {
      set_is_creating(false);
    }
  };

  const commit_optimistic_active_conversation = (next_conversation_id: string) => {
    if (next_conversation_id === active_conversation_id) {
      return;
    }
    flushSync(() => {
      set_optimistic_active_conversation_id(next_conversation_id);
    });
  };

  const handle_close_conversation_tab = (
    event: MouseEvent<HTMLButtonElement>,
    target_conversation_id: string,
  ) => {
    event.stopPropagation();

    if (ordered_conversations.length <= 1) {
      return;
    }

    const visible_ids = ordered_conversations.map((conversation) => conversation.conversation_id);
    const target_index = visible_ids.indexOf(target_conversation_id);
    const next_active_id = target_index >= 0
      ? visible_ids[target_index + 1] ?? visible_ids[target_index - 1] ?? null
      : null;

    set_open_conversation_ids((current_ids) => (
      current_ids.filter((id) => id !== target_conversation_id)
    ));
    if (target_conversation_id === active_conversation_id) {
      set_pending_closed_active_conversation_id(target_conversation_id);
      if (next_active_id) {
        commit_optimistic_active_conversation(next_active_id);
        on_select_conversation(next_active_id);
      }
    }
    if (on_close_conversation) {
      void on_close_conversation(target_conversation_id).catch(() => undefined);
    }
  };

  return (
    <nav
      aria-label={t("room.session_tabs_label")}
      className={CONVERSATION_TAB_TRACK_CLASS_NAME}
      data-tour-anchor={tour_anchor}
      ref={track_ref}
    >
      {on_create_conversation ? (
        <button
          aria-label={t("room.new_conversation")}
          className="relative mr-1 inline-flex h-6.5 w-[84px] shrink-0 items-center justify-start rounded-[13px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_78%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_74%,rgba(255,255,255,0.58))] pl-[22px] pr-2 text-left text-[11px] font-semibold leading-none text-(--text-default) shadow-[inset_0_1px_0_rgba(255,255,255,0.68)] transition-[background-color,border-color,color,box-shadow] duration-(--motion-duration-fast) ease-out hover:border-[color:color-mix(in_srgb,var(--success)_42%,var(--divider-subtle-color)_58%)] hover:bg-(--surface-interactive-hover-background) hover:text-(--success) hover:shadow-[0_1px_2px_rgba(15,23,42,0.08),inset_0_1px_0_rgba(255,255,255,0.76)] disabled:opacity-60"
          disabled={is_creating}
          onClick={() => {
            void handle_create_conversation();
          }}
          title={t("room.new_conversation")}
          type="button"
        >
          <Plus className={cn("absolute left-[7px] top-1/2 h-3 w-3 -translate-y-1/2", is_creating && "animate-spin")} />
          <span className="min-w-0 truncate">{t("room.new_conversation")}</span>
        </button>
      ) : null}

      {ordered_conversations.map((conversation, conversation_index) => {
        const is_active = conversation.conversation_id === active_conversation_id;
        const is_hovered = conversation.conversation_id === hovered_conversation_id;
        const previous_conversation = ordered_conversations[conversation_index - 1];
        const is_previous_highlighted =
          Boolean(previous_conversation) &&
          (
            previous_conversation.conversation_id === active_conversation_id ||
            previous_conversation.conversation_id === hovered_conversation_id
          );
        const should_show_separator = conversation_index > 0 && !is_active && !is_hovered && !is_previous_highlighted;
        const title = conversation.title?.trim() || t("room.untitled_conversation");
        const tab_width = tab_widths.get(conversation.conversation_id);

        return (
          <div
            className={cn(
              CONVERSATION_TAB_BASE_CLASS_NAME,
              is_active
                ? "z-10 border-[color:color-mix(in_srgb,var(--divider-subtle-color)_88%,transparent)] bg-(--surface-interactive-active-background) text-(--text-strong) shadow-[0_0_3px_rgba(15,23,42,0.12),inset_0_1px_0_rgba(255,255,255,0.82)] hover:border-transparent hover:bg-(--surface-interactive-hover-background) hover:shadow-[0_1px_2px_rgba(15,23,42,0.08),inset_0_1px_0_rgba(255,255,255,0.66)]"
                : "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) hover:shadow-[0_1px_2px_rgba(15,23,42,0.08),inset_0_1px_0_rgba(255,255,255,0.66)]",
              should_show_separator &&
                "before:pointer-events-none before:absolute before:left-0 before:top-1.5 before:bottom-1.5 before:w-px before:bg-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] before:content-['']",
            )}
            key={conversation.conversation_id}
            onMouseEnter={() => set_hovered_conversation_id(conversation.conversation_id)}
            onMouseLeave={() => {
              set_hovered_conversation_id((current_id) => (
                current_id === conversation.conversation_id ? null : current_id
              ));
            }}
            style={{
              minWidth: is_active ? ACTIVE_TAB_MIN_WIDTH : INACTIVE_TAB_MIN_WIDTH,
              width: tab_width ?? (is_active ? ACTIVE_TAB_MIN_WIDTH : INACTIVE_TAB_MIN_WIDTH),
            }}
            title={title}
          >
            <button
              aria-current={is_active ? "page" : undefined}
              aria-pressed={is_active}
              className="flex h-full w-full min-w-0 items-center justify-start pl-[22px] pr-7 text-left"
              onClick={() => {
                commit_optimistic_active_conversation(conversation.conversation_id);
                on_select_conversation(conversation.conversation_id);
              }}
              onPointerDown={(event) => {
                if (event.button !== 0) {
                  return;
                }
                commit_optimistic_active_conversation(conversation.conversation_id);
              }}
              type="button"
            >
              <span
                aria-hidden="true"
                className={cn(
                  "absolute left-2.5 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast)",
                  is_active
                    ? "bg-(--primary) shadow-[0_0_0_2px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
                    : "border border-[color:color-mix(in_srgb,var(--icon-muted)_72%,transparent)] bg-transparent group-hover:border-(--icon-default) group-hover:bg-[color:color-mix(in_srgb,var(--icon-default)_28%,transparent)]",
                )}
              />
              <span className="min-w-0 truncate">{title}</span>
            </button>
            {ordered_conversations.length > 1 ? (
              <button
                aria-label={t("room.close_conversation")}
                className={cn(
                  "absolute right-1 top-1/2 flex h-5 w-5 shrink-0 -translate-y-1/2 items-center justify-center rounded-full text-(--icon-muted) transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)",
                  is_active ? "opacity-100" : "opacity-70 group-hover:opacity-100",
                )}
                onClick={(event) => {
                  handle_close_conversation_tab(event, conversation.conversation_id);
                }}
                title={t("room.close_conversation")}
                type="button"
              >
                <X className="h-3 w-3" />
              </button>
            ) : null}
          </div>
        );
      })}
    </nav>
  );
}

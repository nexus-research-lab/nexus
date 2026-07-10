"use client";

import { MouseEvent, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { Plus, X } from "lucide-react";

import { getSessionChannelLabel } from "@/features/conversation/external-session-labels";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { RoomConversationView } from "@/types/conversation/conversation";

interface WorkspaceConversationTabsProps {
  conversations: RoomConversationView[];
  conversationId: string | null;
  tourAnchor?: string;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
}

const CONVERSATION_TAB_BASE_CLASS_NAME =
  "group relative inline-flex h-6.5 flex-none items-center overflow-hidden rounded-[13px] border text-[11px] font-semibold transition-[width,background-color,border-color,color,box-shadow] duration-[145ms] ease-[cubic-bezier(0.25,0.1,0.25,1)]";

const CONVERSATION_TAB_TRACK_CLASS_NAME =
  "soft-scrollbar scrollbar-hide flex h-[30px] w-full min-w-0 items-center gap-0 overflow-x-auto rounded-[15px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_66%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] px-px py-px shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)]";

const CREATE_CONVERSATION_BUTTON_SPACE = 88;
const TRACK_HORIZONTAL_PADDING = 2;
const ACTIVE_TAB_MIN_WIDTH = 142;
const INACTIVE_TAB_MIN_WIDTH = 92;
const ACTIVE_TAB_WIDTH_WEIGHT = 1.32;

function getConversationIdsByActivity(conversations: RoomConversationView[]): string[] {
  return [...conversations]
    .sort((left, right) => {
      if (left.last_activity_at !== right.last_activity_at) {
        return right.last_activity_at - left.last_activity_at;
      }
      return left.conversation_id.localeCompare(right.conversation_id);
    })
    .map((conversation) => conversation.conversation_id);
}

function areConversationIdsEqual(leftIds: string[], rightIds: string[]): boolean {
  if (leftIds.length !== rightIds.length) {
    return false;
  }
  return leftIds.every((id, index) => id === rightIds[index]);
}

function isExternalSessionConversation(conversation?: RoomConversationView): boolean {
  return conversation?.options?.external_session === true;
}

function stringOption(options: Record<string, unknown>, key: string): string | null {
  const value = options[key];
  return typeof value === "string" ? value : null;
}

function getExternalSessionLabel(conversation: RoomConversationView): string | null {
  if (!isExternalSessionConversation(conversation)) {
    return null;
  }
  return getSessionChannelLabel(
    stringOption(conversation.options, "channel_type"),
    conversation.session_key,
  );
}

function getInitialOpenConversationIds(
  conversationId: string | null,
  recentConversationIds: string[],
): string[] {
  if (conversationId && recentConversationIds.includes(conversationId)) {
    return [conversationId];
  }
  return recentConversationIds[0] ? [recentConversationIds[0]] : [];
}

function calculateFilledTabWidths({
  active_conversation_id: activeConversationId,
  has_create_button: hasCreateButton,
  ordered_conversations: orderedConversations,
  track_width: trackWidth,
}: {
  active_conversation_id: string | null;
  has_create_button: boolean;
  ordered_conversations: RoomConversationView[];
  track_width: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  if (!trackWidth || orderedConversations.length === 0) {
    return widths;
  }

  const availableWidth = Math.max(
    0,
    trackWidth -
      TRACK_HORIZONTAL_PADDING -
      (hasCreateButton ? CREATE_CONVERSATION_BUTTON_SPACE : 0),
  );

  if (orderedConversations.length === 1) {
    widths.set(
      orderedConversations[0].conversation_id,
      Math.max(ACTIVE_TAB_MIN_WIDTH, availableWidth),
    );
    return widths;
  }

  const inactiveCount = orderedConversations.length - 1;
  const minimumTotalWidth = ACTIVE_TAB_MIN_WIDTH + INACTIVE_TAB_MIN_WIDTH * inactiveCount;
  let activeWidth = ACTIVE_TAB_MIN_WIDTH;
  let inactiveWidth = INACTIVE_TAB_MIN_WIDTH;

  if (availableWidth > minimumTotalWidth) {
    const weightedUnitWidth = availableWidth / (inactiveCount + ACTIVE_TAB_WIDTH_WEIGHT);
    const maximumActiveWidth = availableWidth - INACTIVE_TAB_MIN_WIDTH * inactiveCount;
    activeWidth = Math.min(
      maximumActiveWidth,
      Math.max(ACTIVE_TAB_MIN_WIDTH, weightedUnitWidth * ACTIVE_TAB_WIDTH_WEIGHT),
    );
    inactiveWidth = (availableWidth - activeWidth) / inactiveCount;
  }

  orderedConversations.forEach((conversation) => {
    widths.set(
      conversation.conversation_id,
      conversation.conversation_id === activeConversationId ? activeWidth : inactiveWidth,
    );
  });

  return widths;
}

export function WorkspaceConversationTabs({
  conversations,
  conversationId: conversationId,
  tourAnchor: tourAnchor,
  onSelectConversation: onSelectConversation,
  onCloseConversation: onCloseConversation,
  onCreateConversation: onCreateConversation,
}: WorkspaceConversationTabsProps) {
  const { t } = useI18n();
  const trackRef = useRef<HTMLElement | null>(null);
  const [trackWidth, setTrackWidth] = useState(0);
  const [isCreating, setIsCreating] = useState(false);
  const [hoveredConversationId, setHoveredConversationId] = useState<string | null>(null);
  const [optimisticActiveConversationId, setOptimisticActiveConversationId] = useState<string | null>(null);
  const [pendingClosedActiveConversationId, setPendingClosedActiveConversationId] = useState<string | null>(null);
  const recentConversationIds = useMemo(
    () => getConversationIdsByActivity(conversations),
    [conversations],
  );
  const [openConversationIds, setOpenConversationIds] = useState<string[]>(() => (
    getInitialOpenConversationIds(conversationId, recentConversationIds)
  ));
  const conversationsById = useMemo(
    () => new Map(conversations.map((conversation) => [conversation.conversation_id, conversation])),
    [conversations],
  );
  const orderedConversations = useMemo(
    () => openConversationIds
      .map((id) => conversationsById.get(id))
      .filter((conversation): conversation is RoomConversationView => Boolean(conversation)),
    [conversationsById, openConversationIds],
  );
  const optimisticActiveConversationExists = Boolean(
    optimisticActiveConversationId &&
      orderedConversations.some((conversation) => (
        conversation.conversation_id === optimisticActiveConversationId
      )),
  );
  const activeConversationId = optimisticActiveConversationExists
    ? optimisticActiveConversationId
    : conversationId;
  const tabWidths = useMemo(() => (
    calculateFilledTabWidths({
      active_conversation_id: activeConversationId,
      has_create_button: Boolean(onCreateConversation),
      ordered_conversations: orderedConversations,
      track_width: trackWidth,
    })
  ), [
    activeConversationId,
    onCreateConversation,
    orderedConversations,
    trackWidth,
  ]);

  useLayoutEffect(() => {
    const trackElement = trackRef.current;
    if (!trackElement) {
      return undefined;
    }

    const updateTrackWidth = () => {
      setTrackWidth((currentWidth) => {
        const nextWidth = trackElement.clientWidth;
        return currentWidth === nextWidth ? currentWidth : nextWidth;
      });
    };

    updateTrackWidth();
    const resizeObserver = new ResizeObserver(updateTrackWidth);
    resizeObserver.observe(trackElement);
    return () => resizeObserver.disconnect();
  }, []);

  useEffect(() => {
    // 会话标签页采用浏览器模型：进入默认只打开当前会话，历史列表点击后再显式加入。
    const liveIds = new Set(recentConversationIds);
    const activeId = conversationId && liveIds.has(conversationId)
      ? conversationId
      : null;
    const fallbackId = activeId ?? recentConversationIds[0] ?? null;

    setOpenConversationIds((currentIds) => {
      let nextIds = currentIds.filter((id) => liveIds.has(id));
      if (
        activeId &&
        activeId !== pendingClosedActiveConversationId &&
        !nextIds.includes(activeId)
      ) {
        nextIds = [...nextIds, activeId];
      }
      if (nextIds.length === 0 && fallbackId) {
        nextIds = [fallbackId];
      }
      return areConversationIdsEqual(currentIds, nextIds) ? currentIds : nextIds;
    });
  }, [
    conversationId,
    pendingClosedActiveConversationId,
    recentConversationIds,
  ]);

  useEffect(() => {
    setPendingClosedActiveConversationId((currentId) => (
      currentId && currentId !== conversationId ? null : currentId
    ));
  }, [conversationId]);

  useEffect(() => {
    setOptimisticActiveConversationId((currentId) => {
      if (!currentId || currentId === conversationId || !conversationsById.has(currentId)) {
        return null;
      }
      return currentId;
    });
  }, [conversationId, conversationsById]);

  const handleCreateConversation = async () => {
    if (!onCreateConversation || isCreating) {
      return;
    }

    setIsCreating(true);
    try {
      await onCreateConversation();
    } finally {
      setIsCreating(false);
    }
  };

  const commitOptimisticActiveConversation = (nextConversationId: string) => {
    if (nextConversationId === activeConversationId) {
      return;
    }
    flushSync(() => {
      setOptimisticActiveConversationId(nextConversationId);
    });
  };

  const handleCloseConversationTab = (
    event: MouseEvent<HTMLButtonElement>,
    targetConversationId: string,
  ) => {
    event.stopPropagation();

    if (orderedConversations.length <= 1) {
      return;
    }

    const visibleIds = orderedConversations.map((conversation) => conversation.conversation_id);
    const targetIndex = visibleIds.indexOf(targetConversationId);
    const nextActiveId = targetIndex >= 0
      ? visibleIds[targetIndex + 1] ?? visibleIds[targetIndex - 1] ?? null
      : null;

    setOpenConversationIds((currentIds) => (
      currentIds.filter((id) => id !== targetConversationId)
    ));
    if (targetConversationId === activeConversationId) {
      setPendingClosedActiveConversationId(targetConversationId);
      if (nextActiveId) {
        commitOptimisticActiveConversation(nextActiveId);
        onSelectConversation(nextActiveId);
      }
    }
    const targetConversation = conversationsById.get(targetConversationId);
    if (onCloseConversation && !isExternalSessionConversation(targetConversation)) {
      void onCloseConversation(targetConversationId).catch(() => undefined);
    }
  };

  return (
    <nav
      aria-label={t("room.session_tabs_label")}
      className={CONVERSATION_TAB_TRACK_CLASS_NAME}
      data-tour-anchor={tourAnchor}
      ref={trackRef}
    >
      {orderedConversations.map((conversation, conversationIndex) => {
        const isActive = conversation.conversation_id === activeConversationId;
        const isHovered = conversation.conversation_id === hoveredConversationId;
        const previousConversation = orderedConversations[conversationIndex - 1];
        const isPreviousHighlighted =
          Boolean(previousConversation) &&
          (
            previousConversation.conversation_id === activeConversationId ||
            previousConversation.conversation_id === hoveredConversationId
          );
        const shouldShowSeparator = conversationIndex > 0 && !isActive && !isHovered && !isPreviousHighlighted;
        const title = conversation.title?.trim() || t("room.untitled_conversation");
        const externalSessionLabel = getExternalSessionLabel(conversation);
        const tabWidth = tabWidths.get(conversation.conversation_id);

        return (
          <div
            className={cn(
              CONVERSATION_TAB_BASE_CLASS_NAME,
              isActive
                ? "z-10 border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color)_82%)] bg-(--surface-interactive-active-background) text-(--text-strong) shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_6%,transparent)] hover:border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] hover:bg-(--surface-interactive-hover-background) hover:shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)]"
                : "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) hover:shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)]",
              shouldShowSeparator &&
                "before:pointer-events-none before:absolute before:left-0 before:top-1.5 before:bottom-1.5 before:w-px before:bg-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] before:content-['']",
            )}
            key={conversation.conversation_id}
            onMouseEnter={() => setHoveredConversationId(conversation.conversation_id)}
            onMouseLeave={() => {
              setHoveredConversationId((currentId) => (
                currentId === conversation.conversation_id ? null : currentId
              ));
            }}
            style={{
              minWidth: isActive ? ACTIVE_TAB_MIN_WIDTH : INACTIVE_TAB_MIN_WIDTH,
              width: tabWidth ?? (isActive ? ACTIVE_TAB_MIN_WIDTH : INACTIVE_TAB_MIN_WIDTH),
            }}
            title={externalSessionLabel ? `${title} · IM ${externalSessionLabel}` : title}
          >
            <button
              aria-current={isActive ? "page" : undefined}
              aria-pressed={isActive}
              className="flex h-full w-full min-w-0 items-center justify-start pl-[22px] pr-7 text-left"
              onClick={() => {
                commitOptimisticActiveConversation(conversation.conversation_id);
                onSelectConversation(conversation.conversation_id);
              }}
              onPointerDown={(event) => {
                if (event.button !== 0) {
                  return;
                }
                commitOptimisticActiveConversation(conversation.conversation_id);
              }}
              type="button"
            >
              <span
                aria-hidden="true"
                className={cn(
                  "absolute left-2.5 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast)",
                  isActive
                    ? "bg-(--primary) shadow-[0_0_0_2px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
                    : "border border-[color:color-mix(in_srgb,var(--icon-muted)_72%,transparent)] bg-transparent group-hover:border-(--icon-default) group-hover:bg-[color:color-mix(in_srgb,var(--icon-default)_28%,transparent)]",
                )}
              />
              <span className="min-w-0 truncate">{title}</span>
              {externalSessionLabel ? (
                <span className="ml-1 inline-flex shrink-0 items-center rounded-[5px] border border-[color:color-mix(in_srgb,var(--primary)_20%,transparent)] px-1 py-px text-[8.5px] font-bold leading-none text-(--primary)">
                  IM
                </span>
              ) : null}
            </button>
            {orderedConversations.length > 1 ? (
              <button
                aria-label={t("room.close_conversation")}
                className={cn(
                  "absolute right-1 top-1/2 flex h-5 w-5 shrink-0 -translate-y-1/2 items-center justify-center rounded-full text-(--icon-muted) transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)",
                  isActive ? "opacity-100" : "opacity-70 group-hover:opacity-100",
                )}
                onClick={(event) => {
                  handleCloseConversationTab(event, conversation.conversation_id);
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

      {onCreateConversation ? (
        <button
          aria-label={t("room.new_conversation")}
          className="relative ml-1 inline-flex h-6.5 w-[84px] shrink-0 items-center justify-start rounded-[13px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_76%,transparent)] pl-[22px] pr-2 text-left text-[11px] font-semibold leading-none text-(--text-default) shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)] transition-[background-color,border-color,color,box-shadow] duration-(--motion-duration-fast) ease-out hover:border-[color:color-mix(in_srgb,var(--success)_24%,var(--divider-subtle-color)_76%)] hover:bg-(--surface-interactive-hover-background) hover:text-(--success) hover:shadow-[inset_0_1px_0_color-mix(in_srgb,var(--success)_8%,transparent)] disabled:opacity-60"
          disabled={isCreating}
          onClick={() => {
            void handleCreateConversation();
          }}
          title={t("room.new_conversation")}
          type="button"
        >
          <Plus className={cn("absolute left-[7px] top-1/2 h-3 w-3 -translate-y-1/2", isCreating && "animate-spin")} />
          <span className="min-w-0 truncate">{t("room.new_conversation")}</span>
        </button>
      ) : null}
    </nav>
  );
}

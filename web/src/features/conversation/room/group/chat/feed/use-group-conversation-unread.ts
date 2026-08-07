/**
 * INPUT: 当前 Room 稳定节点、精确完成消息锚点、窗口加载器和共享轮次导航。
 * OUTPUT: 本批新消息起点标记、逐条已读消费、当前未读数量/方向及精确跳转动作。
 * POS: Room Feed 专属未读协调器；完成锚点留在侧栏 Store，DM 保持原滚动行为。
 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
} from "react";

import {
  type ConversationRoundScrollHandleRef,
} from "@/features/conversation/shared/timeline/scroll/round-scroll";
import { buildChatNotificationTargetKey } from "@/features/home/notifications/chat-notification-target";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useSidebarStore } from "@/store/sidebar";

import type {
  GroupAgentTimelineProjection,
} from "./group-agent-timeline-model";
import {
  countUnreadAgentNodes,
  resolveGroupUnreadNodePosition,
  resolveStoredUnreadMessages,
  type GroupUnreadNodePosition,
} from "./group-conversation-unread-model";
import { useGroupConversationUnreadScroll } from "./use-group-conversation-unread-scroll";

export interface GroupConversationUnreadModel {
  direction: Exclude<GroupUnreadNodePosition, "visible"> | null;
  jumpToFirstUnread: () => void;
  markerRoundId: string | null;
  unreadCount: number;
}

interface UseGroupConversationUnreadOptions {
  conversationId: string | null;
  loadRoundWindow?: (roundId: string) => Promise<boolean>;
  pauseFollowLatest: () => void;
  roomId: string | null;
  roundScrollRef: ConversationRoundScrollHandleRef;
  scrollRef: React.RefObject<HTMLDivElement | null>;
  sessionKey: string | null;
  source: GroupAgentTimelineProjection;
}

export function useGroupConversationUnread({
  conversationId,
  loadRoundWindow,
  pauseFollowLatest,
  roomId,
  roundScrollRef,
  scrollRef,
  sessionKey,
  source,
}: UseGroupConversationUnreadOptions): GroupConversationUnreadModel {
  const scopeKey = sessionKey ?? "";
  const targetKeys = useMemo(
    () => buildUnreadTargetKeys(roomId, conversationId, sessionKey),
    [conversationId, roomId, sessionKey],
  );
  const storedAnchorMap = useSidebarStore(
    (state) => state.chat_unread_anchors,
  );
  const consumeUnreadMessages = useSidebarStore(
    (state) => state.consume_chat_unread_messages,
  );
  const storedAnchors = useMemo(
    () => targetKeys.flatMap((targetKey) => {
      const anchor = storedAnchorMap[targetKey];
      return anchor ? [anchor] : [];
    }),
    [storedAnchorMap, targetKeys],
  );
  const unreadMessages = useMemo(
    () => resolveStoredUnreadMessages(source, storedAnchors),
    [source, storedAnchors],
  );
  const [markerRoundId, setMarkerRoundId] = useResettableState<string | null>(
    null,
    scopeKey,
  );
  const [headPosition, setHeadPosition] =
    useResettableState<GroupUnreadNodePosition | null>(null, scopeKey);
  const [loadAttempt, setLoadAttempt] = useResettableState(
    0,
    `${scopeKey}:${unreadMessages[0]?.messageId ?? ""}`,
  );
  const unreadMessagesRef = useRef(unreadMessages);
  unreadMessagesRef.current = unreadMessages;
  const observedScopeRef = useRef<string | null>(null);
  const initialUnreadMessageIdRef = useRef<string | null>(null);
  const unreadBatchWasEmptyRef = useRef(true);

  if (observedScopeRef.current !== scopeKey) {
    observedScopeRef.current = scopeKey;
    initialUnreadMessageIdRef.current =
      unreadMessages[0]?.messageId ?? null;
    unreadBatchWasEmptyRef.current = true;
  }

  const roundIdsKey = source.roundIds.join("\u001f");
  const headMessage = unreadMessages[0] ?? null;
  const headRoundId = headMessage?.nodeId ?? null;
  const {
    cancelPendingPosition,
    scrollToUnreadNode,
  } = useGroupConversationUnreadScroll({
    pauseFollowLatest,
    roundIds: source.roundIds,
    roundScrollRef,
    scopeKey,
    scrollRef,
  });

  const resolveHeadPosition = useCallback((): GroupUnreadNodePosition | null => {
    const nodeId = unreadMessagesRef.current[0]?.nodeId;
    const scrollElement = scrollRef.current;
    if (!nodeId || !scrollElement) {
      return null;
    }
    return resolveGroupUnreadNodePosition(
      scrollElement,
      source.roundIds,
      nodeId,
    );
  }, [scrollRef, source.roundIds]);

  const consumeVisibleMessages = useCallback(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const visibleMessageIdsByTarget = new Map<string, string[]>();
    const positions = new Map<string, GroupUnreadNodePosition>();
    for (const message of unreadMessagesRef.current) {
      if (!message.nodeId) {
        continue;
      }
      const position = positions.get(message.nodeId)
        ?? resolveGroupUnreadNodePosition(
          scrollElement,
          source.roundIds,
          message.nodeId,
        );
      positions.set(message.nodeId, position);
      if (position !== "visible") {
        continue;
      }
      const messageIds = visibleMessageIdsByTarget.get(message.targetKey) ?? [];
      messageIds.push(message.messageId);
      visibleMessageIdsByTarget.set(message.targetKey, messageIds);
    }
    for (const [targetKey, messageIds] of visibleMessageIdsByTarget) {
      consumeUnreadMessages(targetKey, messageIds);
    }
  }, [consumeUnreadMessages, scrollRef, source.roundIds]);

  const syncUnreadState = useCallback(() => {
    const firstMessage = unreadMessagesRef.current[0];
    const position = resolveHeadPosition();
    setHeadPosition(position);
    if (firstMessage?.nodeId && position && position !== "visible") {
      setMarkerRoundId((current) => current ?? firstMessage.nodeId);
    }
    consumeVisibleMessages();
  }, [
    consumeVisibleMessages,
    resolveHeadPosition,
    setHeadPosition,
    setMarkerRoundId,
  ]);

  useEffect(() => {
    const firstMessage = unreadMessages[0];
    const rootRoundId = firstMessage?.rootRoundId;
    if (
      !firstMessage
      || firstMessage.nodeId
      || !rootRoundId
      || !loadRoundWindow
      || loadAttempt >= 3
    ) {
      return;
    }
    let cancelled = false;
    let retryTimer = 0;
    void loadRoundWindow(rootRoundId)
      .catch(() => false)
      .then(() => {
        if (cancelled) {
          return;
        }
        retryTimer = window.setTimeout(() => {
          setLoadAttempt((attempt) => attempt + 1);
        }, 320);
      });
    return () => {
      cancelled = true;
      window.clearTimeout(retryTimer);
    };
  }, [
    loadAttempt,
    loadRoundWindow,
    setLoadAttempt,
    unreadMessages,
  ]);

  useLayoutEffect(() => {
    if (unreadMessages.length === 0) {
      unreadBatchWasEmptyRef.current = true;
      return;
    }
    if (!unreadBatchWasEmptyRef.current) {
      return;
    }
    unreadBatchWasEmptyRef.current = false;
    setMarkerRoundId(null);
  }, [setMarkerRoundId, unreadMessages.length]);

  useLayoutEffect(() => {
    const firstMessage = unreadMessagesRef.current[0];
    if (
      !firstMessage
      || initialUnreadMessageIdRef.current !== firstMessage.messageId
    ) {
      syncUnreadState();
      return;
    }
    if (!firstMessage.nodeId) {
      setHeadPosition(null);
      return;
    }
    setMarkerRoundId((current) => current ?? firstMessage.nodeId);
    if (!scrollToUnreadNode(firstMessage.nodeId, "auto")) {
      syncUnreadState();
      return;
    }
    initialUnreadMessageIdRef.current = null;
    setHeadPosition("visible");
    const frame = window.requestAnimationFrame(consumeVisibleMessages);
    return () => window.cancelAnimationFrame(frame);
  }, [
    consumeVisibleMessages,
    headMessage,
    loadRoundWindow,
    roundIdsKey,
    scrollToUnreadNode,
    setHeadPosition,
    setMarkerRoundId,
    syncUnreadState,
  ]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    scrollElement.addEventListener("scroll", syncUnreadState, {
      passive: true,
    });
    return () => scrollElement.removeEventListener(
      "scroll",
      syncUnreadState,
    );
  }, [scrollRef, syncUnreadState]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const cancelInitialPosition = () => {
      initialUnreadMessageIdRef.current = null;
      cancelPendingPosition();
    };
    scrollElement.addEventListener("pointerdown", cancelInitialPosition, {
      passive: true,
    });
    scrollElement.addEventListener("touchstart", cancelInitialPosition, {
      passive: true,
    });
    scrollElement.addEventListener("wheel", cancelInitialPosition, {
      passive: true,
    });
    return () => {
      scrollElement.removeEventListener("pointerdown", cancelInitialPosition);
      scrollElement.removeEventListener("touchstart", cancelInitialPosition);
      scrollElement.removeEventListener("wheel", cancelInitialPosition);
    };
  }, [
    cancelPendingPosition,
    scrollRef,
    scopeKey,
  ]);

  useLayoutEffect(syncUnreadState, [
    headRoundId,
    roundIdsKey,
    syncUnreadState,
  ]);

  const jumpToFirstUnread = useCallback(() => {
    const nodeId = unreadMessagesRef.current[0]?.nodeId;
    if (!nodeId) {
      return;
    }
    scrollToUnreadNode(nodeId, "smooth");
  }, [scrollToUnreadNode]);

  return {
    direction: headPosition === "above" || headPosition === "below"
      ? headPosition
      : null,
    jumpToFirstUnread,
    markerRoundId,
    unreadCount: countUnreadAgentNodes(unreadMessages),
  };
}

function buildUnreadTargetKeys(
  roomId: string | null,
  conversationId: string | null,
  sessionKey: string | null,
): string[] {
  const candidates = [
    buildChatNotificationTargetKey({
      conversation_id: conversationId,
      room_id: roomId,
    }),
    buildChatNotificationTargetKey({ room_id: roomId }),
    buildChatNotificationTargetKey({ session_key: sessionKey }),
  ];
  return Array.from(new Set(candidates.filter(
    (candidate): candidate is string => Boolean(candidate),
  )));
}

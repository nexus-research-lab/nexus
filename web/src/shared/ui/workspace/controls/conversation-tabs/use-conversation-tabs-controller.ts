import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { flushSync } from "react-dom";

import { isExternalSessionConversation } from "@/lib/conversation/external-session";
import {
  calculateConversationTabWidths,
  getCloseFallbackConversationId,
  getConversationTabCapacity,
  getInitialOpenConversationIds,
  getRecentConversationIds,
  reconcileOpenConversationIds,
  resolveActiveConversationId,
} from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model";
import { RoomConversationView } from "@/types/conversation/conversation";

interface ConversationTabsControllerOptions {
  conversations: RoomConversationView[];
  conversationId: string | null;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
}

export function useConversationTabsController({
  conversations,
  conversationId,
  onCloseConversation,
  onCreateConversation,
  onSelectConversation,
}: ConversationTabsControllerOptions) {
  const trackRef = useRef<HTMLElement | null>(null);
  const [trackWidth, setTrackWidth] = useState(0);
  const [isCreating, setIsCreating] = useState(false);
  const [hoveredConversationId, setHoveredConversationId] = useState<string | null>(null);
  const [optimisticActiveId, setOptimisticActiveId] = useState<string | null>(null);
  const [pendingClosedActiveId, setPendingClosedActiveId] = useState<string | null>(null);
  const closedConversationIdsRef = useRef<Set<string>>(new Set());
  const knownRecentConversationIdsRef = useRef<Set<string>>(new Set());
  const previousTabCapacityRef = useRef(0);
  const hasCreateButton = Boolean(onCreateConversation);
  const recentConversationIds = useMemo(
    () => getRecentConversationIds(conversations),
    [conversations],
  );
  const [openConversationIds, setOpenConversationIds] = useState<string[]>(() => (
    getInitialOpenConversationIds(conversationId, recentConversationIds)
  ));
  const tabCapacity = useMemo(() => getConversationTabCapacity({
    hasCreateButton,
    trackWidth,
  }), [hasCreateButton, trackWidth]);
  const conversationsById = useMemo(
    () => new Map(
      conversations.map((conversation) => [conversation.conversation_id, conversation]),
    ),
    [conversations],
  );
  const orderedConversations = useMemo(
    () => openConversationIds
      .map((id) => conversationsById.get(id))
      .filter((conversation): conversation is RoomConversationView => Boolean(conversation)),
    [conversationsById, openConversationIds],
  );
  const activeConversationId = resolveActiveConversationId({
    conversationId,
    optimisticId: optimisticActiveId,
    orderedConversations,
  });
  const tabWidths = useMemo(() => calculateConversationTabWidths({
    activeConversationId,
    hasCreateButton,
    orderedConversations,
    trackWidth,
  }), [activeConversationId, hasCreateButton, orderedConversations, trackWidth]);

  useTrackWidth(trackRef, setTrackWidth);

  useEffect(() => {
    const previousTabCapacity = previousTabCapacityRef.current;
    const capacityIncreased = tabCapacity > previousTabCapacity;
    previousTabCapacityRef.current = tabCapacity;
    const liveConversationIds = new Set(recentConversationIds);
    const hasNewRecentConversation = recentConversationIds.some(
      (id) => !knownRecentConversationIdsRef.current.has(id),
    );
    knownRecentConversationIdsRef.current = liveConversationIds;
    for (const id of closedConversationIdsRef.current) {
      if (!liveConversationIds.has(id)) {
        closedConversationIdsRef.current.delete(id);
      }
    }
    if (conversationId && conversationId !== pendingClosedActiveId) {
      closedConversationIdsRef.current.delete(conversationId);
    }

    // 宽度增长或已打开会话被服务端移除时，补入最近会话；手动关闭的标签不自动复开。
    setOpenConversationIds((currentIds) => reconcileOpenConversationIds({
      conversationId,
      currentIds,
      excludedConversationIds: closedConversationIdsRef.current,
      fillRecent: capacityIncreased
        || hasNewRecentConversation
        || currentIds.length === 0
        || currentIds.some((id) => !liveConversationIds.has(id)),
      maxOpenCount: tabCapacity,
      pendingClosedId: pendingClosedActiveId,
      recentIds: recentConversationIds,
    }));
  }, [conversationId, pendingClosedActiveId, recentConversationIds, tabCapacity]);

  useEffect(() => {
    setPendingClosedActiveId((currentId) => (
      currentId && currentId !== conversationId ? null : currentId
    ));
  }, [conversationId]);

  useEffect(() => {
    setOptimisticActiveId((currentId) => {
      if (!currentId || currentId === conversationId || !conversationsById.has(currentId)) {
        return null;
      }
      return currentId;
    });
  }, [conversationId, conversationsById]);

  const previewConversation = (nextConversationId: string) => {
    if (nextConversationId === activeConversationId) {
      return;
    }
    flushSync(() => {
      setOptimisticActiveId(nextConversationId);
    });
  };

  const selectConversation = (nextConversationId: string) => {
    closedConversationIdsRef.current.delete(nextConversationId);
    previewConversation(nextConversationId);
    onSelectConversation(nextConversationId);
  };

  const closeConversation = (targetConversationId: string) => {
    if (orderedConversations.length <= 1) {
      return;
    }

    const nextActiveId = getCloseFallbackConversationId(
      orderedConversations,
      targetConversationId,
    );
    closedConversationIdsRef.current.add(targetConversationId);
    setOpenConversationIds((currentIds) => (
      currentIds.filter((id) => id !== targetConversationId)
    ));

    if (targetConversationId === activeConversationId && nextActiveId) {
      setPendingClosedActiveId(targetConversationId);
      previewConversation(nextActiveId);
      onSelectConversation(nextActiveId);
    }

    const targetConversation = conversationsById.get(targetConversationId);
    if (onCloseConversation && !isExternalSessionConversation(targetConversation)) {
      void onCloseConversation(targetConversationId).catch(() => undefined);
    }
  };

  const createConversation = async () => {
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

  const setConversationHovered = (targetConversationId: string, hovered: boolean) => {
    if (hovered) {
      setHoveredConversationId(targetConversationId);
      return;
    }
    setHoveredConversationId((currentId) => (
      currentId === targetConversationId ? null : currentId
    ));
  };

  return {
    activeConversationId,
    closeConversation,
    createConversation,
    hoveredConversationId,
    isCreating,
    orderedConversations,
    previewConversation,
    selectConversation,
    setConversationHovered,
    tabWidths,
    trackRef,
  };
}

function useTrackWidth(
  trackRef: React.RefObject<HTMLElement | null>,
  setTrackWidth: React.Dispatch<React.SetStateAction<number>>,
): void {
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
  }, [setTrackWidth, trackRef]);
}

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
  const recentConversationIds = useMemo(
    () => getRecentConversationIds(conversations),
    [conversations],
  );
  const [openConversationIds, setOpenConversationIds] = useState<string[]>(() => (
    getInitialOpenConversationIds(conversationId, recentConversationIds)
  ));
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
    hasCreateButton: Boolean(onCreateConversation),
    orderedConversations,
    trackWidth,
  }), [activeConversationId, onCreateConversation, orderedConversations, trackWidth]);

  useTrackWidth(trackRef, setTrackWidth);

  useEffect(() => {
    // 打开集合只响应真实会话目录和外部选择；关闭中的活动标签由本地事务接管。
    setOpenConversationIds((currentIds) => reconcileOpenConversationIds({
      conversationId,
      currentIds,
      pendingClosedId: pendingClosedActiveId,
      recentIds: recentConversationIds,
    }));
  }, [conversationId, pendingClosedActiveId, recentConversationIds]);

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

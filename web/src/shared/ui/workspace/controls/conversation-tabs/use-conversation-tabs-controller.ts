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
  getConversationIdsByCreationTime,
  getCloseFallbackConversationId,
  getInitialOpenConversationIds,
  getRecentConversationIds,
  reconcileOpenConversationIds,
  resolveActiveConversationId,
  shouldShowConversationTabsOverview,
} from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model";
import { RoomConversationView } from "@/types/conversation/conversation";

import { useConversationTabsScroll } from "./use-conversation-tabs-scroll";

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
  const [optimisticActiveId, setOptimisticActiveId] = useState<string | null>(null);
  const [pendingClosedActiveId, setPendingClosedActiveId] = useState<string | null>(null);
  const closedConversationIdsRef = useRef<Set<string>>(new Set());
  const knownConversationIdsRef = useRef<Set<string>>(new Set());
  const hasCreateButton = Boolean(onCreateConversation);
  const orderedConversationIds = useMemo(
    () => getConversationIdsByCreationTime(conversations),
    [conversations],
  );
  const recentConversationIds = useMemo(
    () => getRecentConversationIds(conversations),
    [conversations],
  );
  const [openConversationIds, setOpenConversationIds] = useState<string[]>(() => (
    getInitialOpenConversationIds(
      conversationId,
      orderedConversationIds,
      orderedConversationIds.length,
    )
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
  const recentConversations = useMemo(
    () => recentConversationIds
      .map((id) => conversationsById.get(id))
      .filter((conversation): conversation is RoomConversationView => Boolean(conversation)),
    [conversationsById, recentConversationIds],
  );
  const activeConversationId = resolveActiveConversationId({
    conversationId,
    optimisticId: optimisticActiveId,
    orderedConversations,
  });
  const hasTabsOverflow = useMemo(
    () => shouldShowConversationTabsOverview({
      conversationCount: orderedConversations.length,
      hasCreateButton,
      trackWidth,
    }),
    [hasCreateButton, orderedConversations.length, trackWidth],
  );
  const tabsScroll = useConversationTabsScroll({
    activeConversationId,
    contentKey: openConversationIds.join(":"),
  });
  const tabWidths = useMemo(() => calculateConversationTabWidths({
    activeConversationId,
    hasCreateButton,
    hasOverviewButton: hasTabsOverflow,
    orderedConversations,
    trackWidth,
  }), [
    activeConversationId,
    hasCreateButton,
    hasTabsOverflow,
    orderedConversations,
    trackWidth,
  ]);

  useTrackWidth(trackRef, setTrackWidth);

  useEffect(() => {
    const liveConversationIds = new Set(orderedConversationIds);
    const hasNewConversation = orderedConversationIds.some(
      (id) => !knownConversationIdsRef.current.has(id),
    );
    knownConversationIdsRef.current = liveConversationIds;
    for (const id of closedConversationIdsRef.current) {
      if (!liveConversationIds.has(id)) {
        closedConversationIdsRef.current.delete(id);
      }
    }
    if (conversationId && conversationId !== pendingClosedActiveId) {
      closedConversationIdsRef.current.delete(conversationId);
    }

    // 中文注释：全部会话按创建时间保留在可滚动标签带中；手动关闭的标签不自动复开。
    setOpenConversationIds((currentIds) => reconcileOpenConversationIds({
      conversationId,
      currentIds,
      excludedConversationIds: closedConversationIdsRef.current,
      fillAvailable: hasNewConversation
        || currentIds.length === 0
        || currentIds.some((id) => !liveConversationIds.has(id)),
      maxOpenCount: orderedConversationIds.length,
      orderedIds: orderedConversationIds,
      pendingClosedId: pendingClosedActiveId,
    }));
  }, [conversationId, orderedConversationIds, pendingClosedActiveId]);

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
    flushSync(() => {
      setOpenConversationIds((currentIds) => reconcileOpenConversationIds({
        conversationId: nextConversationId,
        currentIds,
        excludedConversationIds: closedConversationIdsRef.current,
        fillAvailable: false,
        maxOpenCount: orderedConversationIds.length,
        orderedIds: orderedConversationIds,
        pendingClosedId: null,
      }));
      setOptimisticActiveId(nextConversationId);
    });
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

  return {
    activeConversationId,
    closeConversation,
    createConversation,
    hasTabsOverflow,
    isCreating,
    orderedConversations,
    previewConversation,
    recentConversations,
    selectConversation,
    tabsScroll,
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

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
  hasConversationTabsOverflow,
  reconcileOpenConversationIds,
  resolveActiveConversationId,
  shouldPersistConversationTabs,
} from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model";
import { useRoomNavigationStore } from "@/store/room-navigation";
import { RoomConversationView } from "@/types/conversation/conversation";

import { useConversationTabsScroll } from "./use-conversation-tabs-scroll";

interface ConversationTabsControllerOptions {
  conversations: RoomConversationView[];
  conversationId: string | null;
  hasLeadingControl: boolean;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string | null) => void;
}

export function useConversationTabsController({
  conversations,
  conversationId,
  hasLeadingControl,
  onCloseConversation,
  onCreateConversation,
  onSelectConversation,
}: ConversationTabsControllerOptions) {
  const trackRef = useRef<HTMLElement | null>(null);
  const [trackWidth, setTrackWidth] = useState(0);
  const [isCreating, setIsCreating] = useState(false);
  const [optimisticActiveId, setOptimisticActiveId] = useState<string | null>(null);
  const [pendingClosedActiveId, setPendingClosedActiveId] = useState<string | null>(null);
  const hasCreateButton = Boolean(onCreateConversation);
  const roomId = conversations[0]?.room_id ?? null;
  const persistedTabs = useRoomNavigationStore((state) => (
    roomId ? state.conversation_tabs_by_room[roomId] : undefined
  ));
  const saveRoomConversationTabs = useRoomNavigationStore(
    (state) => state.save_room_conversation_tabs,
  );
  const orderedConversationIds = useMemo(
    () => getConversationIdsByCreationTime(conversations),
    [conversations],
  );
  const selectedConversationId = conversationId === pendingClosedActiveId
    ? persistedTabs?.active_conversation_id ?? null
    : conversationId ?? persistedTabs?.active_conversation_id ?? null;
  const initialOpenConversationIds = useMemo(
    () => persistedTabs?.open_conversation_ids ?? getInitialOpenConversationIds(
      selectedConversationId,
      orderedConversationIds,
    ),
    [orderedConversationIds, persistedTabs, selectedConversationId],
  );
  const openConversationIds = useMemo(
    () => reconcileOpenConversationIds({
      conversationId: selectedConversationId,
      currentIds: initialOpenConversationIds,
      orderedIds: orderedConversationIds,
      pendingClosedId: pendingClosedActiveId,
      preserveEmpty: persistedTabs?.active_conversation_id === null,
    }),
    [
      initialOpenConversationIds,
      orderedConversationIds,
      pendingClosedActiveId,
      persistedTabs?.active_conversation_id,
      selectedConversationId,
    ],
  );
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
    conversationId: selectedConversationId,
    optimisticId: optimisticActiveId,
    orderedConversations,
  });
  const hasTabsOverflow = useMemo(
    () => hasConversationTabsOverflow({
      conversationCount: orderedConversations.length,
      hasCreateButton,
      hasLeadingControl,
      trackWidth,
    }),
    [hasCreateButton, hasLeadingControl, orderedConversations.length, trackWidth],
  );
  const tabsScroll = useConversationTabsScroll({
    activeConversationId,
    contentKey: openConversationIds.join(":"),
  });
  const tabWidths = useMemo(() => calculateConversationTabWidths({
    activeConversationId,
    hasCreateButton,
    hasLeadingControl,
    hasTabsOverflow,
    orderedConversations,
    trackWidth,
  }), [
    activeConversationId,
    hasCreateButton,
    hasLeadingControl,
    hasTabsOverflow,
    orderedConversations,
    trackWidth,
  ]);

  useTrackWidth(trackRef, setTrackWidth);

  useEffect(() => {
    if (
      !roomId
      || !activeConversationId
      || openConversationIds.length === 0
      || !shouldPersistConversationTabs({
        activeConversationId,
        routeConversationId: conversationId,
      })
    ) {
      return;
    }
    // 中文注释：路由追上乐观活动项后再收敛持久化，避免旧路由把点击事务反向覆盖。
    saveRoomConversationTabs(roomId, openConversationIds, activeConversationId);
  }, [
    activeConversationId,
    conversationId,
    openConversationIds,
    roomId,
    saveRoomConversationTabs,
  ]);

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
    const nextOpenConversationIds = reconcileOpenConversationIds({
      conversationId: nextConversationId,
      currentIds: openConversationIds,
      orderedIds: orderedConversationIds,
      pendingClosedId: null,
    });
    flushSync(() => {
      if (roomId) {
        saveRoomConversationTabs(
          roomId,
          nextOpenConversationIds,
          nextConversationId,
        );
      }
      setOptimisticActiveId(nextConversationId);
    });
    onSelectConversation(nextConversationId);
  };

  const closeConversation = (targetConversationId: string) => {
    const fallbackConversationId = getCloseFallbackConversationId(
      orderedConversations,
      targetConversationId,
    );
    const nextOpenConversationIds = openConversationIds.filter(
      (id) => id !== targetConversationId,
    );
    const nextActiveConversationId = targetConversationId === activeConversationId
      ? fallbackConversationId
      : activeConversationId;

    flushSync(() => {
      if (roomId) {
        saveRoomConversationTabs(
          roomId,
          nextOpenConversationIds,
          nextActiveConversationId,
        );
      }
      if (targetConversationId === activeConversationId && nextActiveConversationId) {
        setPendingClosedActiveId(targetConversationId);
        setOptimisticActiveId(nextActiveConversationId);
      } else if (targetConversationId === activeConversationId) {
        setPendingClosedActiveId(targetConversationId);
        setOptimisticActiveId(null);
      }
    });

    if (targetConversationId === activeConversationId && nextActiveConversationId) {
      onSelectConversation(nextActiveConversationId);
    } else if (targetConversationId === activeConversationId) {
      onSelectConversation(null);
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

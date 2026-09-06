// INPUT: Room 会话、路由选择、持久标签偏好与创建/关闭/替换命令。
// OUTPUT: 已打开会话、乐观活动项、单飞事务和精确打开/关闭命令；旧列表与最后标签替换不覆盖其他持久标签。
// POS: Room 标签业务控制器；共享视图独立拥有 DOM、测量与滚动。

import {
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { flushSync } from "react-dom";

import { isExternalSessionConversation } from "@/lib/conversation/external-session";
import {
  getConversationIdsByCreationTime,
  getCloseFallbackConversationId,
  getInitialOpenConversationIds,
  reconcileOpenConversationIds,
  resolveActiveConversationId,
  shouldPersistConversationTabs,
} from "./room-conversation-tabs-model";
import { useRoomNavigationStore } from "@/store/room-navigation";
import { RoomConversationView } from "@/types/conversation/conversation";

import type { FinalConversationReplacementHandler } from "./final-conversation-replacement";

interface ConversationTabsControllerOptions {
  conversations: RoomConversationView[];
  conversationId: string | null;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation?: FinalConversationReplacementHandler;
  onSelectConversation: (conversationId: string) => void;
}

interface PendingConversationAction {
  kind: "create" | "replace";
  task: Promise<void>;
}

export function useRoomConversationTabs({
  conversations,
  conversationId,
  onCloseConversation,
  onCreateConversation,
  onReplaceFinalConversation,
  onSelectConversation,
}: ConversationTabsControllerOptions) {
  const [isCreating, setIsCreating] = useState(false);
  const pendingActionRef = useRef<PendingConversationAction | null>(null);
  const [optimisticActiveId, setOptimisticActiveId] = useState<string | null>(null);
  const [pendingClosedActiveId, setPendingClosedActiveId] = useState<string | null>(null);
  const roomId = conversations[0]?.room_id ?? null;
  const persistedTabs = useRoomNavigationStore((state) => (
    roomId ? state.conversation_tabs_by_room[roomId] : undefined
  ));
  const rememberLastActiveConversation = useRoomNavigationStore((state) => state.remember_last_active_conversation);
  const closeConversationTab = useRoomNavigationStore((state) => state.close_conversation_tab);
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
    }),
    [
      initialOpenConversationIds,
      orderedConversationIds,
      pendingClosedActiveId,
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
  useEffect(() => {
    if (
      !roomId
      || !activeConversationId
      || !shouldPersistConversationTabs({
        activeConversationId,
        routeConversationId: conversationId,
      })
    ) {
      return;
    }
    // A route change opens its exact target. List refreshes are presentation
    // updates and must not remove tabs opened by another page or newer command.
    rememberLastActiveConversation(roomId, activeConversationId);
  }, [
    activeConversationId,
    conversationId,
    roomId,
    rememberLastActiveConversation,
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

  const selectConversation = (nextConversationId: string) => {
    if (pendingActionRef.current?.kind === "replace") {
      return;
    }
    flushSync(() => {
      if (roomId) {
        rememberLastActiveConversation(roomId, nextConversationId);
      }
      setOptimisticActiveId(nextConversationId);
    });
    onSelectConversation(nextConversationId);
  };

  const createConversation = async (): Promise<void> => {
    if (!onCreateConversation || pendingActionRef.current) {
      return;
    }
    const task = onCreateConversation().then(() => undefined);
    pendingActionRef.current = {kind: "create", task};
    setIsCreating(true);
    try {
      await task;
    } finally {
      if (pendingActionRef.current?.task === task) {
        pendingActionRef.current = null;
        setIsCreating(false);
      }
    }
  };

  const replaceFinalConversation = async (
    targetConversationId: string,
  ): Promise<void> => {
    if (pendingActionRef.current) {
      return pendingActionRef.current.task;
    }
    const targetConversation = conversationsById.get(targetConversationId);
    if (
      !roomId
      || !onReplaceFinalConversation
      || !targetConversation
      || targetConversationId !== activeConversationId
    ) {
      return;
    }

    const task = (async () => {
      setIsCreating(true);
      await onReplaceFinalConversation(
        targetConversation,
        (nextConversationId) => {
          flushSync(() => {
            closeConversationTab(
              roomId,
              targetConversationId,
              nextConversationId,
            );
            if (nextConversationId !== targetConversationId) {
              setPendingClosedActiveId(targetConversationId);
              setOptimisticActiveId(nextConversationId);
            }
          });
          onSelectConversation(nextConversationId);
        },
      );
    })();
    pendingActionRef.current = {kind: "replace", task};
    try {
      await task;
    } finally {
      if (pendingActionRef.current?.task === task) {
        pendingActionRef.current = null;
        setIsCreating(false);
      }
    }
  };

  const closeConversation = (targetConversationId: string) => {
    const fallbackConversationId = getCloseFallbackConversationId(
      orderedConversations,
      targetConversationId,
    );
    if (!fallbackConversationId && orderedConversations.length === 1) {
      void replaceFinalConversation(targetConversationId).catch(() => undefined);
      return;
    }

    const nextActiveConversationId = targetConversationId === activeConversationId
      ? fallbackConversationId
      : activeConversationId;

    flushSync(() => {
      if (roomId && nextActiveConversationId) {
        closeConversationTab(
          roomId,
          targetConversationId,
          nextActiveConversationId,
        );
      }
      if (targetConversationId === activeConversationId && nextActiveConversationId) {
        setPendingClosedActiveId(targetConversationId);
        setOptimisticActiveId(nextActiveConversationId);
      }
    });

    if (targetConversationId === activeConversationId && nextActiveConversationId) {
      onSelectConversation(nextActiveConversationId);
    }

    const targetConversation = conversationsById.get(targetConversationId);
    if (onCloseConversation && !isExternalSessionConversation(targetConversation)) {
      void onCloseConversation(targetConversationId).catch(() => undefined);
    }
  };

  return {
    activeConversationId,
    closeConversation,
    createConversation,
    isCreating,
    orderedConversations,
    selectConversation,
  };
}

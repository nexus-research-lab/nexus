/**
 * [INPUT]: Room 路由、当前显式草稿与页面写命令。
 * [OUTPUT]: 会话选择、单飞创建、删除回退和目录返回导航。
 * [POS]: Room 页面浏览器协调层，不解释服务端会话协议。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getExternalSessionKeyFromConversationId } from "@/lib/conversation/external-session";
import { useRoomNavigationStore } from "@/store/room-navigation";
import { useSidebarStore } from "@/store/sidebar";

interface UseRoomPageNavigationOptions {
  roomId?: string | null;
  routeConversationId?: string | null;
  routeSessionKey?: string | null;
  currentRoomId: string | null;
  selectedConversationId: string | null;
  selectedDraftConversationId: string | null;
  isHydrated: boolean;
  createConversation: (title?: string) => Promise<string | null>;
  deleteConversation: (conversationId: string) => Promise<string | null>;
}

function buildConversationRoute(roomId: string, conversationId: string): string {
  const externalSessionKey = getExternalSessionKeyFromConversationId(conversationId);
  return externalSessionKey
    ? AppRouteBuilders.roomSession(roomId, externalSessionKey)
    : AppRouteBuilders.roomConversation(roomId, conversationId);
}

export function useRoomPageNavigation({
  roomId,
  routeConversationId,
  routeSessionKey,
  currentRoomId,
  selectedConversationId,
  selectedDraftConversationId,
  isHydrated,
  createConversation,
  deleteConversation,
}: UseRoomPageNavigationOptions) {
  const navigate = useNavigate();
  const createConversationTaskRef = useRef<Promise<string | null> | null>(null);
  const setWidePanelCollapsed = useSidebarStore(
    (state) => state.set_wide_panel_collapsed,
  );
  const rememberLastActiveConversation = useRoomNavigationStore(
    (state) => state.remember_last_active_conversation,
  );
  const [searchParams, setSearchParams] = useSearchParams();
  const queryInitialDraft = searchParams.get("initial")?.trim() || null;
  const [initialDraft, setInitialDraft] = useState<string | null>(queryInitialDraft);

  useEffect(() => {
    if (!queryInitialDraft) {
      return;
    }

    setInitialDraft((currentDraft) => currentDraft ?? queryInitialDraft);
    const nextSearchParams = new URLSearchParams(searchParams);
    nextSearchParams.delete("initial");
    setSearchParams(nextSearchParams, {replace: true});
  }, [queryInitialDraft, searchParams, setSearchParams]);

  const selectConversation = useCallback((conversationId: string | null) => {
    if (!roomId) {
      return;
    }
    if (!conversationId) {
      navigate(AppRouteBuilders.room(roomId));
      return;
    }
    rememberLastActiveConversation(roomId, conversationId);
    navigate(buildConversationRoute(roomId, conversationId));
  }, [navigate, rememberLastActiveConversation, roomId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    if (createConversationTaskRef.current) {
      return createConversationTaskRef.current;
    }
    const normalizedTitle = title?.trim();
    const task = (async () => {
      const reusableConversationId = !normalizedTitle
        ? selectedDraftConversationId
        : null;
      const conversationId = reusableConversationId
        ?? await createConversation(title);
      if (roomId && conversationId) {
        rememberLastActiveConversation(roomId, conversationId);
        navigate(buildConversationRoute(roomId, conversationId));
      }
      return conversationId;
    })();
    createConversationTaskRef.current = task;
    try {
      return await task;
    } finally {
      if (createConversationTaskRef.current === task) {
        createConversationTaskRef.current = null;
      }
    }
  }, [
    createConversation,
    navigate,
    rememberLastActiveConversation,
    roomId,
    selectedDraftConversationId,
  ]);

  const handleDeleteConversation = useCallback(async (conversationId: string) => {
    const isDeletingSelectedConversation = conversationId === selectedConversationId;
    const fallbackConversationId = await deleteConversation(conversationId);
    if (!roomId || !isDeletingSelectedConversation) {
      return fallbackConversationId;
    }

    navigate(
      fallbackConversationId
        ? buildConversationRoute(roomId, fallbackConversationId)
        : AppRouteBuilders.room(roomId),
    );
    if (fallbackConversationId) {
      rememberLastActiveConversation(roomId, fallbackConversationId);
    }
    return fallbackConversationId;
  }, [
    deleteConversation,
    navigate,
    rememberLastActiveConversation,
    roomId,
    selectedConversationId,
  ]);
  const backToChatDirectory = useCallback(() => {
    setWidePanelCollapsed(false);
    navigate(AppRouteBuilders.home());
  }, [navigate, setWidePanelCollapsed]);

  useEffect(() => {
    const shouldSelectCurrentConversation = (
      isHydrated
      && roomId
      && currentRoomId === roomId
      && !routeConversationId
      && !routeSessionKey
      && selectedConversationId
      && !initialDraft
      && !queryInitialDraft
    );
    if (!shouldSelectCurrentConversation) {
      return;
    }

    navigate(buildConversationRoute(roomId, selectedConversationId), {replace: true});
  }, [
    currentRoomId,
    initialDraft,
    isHydrated,
    navigate,
    queryInitialDraft,
    roomId,
    routeConversationId,
    routeSessionKey,
    selectedConversationId,
  ]);

  return {
    initialDraft,
    consumeInitialDraft: () => setInitialDraft(null),
    backToChatDirectory,
    selectConversation,
    createConversation: handleCreateConversation,
    deleteConversation: handleDeleteConversation,
  };
}

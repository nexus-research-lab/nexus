import { useCallback, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getExternalSessionKeyFromConversationId } from "@/lib/conversation/external-session";

interface UseRoomPageNavigationOptions {
  roomId?: string | null;
  routeConversationId?: string | null;
  routeSessionKey?: string | null;
  currentRoomId: string | null;
  selectedConversationId: string | null;
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
  isHydrated,
  createConversation,
  deleteConversation,
}: UseRoomPageNavigationOptions) {
  const navigate = useNavigate();
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

  const selectConversation = useCallback((conversationId: string) => {
    if (roomId) {
      navigate(buildConversationRoute(roomId, conversationId));
    }
  }, [navigate, roomId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    const conversationId = await createConversation(title);
    if (roomId && conversationId) {
      navigate(buildConversationRoute(roomId, conversationId));
    }
    return conversationId;
  }, [createConversation, navigate, roomId]);

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
    return fallbackConversationId;
  }, [deleteConversation, navigate, roomId, selectedConversationId]);

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
    backToLauncher: () => navigate(AppRouteBuilders.launcher()),
    selectConversation,
    createConversation: handleCreateConversation,
    deleteConversation: handleDeleteConversation,
  };
}

/**
 * [INPUT]: Room 路由、当前显式草稿与页面写命令。
 * [OUTPUT]: 会话选择、按 Room 单飞创建、最后标签带导航代次/作用域校验的替换、删除回退和目录导航。
 * [POS]: Room 页面浏览器协调层；拥有路由提交顺序，不解释服务端会话协议。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { deleteSessionApi } from "@/lib/api/conversation/session-api";
import { getExternalSessionKeyFromConversationId } from "@/lib/conversation/external-session";
import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import {
  replaceFinalConversation as runFinalConversationReplacement,
  type FinalConversationReplacementHandler,
} from "@/shared/ui/workspace/controls/conversation-tabs/final-conversation-replacement";
import { useRoomNavigationStore } from "@/store/room-navigation";
import { useSidebarStore } from "@/store/sidebar";

interface FinalConversationReplacementScope {
  activeConversationId: string | null;
  currentEpoch: number;
  currentRoomId: string | null;
  expectedConversationId: string;
  expectedEpoch: number;
  expectedRoomId: string;
  openConversationIds: readonly string[] | null;
  selectedConversationId: string | null;
}

export function isFinalConversationReplacementCurrent({
  activeConversationId,
  currentEpoch,
  currentRoomId,
  expectedConversationId,
  expectedEpoch,
  expectedRoomId,
  openConversationIds,
  selectedConversationId,
}: FinalConversationReplacementScope): boolean {
  const tabsRemainExact = !openConversationIds || (
    activeConversationId === expectedConversationId
    && openConversationIds.length === 1
    && openConversationIds[0] === expectedConversationId
  );
  return currentEpoch === expectedEpoch
    && currentRoomId === expectedRoomId
    && selectedConversationId === expectedConversationId
    && tabsRemainExact;
}

interface UseRoomPageNavigationOptions {
  roomId?: string | null;
  routeConversationId?: string | null;
  routeSessionKey?: string | null;
  currentRoomId: string | null;
  selectedConversationId: string | null;
  selectedDraftConversationId: string | null;
  isHydrated: boolean;
  closeConversation: (conversationId: string) => Promise<void>;
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
  closeConversation,
  createConversation,
  deleteConversation,
}: UseRoomPageNavigationOptions) {
  const navigate = useNavigate();
  const createConversationTasksRef = useRef(
    new Map<string, Promise<string | null>>(),
  );
  const navigationEpochRef = useRef(0);
  const navigationScopeRef = useRef({
    roomId: roomId ?? null,
    routeConversationId: routeConversationId ?? null,
    routeSessionKey: routeSessionKey ?? null,
    selectedConversationId,
  });
  if (
    navigationScopeRef.current.roomId !== (roomId ?? null)
    || navigationScopeRef.current.routeConversationId !== (routeConversationId ?? null)
    || navigationScopeRef.current.routeSessionKey !== (routeSessionKey ?? null)
    || navigationScopeRef.current.selectedConversationId !== selectedConversationId
  ) {
    navigationEpochRef.current += 1;
    navigationScopeRef.current = {
      roomId: roomId ?? null,
      routeConversationId: routeConversationId ?? null,
      routeSessionKey: routeSessionKey ?? null,
      selectedConversationId,
    };
  }
  const setWidePanelCollapsed = useSidebarStore(
    (state) => state.set_wide_panel_collapsed,
  );
  const rememberLastActiveConversation = useRoomNavigationStore(
    (state) => state.remember_last_active_conversation,
  );
  const forgetConversation = useRoomNavigationStore(
    (state) => state.forget_conversation,
  );
  const [searchParams, setSearchParams] = useSearchParams();
  const queryInitialDraft = searchParams.get("initial")?.trim() || null;
  const [initialDraft, setInitialDraft] = useState<string | null>(queryInitialDraft);

  useEffect(() => () => {
    navigationEpochRef.current += 1;
  }, []);

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
      navigationEpochRef.current += 1;
      rememberLastActiveConversation(roomId, conversationId);
      navigate(buildConversationRoute(roomId, conversationId));
    }
  }, [navigate, rememberLastActiveConversation, roomId]);

  const ensureConversation = useCallback(async (title?: string) => {
    if (!roomId) {
      return null;
    }
    const normalizedTitle = title?.trim();
    const reusableConversationId = !normalizedTitle
      ? selectedDraftConversationId
      : null;
    if (reusableConversationId) {
      return reusableConversationId;
    }
    const currentTask = createConversationTasksRef.current.get(roomId);
    if (currentTask) {
      return currentTask;
    }
    const task = createConversation(title);
    createConversationTasksRef.current.set(roomId, task);
    try {
      return await task;
    } finally {
      if (createConversationTasksRef.current.get(roomId) === task) {
        createConversationTasksRef.current.delete(roomId);
      }
    }
  }, [createConversation, roomId, selectedDraftConversationId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    if (!roomId) {
      return null;
    }
    const scopeRoomId = roomId;
    const actionEpoch = ++navigationEpochRef.current;
    const conversationId = await ensureConversation(title);
    if (!conversationId) {
      return conversationId;
    }
    if (
      navigationEpochRef.current !== actionEpoch
      || navigationScopeRef.current.roomId !== scopeRoomId
    ) {
      return null;
    }
    rememberLastActiveConversation(scopeRoomId, conversationId);
    navigate(buildConversationRoute(scopeRoomId, conversationId));
    return conversationId;
  }, [ensureConversation, navigate, rememberLastActiveConversation, roomId]);

  const replaceFinalConversation = useCallback<FinalConversationReplacementHandler>(async (
    conversation,
    commitConversation,
  ) => {
    const scopeRoomId = roomId ?? null;
    if (
      !scopeRoomId
      || createConversationTasksRef.current.has(scopeRoomId)
    ) {
      return;
    }
    const expectedConversationId = conversation.conversation_id;
    const actionEpoch = ++navigationEpochRef.current;
    const isCurrent = () => {
      const liveTabs = useRoomNavigationStore.getState()
        .conversation_tabs_by_room[scopeRoomId];
      return isFinalConversationReplacementCurrent({
        activeConversationId: liveTabs?.active_conversation_id ?? null,
        currentEpoch: navigationEpochRef.current,
        currentRoomId: navigationScopeRef.current.roomId,
        expectedConversationId,
        expectedEpoch: actionEpoch,
        expectedRoomId: scopeRoomId,
        openConversationIds: liveTabs?.open_conversation_ids ?? null,
        selectedConversationId: navigationScopeRef.current.selectedConversationId,
      });
    };

    await runFinalConversationReplacement({
      closeConversation,
      conversation,
      createConversation: () => ensureConversation(),
      isCurrent,
      commitConversation,
    });
  }, [
    closeConversation,
    ensureConversation,
    roomId,
  ]);

  const handleDeleteConversation = useCallback(async (conversationId: string) => {
    navigationEpochRef.current += 1;
    const isDeletingSelectedConversation = conversationId === selectedConversationId;
    const externalSessionKey = getExternalSessionKeyFromConversationId(conversationId);
    if (externalSessionKey) {
      await deleteSessionApi(externalSessionKey);
      notifyRoomDirectoryUpdated();
      if (roomId) {
        forgetConversation(roomId, conversationId);
      }
      if (roomId && isDeletingSelectedConversation) {
        navigate(AppRouteBuilders.room(roomId));
      }
      return null;
    }
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
    forgetConversation,
    navigate,
    rememberLastActiveConversation,
    roomId,
    selectedConversationId,
  ]);
  const backToChatDirectory = useCallback(() => {
    navigationEpochRef.current += 1;
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
    replaceFinalConversation,
    deleteConversation: handleDeleteConversation,
  };
}

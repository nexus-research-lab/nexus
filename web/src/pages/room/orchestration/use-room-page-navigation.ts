/**
 * [INPUT]: Room 路由参数、首次主持消息与目标 Agent 参数、当前 Room 状态及会话命令。
 * [OUTPUT]: Room 导航、首次主持消息投递及引导进入/退出状态。
 * [POS]: URL 与 Room 页面状态之间的导航编排边界。
 */

import { useCallback, useEffect, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  completeHomeOnboarding,
  getHomeOnboardingReturnPath,
  isHomeOnboardingCompleted,
} from "@/features/onboarding/home-agent-onboarding";
import { getExternalSessionKeyFromConversationId } from "@/lib/conversation/external-session";
import {
  isRoomInitialActionConsumed,
  markRoomInitialActionConsumed,
} from "@/lib/conversation/room-initial-action";

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

function parseInitialTargetAgentIDs(value: string): string[] {
  return value
    .split(",")
    .map((agentID) => agentID.trim())
    .filter(Boolean);
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
  const { pathname } = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryInitialActionKey =
    searchParams.get("initial_action_key")?.trim() || null;
  const queryInitialActionConsumed = queryInitialActionKey
    ? isRoomInitialActionConsumed(queryInitialActionKey)
    : false;
  const queryInitialDraft = queryInitialActionConsumed
    ? null
    : searchParams.get("initial")?.trim() || null;
  const queryOnboarding = searchParams.get("onboarding") === "1";
  const queryScriptedHostMessage =
    !queryInitialActionConsumed
    && searchParams.get("scripted_host_message") === "1";
  const queryInitialTargetAgentIDs =
    searchParams.get("initial_target_agent_ids")?.trim() || "";
  const rememberedOnboardingRoute =
    getHomeOnboardingReturnPath()?.split("?")[0] === pathname;
  const [initialDraft, setInitialDraft] = useState<string | null>(queryInitialDraft);
  const [initialDraftAsHost, setInitialDraftAsHost] = useState(
    queryScriptedHostMessage,
  );
  const [initialTargetAgentIDs, setInitialTargetAgentIDs] = useState(
    parseInitialTargetAgentIDs(queryInitialTargetAgentIDs),
  );
  const [initialActionKey, setInitialActionKey] = useState<string | null>(
    queryInitialActionConsumed ? null : queryInitialActionKey,
  );
  const [suppressRoomTour] = useState(queryScriptedHostMessage);
  const [isOnboarding, setIsOnboarding] = useState(
    queryOnboarding || rememberedOnboardingRoute,
  );

  useEffect(() => {
    if (!queryScriptedHostMessage) {
      return;
    }
    // 进入最终 Room 就是新手引导的完成边界；即使链接在新标签页打开，
    // 也要由目标页自行收口状态，立即撤销 NX 提示光效与页面锁定。
    completeHomeOnboarding();
    setIsOnboarding(false);
  }, [queryScriptedHostMessage]);

  useEffect(() => {
    const handleOnboardingStateChange = () => {
      if (isHomeOnboardingCompleted()) {
        setIsOnboarding(false);
      }
    };
    window.addEventListener(
      "nexus:home-onboarding-state-change",
      handleOnboardingStateChange,
    );
    return () => {
      window.removeEventListener(
        "nexus:home-onboarding-state-change",
        handleOnboardingStateChange,
      );
    };
  }, []);

  useEffect(() => {
    if (
      !queryInitialDraft
      && !queryOnboarding
      && !queryScriptedHostMessage
      && !queryInitialActionKey
    ) {
      return;
    }

    setInitialDraft((currentDraft) => currentDraft ?? queryInitialDraft);
    setInitialDraftAsHost((currentValue) => (
      currentValue || queryScriptedHostMessage
    ));
    setInitialTargetAgentIDs((currentValue) => (
      currentValue.length > 0
        ? currentValue
        : parseInitialTargetAgentIDs(queryInitialTargetAgentIDs)
    ));
    setInitialActionKey((currentValue) => (
      currentValue ?? (queryInitialActionConsumed ? null : queryInitialActionKey)
    ));
    const nextSearchParams = new URLSearchParams(searchParams);
    nextSearchParams.delete("initial");
    nextSearchParams.delete("onboarding");
    nextSearchParams.delete("scripted_host_message");
    nextSearchParams.delete("initial_target_agent_ids");
    nextSearchParams.delete("initial_action_key");
    setSearchParams(nextSearchParams, {replace: true});
  }, [
    queryInitialDraft,
    queryOnboarding,
    queryScriptedHostMessage,
    queryInitialTargetAgentIDs,
    queryInitialActionConsumed,
    queryInitialActionKey,
    searchParams,
    setSearchParams,
  ]);

  const selectConversation = useCallback((conversationId: string) => {
    if (roomId) {
      setIsOnboarding(false);
      navigate(buildConversationRoute(roomId, conversationId));
    }
  }, [navigate, roomId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    const conversationId = await createConversation(title);
    if (roomId && conversationId) {
      setIsOnboarding(false);
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
    setIsOnboarding(false);
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
        && !isOnboarding
        && !queryOnboarding
    );
    if (!shouldSelectCurrentConversation) {
      return;
    }

    navigate(buildConversationRoute(roomId, selectedConversationId), {replace: true});
  }, [
    currentRoomId,
    initialDraft,
    isOnboarding,
    isHydrated,
    navigate,
    queryInitialDraft,
    queryOnboarding,
    roomId,
    routeConversationId,
    routeSessionKey,
    selectedConversationId,
  ]);

  return {
    initialDraft,
    initialSendOptions: initialDraftAsHost
      ? {
          scripted_host_message: true as const,
          ...(initialTargetAgentIDs.length > 0
            ? { target_agent_ids: initialTargetAgentIDs }
            : {}),
        }
      : undefined,
    isOnboarding,
    suppressRoomTour,
    consumeInitialDraft: () => {
      if (initialActionKey) {
        markRoomInitialActionConsumed(initialActionKey);
      }
      setInitialDraft(null);
      setInitialDraftAsHost(false);
      setInitialTargetAgentIDs([]);
      setInitialActionKey(null);
    },
    backToLauncher: () => navigate(AppRouteBuilders.launcher()),
    selectConversation,
    createConversation: handleCreateConversation,
    deleteConversation: handleDeleteConversation,
  };
}

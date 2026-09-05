"use client";

/**
 * INPUT: Launcher 输入、最近会话与页面导航能力。
 * OUTPUT: 串行查询、读取安全重试、私聊写入结果核对和持久反馈状态。
 * POS: Launcher 用户命令控制器；只读查询可重试，结果未知的 DM ensure 不得盲目重放。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import {
  queryLauncher,
} from "@/lib/api/launcher-api";
import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import {
  projectMutationFailure,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

import type {
  LauncherConsoleProps,
  RecentLauncherEntry,
} from "./launcher-console-types";
import {
  projectLauncherOperationFailure,
  type LauncherOperationFailure,
} from "./launcher-operation-failure";

type LauncherControllerFailure =
  | ({ submittedQuery: string } & Extract<LauncherOperationFailure, { kind: "query_read" }>)
  | ({ initialMessage?: string; roomId: string } & Extract<LauncherOperationFailure, { kind: "room_read" }>)
  | ({ entryKey: string } & Extract<LauncherOperationFailure, { kind: "target_missing" }>)
  | ({ agentId: string; initialMessage?: string } & Extract<LauncherOperationFailure, { kind: "direct_room" }>);

interface UseLauncherConsoleControllerOptions {
  initialQuery: string;
  onOpenMainAgentDm: LauncherConsoleProps["onOpenMainAgentDm"];
  onOpenRoute: LauncherConsoleProps["onOpenRoute"];
}

export function useLauncherConsoleController({
  initialQuery,
  onOpenMainAgentDm,
  onOpenRoute,
}: UseLauncherConsoleControllerOptions) {
  const { t } = useI18n();
  const [query, setQuery] = useState(initialQuery);
  const [isQueryLoading, setIsQueryLoading] = useState(false);
  const [failure, setFailure] = useState<LauncherControllerFailure | null>(null);
  const queryInFlightRef = useRef(false);
  const setCurrentAgent = useAgentStore((state) => state.set_current_agent);
  const setActivePanelItem = useSidebarStore((state) => state.set_active_panel_item);

  useEffect(() => {
    if (initialQuery) setQuery(initialQuery);
  }, [initialQuery]);

  const openConversation = useCallback((
    roomId: string,
    conversationId: string,
    initialMessage?: string,
  ) => {
    setActivePanelItem(roomId);
    const route = AppRouteBuilders.roomConversation(roomId, conversationId);
    onOpenRoute(initialMessage
      ? `${route}?initial=${encodeURIComponent(initialMessage)}`
      : route);
  }, [onOpenRoute, setActivePanelItem]);

  const openAgentTarget = useCallback(async (
    agentId: string,
    initialMessage?: string,
  ) => {
    try {
      const { context } = await resolveDirectRoomNavigationTarget(agentId);
      setCurrentAgent(agentId);
      openConversation(
        context.room.id,
        context.conversation.id,
        initialMessage,
      );
      setFailure(null);
    } catch (error) {
      const projected = projectMutationFailure(
        error,
        t("launcher.failure.direct_room_message"),
      );
      setFailure({
        agentId,
        effect: projected.effect,
        initialMessage,
        kind: "direct_room",
      });
    }
  }, [openConversation, setCurrentAgent, t]);

  const openRoomTarget = useCallback(async (
    roomId: string,
    initialMessage?: string,
  ) => {
    try {
      const contexts = await getRoomContexts(roomId);
      const conversation = contexts[0]?.conversation;
      if (!conversation) {
        setFailure({ entryKey: roomId, kind: "target_missing" });
        return;
      }
      openConversation(roomId, conversation.id, initialMessage);
      setFailure(null);
    } catch {
      setFailure({ initialMessage, kind: "room_read", roomId });
    }
  }, [openConversation]);

  const openRecentEntry = useCallback((entry: RecentLauncherEntry) => {
    if (entry.conversation_id && entry.room_id) {
      openConversation(entry.room_id, entry.conversation_id);
      setFailure(null);
      return;
    }
    if (entry.type === "dm" && entry.agent_id) {
      void openAgentTarget(entry.agent_id);
      return;
    }
    if (entry.type === "room" && entry.room_id) {
      void openRoomTarget(entry.room_id);
      return;
    }
    setFailure({ entryKey: entry.key, kind: "target_missing" });
  }, [openAgentTarget, openConversation, openRoomTarget]);

  const executeQuery = useCallback(async (submittedQuery: string) => {
    queryInFlightRef.current = true;
    setIsQueryLoading(true);
    try {
      let action;
      try {
        // Launcher Query 只解析目录，不写入聊天；同一查询可以安全重试。
        action = await queryLauncher({ query: submittedQuery });
      } catch {
        setFailure({ kind: "query_read", submittedQuery });
        return;
      }

      switch (action.action_type) {
        case "open_agent_dm":
          await openAgentTarget(action.target_id, action.initial_message);
          break;
        case "open_room":
          await openRoomTarget(action.target_id, action.initial_message);
          break;
        case "open_app":
          setFailure(null);
          onOpenMainAgentDm(action.initial_message || submittedQuery);
          break;
      }
    } finally {
      queryInFlightRef.current = false;
      setIsQueryLoading(false);
    }
  }, [onOpenMainAgentDm, openAgentTarget, openRoomTarget]);

  const submitQuery = useCallback((input: string) => {
    const submittedQuery = input.trim();
    if (!submittedQuery || queryInFlightRef.current) {
      return false;
    }
    void executeQuery(submittedQuery);
    return true;
  }, [executeQuery]);

  const updateQuery = useCallback((value: string) => setQuery(value), []);
  const enterHome = useCallback(() => {
    onOpenRoute(AppRouteBuilders.home());
  }, [onOpenRoute]);

  const recoverFailure = useCallback(() => {
    if (!failure) {
      return;
    }
    switch (failure.kind) {
      case "query_read":
        void executeQuery(failure.submittedQuery);
        return;
      case "room_read":
        void openRoomTarget(failure.roomId, failure.initialMessage);
        return;
      case "direct_room":
        if (failure.effect === "not_applied") {
          void openAgentTarget(failure.agentId, failure.initialMessage);
          return;
        }
        enterHome();
        return;
      case "target_missing":
        enterHome();
    }
  }, [enterHome, executeQuery, failure, openAgentTarget, openRoomTarget]);

  const feedback = useMemo(
    () => failure
      ? projectLauncherOperationFailure(t, failure, recoverFailure)
      : null,
    [failure, recoverFailure, t],
  );

  return {
    actions: {
      enterHome,
      openRecentEntry,
      submitQuery,
      updateQuery,
    },
    state: {
      feedback,
      isQueryLoading,
      query,
    },
  };
}

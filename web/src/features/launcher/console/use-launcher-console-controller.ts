"use client";

import { useCallback, useMemo, useRef, useState } from "react";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  queryLauncher,
  type LauncherQueryResponse,
} from "@/lib/api/launcher-api";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import { useSidebarStore } from "@/store/sidebar";

import type {
  LauncherConsoleProps,
  RecentLauncherEntry,
} from "./launcher-console-types";

type LauncherActionType = LauncherQueryResponse["action_type"];
type LauncherActionHandler = (
  action: LauncherQueryResponse,
  submittedQuery: string,
) => Promise<void>;
type RecentEntryKind = "conversation" | "dm" | "room";
type RecentEntryHandler = (entry: RecentLauncherEntry) => Promise<void>;

interface UseLauncherConsoleControllerOptions {
  onOpenMainAgentDm: LauncherConsoleProps["onOpenMainAgentDm"];
  onOpenRoute: LauncherConsoleProps["onOpenRoute"];
  onSelectAgent: LauncherConsoleProps["onSelectAgent"];
}

export function useLauncherConsoleController({
  onOpenMainAgentDm,
  onOpenRoute,
  onSelectAgent,
}: UseLauncherConsoleControllerOptions) {
  const [query, setQuery] = useState("");
  const [isQueryLoading, setIsQueryLoading] = useState(false);
  const queryInFlightRef = useRef(false);
  const setActivePanelItem = useSidebarStore((state) => state.set_active_panel_item);

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

  const actionHandlers = useMemo<Readonly<Record<LauncherActionType, LauncherActionHandler>>>(
    () => ({
      open_agent_dm: async (action) => {
        onSelectAgent(action.target_id);
        const { context } = await resolveDirectRoomNavigationTarget(action.target_id);
        openConversation(
          context.room.id,
          context.conversation.id,
          action.initial_message,
        );
      },
      open_app: async (action, submittedQuery) => {
        onOpenMainAgentDm(action.initial_message || submittedQuery);
      },
      open_room: async (action) => {
        const contexts = await getRoomContexts(action.target_id);
        const conversation = contexts[0]?.conversation;
        if (conversation) {
          openConversation(action.target_id, conversation.id, action.initial_message);
        }
      },
    }),
    [onOpenMainAgentDm, onSelectAgent, openConversation],
  );

  const recentEntryHandlers = useMemo<Readonly<Record<RecentEntryKind, RecentEntryHandler>>>(
    () => ({
      conversation: async (entry) => {
        if (entry.room_id && entry.conversation_id) {
          openConversation(entry.room_id, entry.conversation_id);
        }
      },
      dm: async (entry) => {
        if (!entry.agent_id) {
          return;
        }
        onSelectAgent(entry.agent_id);
        const { context } = await resolveDirectRoomNavigationTarget(entry.agent_id);
        openConversation(context.room.id, context.conversation.id);
      },
      room: async (entry) => {
        if (!entry.room_id) {
          return;
        }
        const contexts = await getRoomContexts(entry.room_id);
        const conversation = contexts[0]?.conversation;
        if (conversation) {
          openConversation(entry.room_id, conversation.id);
        }
      },
    }),
    [onSelectAgent, openConversation],
  );

  const openRecentEntry = useCallback((entry: RecentLauncherEntry) => {
    const kind: RecentEntryKind = entry.conversation_id ? "conversation" : entry.type;
    void recentEntryHandlers[kind](entry).catch((error) => {
      console.error("Failed to open recent entry:", error);
    });
  }, [recentEntryHandlers]);

  const executeQuery = useCallback(async (submittedQuery: string) => {
    queryInFlightRef.current = true;
    setIsQueryLoading(true);
    try {
      const action = await queryLauncher({ query: submittedQuery });
      await actionHandlers[action.action_type](action, submittedQuery);
    } catch (error) {
      console.error("Launcher query failed:", error);
    } finally {
      queryInFlightRef.current = false;
      setIsQueryLoading(false);
    }
  }, [actionHandlers]);

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

  return {
    actions: {
      enterHome,
      openRecentEntry,
      submitQuery,
      updateQuery,
    },
    state: {
      isQueryLoading,
      query,
    },
  };
}

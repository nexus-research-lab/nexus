"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { useSidebarStore } from "@/store/sidebar";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { resolveRoomSubagentTaskSource } from "../room-surface-model";
import type { RoomAgentAboutRequest } from "./room-surface-layout-types";

const RIGHT_PANEL_AUTO_COLLAPSE_SIDEBAR_QUERY = "(max-width: 1440px)";

interface RoomSurfaceLayoutControllerOptions {
  activeSurfaceTab: RoomSurfaceTabKey;
  conversationId: string | null;
  currentAgentId: string;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  isDm: boolean;
  isThreadPanelOpen: boolean;
  onChangeSurfaceTab: (tab: RoomSurfaceTabKey) => void;
  roomId: string | null;
}

export function useRoomSurfaceLayoutController({
  activeSurfaceTab,
  conversationId,
  currentAgentId,
  currentAgentSessionIdentity,
  isDm,
  isThreadPanelOpen,
  onChangeSurfaceTab,
  roomId,
}: RoomSurfaceLayoutControllerOptions) {
  const [aboutRequest, setAboutRequest] = useState<RoomAgentAboutRequest>({
    agent_id: null,
    tab: "identity",
    key: 0,
  });
  const isAuxiliaryPanelOpen = activeSurfaceTab !== "chat";
  const subagentTaskSource = useMemo(
    () => resolveRoomSubagentTaskSource({
      conversationId,
      isDm,
      roomId,
      sessionIdentity: currentAgentSessionIdentity,
    }),
    [conversationId, currentAgentSessionIdentity, isDm, roomId],
  );

  useWidePanelAutoCollapse(isAuxiliaryPanelOpen || isThreadPanelOpen);

  const requestAboutPanel = useCallback((agentId: string) => {
    setAboutRequest((current) => ({
      agent_id: agentId,
      tab: "identity",
      key: current.key + 1,
    }));
  }, []);
  const handleChangeSurfaceTab = useCallback((tab: RoomSurfaceTabKey) => {
    if (tab === "about") {
      requestAboutPanel(currentAgentId);
    }
    onChangeSurfaceTab(tab);
  }, [currentAgentId, onChangeSurfaceTab, requestAboutPanel]);
  const handleOpenAgentContact = useCallback((agentId: string) => {
    requestAboutPanel(agentId);
    onChangeSurfaceTab("about");
  }, [onChangeSurfaceTab, requestAboutPanel]);
  const handleCloseAuxiliaryPanel = useCallback(() => {
    onChangeSurfaceTab("chat");
  }, [onChangeSurfaceTab]);

  useEffect(() => {
    if (activeSurfaceTab === "subagents" && !subagentTaskSource) {
      onChangeSurfaceTab("chat");
    }
  }, [activeSurfaceTab, onChangeSurfaceTab, subagentTaskSource]);

  return {
    aboutRequest,
    handleChangeSurfaceTab,
    handleCloseAuxiliaryPanel,
    handleOpenAgentContact,
    isAuxiliaryPanelOpen,
    subagentTaskSource,
  };
}

function useWidePanelAutoCollapse(isRightPanelOpen: boolean) {
  const shouldAutoCollapse = useMediaQuery(
    RIGHT_PANEL_AUTO_COLLAPSE_SIDEBAR_QUERY,
  );
  const collapseWidePanel = useSidebarStore(
    (state) => state.collapse_wide_panel_for_right_panel,
  );
  const restoreWidePanel = useSidebarStore(
    (state) => state.expand_wide_panel_after_right_panel,
  );

  useEffect(() => {
    if (isRightPanelOpen && shouldAutoCollapse) {
      collapseWidePanel();
      return;
    }
    restoreWidePanel();
  }, [collapseWidePanel, isRightPanelOpen, restoreWidePanel, shouldAutoCollapse]);

  useEffect(() => () => {
    restoreWidePanel();
  }, [restoreWidePanel]);
}

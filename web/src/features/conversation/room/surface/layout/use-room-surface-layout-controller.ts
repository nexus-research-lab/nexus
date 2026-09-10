// INPUT: 当前 Room/Agent/Session 身份、辅助页与 Thread 开关、页面导航命令。
// OUTPUT: 简介请求、子任务来源和宽侧栏联动的桌面布局控制器。
// POS: Room 桌面导航编排；简介请求按 owner/Room/Agent 隔离，Session 切换保留简介选择。
"use client";

import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from "react";

import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
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
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration,
  );
  const aboutScope = JSON.stringify([ownerGeneration, roomId, currentAgentId]);
  const [aboutRequest, setAboutRequest] = useResettableState<RoomAgentAboutRequest>({
    agent_id: null,
    tab: "identity",
    key: 0,
  }, aboutScope);
  const [subagentRequest, setSubagentRequest] = useState({
    hostAgentId: null as string | null,
    key: 0,
    toolUseId: null as string | null,
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
  }, [setAboutRequest]);
  const handleChangeSurfaceTab = useCallback((tab: RoomSurfaceTabKey) => {
    if (tab === "about") {
      requestAboutPanel(currentAgentId);
    }
    if (tab === "subagents") {
      setSubagentRequest((current) => ({
        hostAgentId: null,
        key: current.key + 1,
        toolUseId: null,
      }));
    }
    onChangeSurfaceTab(tab);
  }, [currentAgentId, onChangeSurfaceTab, requestAboutPanel]);
  const handleOpenAgentContact = useCallback((agentId: string) => {
    requestAboutPanel(agentId);
    onChangeSurfaceTab("about");
  }, [onChangeSurfaceTab, requestAboutPanel]);
  const handleOpenSubagentTask = useCallback((
    toolUseId: string,
    hostAgentId?: string | null,
  ) => {
    const normalizedToolUseId = toolUseId.trim();
    if (!normalizedToolUseId || !subagentTaskSource) {
      return;
    }
    setSubagentRequest((current) => ({
      hostAgentId: hostAgentId?.trim() || null,
      key: current.key + 1,
      toolUseId: normalizedToolUseId,
    }));
    onChangeSurfaceTab("subagents");
  }, [onChangeSurfaceTab, subagentTaskSource]);
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
    handleOpenSubagentTask,
    isAuxiliaryPanelOpen,
    subagentRequest,
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

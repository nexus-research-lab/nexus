"use client";

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { getDefaultAgentId } from "@/config/runtime-options";
import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { LauncherConsole } from "@/features/launcher/console/launcher-console";
import { getLauncherSurfaceThemeStyle } from "@/features/launcher/hero/launcher-surface-theme";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { useTheme } from "@/shared/theme/theme-context";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

export function LauncherPage() {
  const { theme } = useTheme();
  const { agents, conversations, isLoading, rooms } = useHomeDirectory();
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const setCurrentAgent = useAgentStore((state) => state.set_current_agent);
  const navigate = useNavigate();
  const setActivePanelItem = useSidebarStore(
    (state) => state.set_active_panel_item,
  );
  const defaultAgentId = getDefaultAgentId();

  const openNavigationRoute = useCallback(
    (route: string) => {
      navigate(route);
    },
    [navigate],
  );

  const openAgentDm = useCallback(
    (agentId: string, initialPrompt?: string) => {
      void resolveDirectRoomNavigationTarget(agentId, initialPrompt)
        .then(({ context, route }) => {
          setCurrentAgent(agentId);
          setActivePanelItem(context.room.id);
          openNavigationRoute(route);
        })
        .catch((error) => {
          console.error("[LauncherPage] 打开 Agent DM 失败:", error);
        });
    },
    [openNavigationRoute, setActivePanelItem, setCurrentAgent],
  );

  const handleOpenMainAgentDm = useCallback(
    (initialPrompt?: string) => {
      if (!defaultAgentId) {
        console.error("[LauncherPage] 主智能体 ID 未就绪，无法打开 DM。");
        return;
      }
      openAgentDm(defaultAgentId, initialPrompt);
    },
    [defaultAgentId, openAgentDm],
  );

  const handleSelectAgent = useCallback(
    (agentId: string) => {
      openAgentDm(agentId);
    },
    [openAgentDm],
  );

  if (isLoading) {
    return <AppLoadingScreen />;
  }

  return (
    <div
      className="relative flex min-h-0 flex-1 overflow-hidden"
      style={getLauncherSurfaceThemeStyle(theme)}
    >
      <LauncherConsole
        agents={agents}
        conversations={conversations}
        currentAgentId={currentAgentId}
        onOpenMainAgentDm={handleOpenMainAgentDm}
        onOpenRoute={openNavigationRoute}
        onSelectAgent={handleSelectAgent}
        rooms={rooms}
      />
    </div>
  );
}

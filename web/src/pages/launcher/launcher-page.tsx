"use client";

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { getDefaultAgentId, isMainAgent } from "@/config/options";
import { LauncherConsole } from "@/features/launcher/console/launcher-console";
import { getLauncherSurfaceThemeStyle } from "@/features/launcher/hero/launcher-surface-theme";
import { useLauncherPageController } from "@/hooks/launcher/use-launcher-page-controller";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { useTheme } from "@/shared/theme/theme-context";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { SIDEBAR_SYSTEM_ITEM_IDS, useSidebarStore } from "@/store/sidebar";

export function LauncherPage() {
  const { theme } = useTheme();
  const controller = useLauncherPageController();
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
      const nextActiveItemId = isMainAgent(agentId)
        ? SIDEBAR_SYSTEM_ITEM_IDS.nexus
        : agentId;
      setActivePanelItem(nextActiveItemId);

      void resolveDirectRoomNavigationTarget(agentId, initialPrompt)
        .then(({ context, route }) => {
          controller.handle_select_agent(agentId);
          setActivePanelItem(
            isMainAgent(agentId)
              ? SIDEBAR_SYSTEM_ITEM_IDS.nexus
              : context.room.id,
          );
          openNavigationRoute(route);
        })
        .catch((error) => {
          console.error("[LauncherPage] 打开 Agent DM 失败:", error);
        });
    },
    [controller, openNavigationRoute, setActivePanelItem],
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

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  return (
    <div
      className="relative flex min-h-0 flex-1 overflow-hidden"
      style={getLauncherSurfaceThemeStyle(theme)}
    >
      <LauncherConsole
        agents={controller.agents}
        conversations={controller.conversations}
        currentAgentId={controller.current_agent_id}
        onOpenMainAgentDm={handleOpenMainAgentDm}
        onOpenRoute={openNavigationRoute}
        onSelectAgent={handleSelectAgent}
        rooms={controller.rooms}
      />
    </div>
  );
}

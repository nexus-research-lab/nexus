"use client";

/**
 * INPUT: 共享 Home 目录、主题、当前 Agent、可选初始草稿与 Launcher 导航命令。
 * OUTPUT: 加载、首次失败、stale 降级和带可确认草稿的正常 Console 页面状态。
 * POS: Launcher 页面装配层；目录请求与重试状态仍归 Home 共享资源。
 */
import { CircleAlert } from "lucide-react";
import { useCallback } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { getDefaultAgentId } from "@/config/runtime-options";
import { HomeDirectoryRefreshErrorNotice } from "@/features/home/home-directory-refresh-error-notice";
import {
  refreshHomeDirectory,
  useHomeDirectory,
} from "@/features/home/home-directory-resource";
import { LauncherConsole } from "@/features/launcher/console/launcher-console";
import { getLauncherSurfaceThemeStyle } from "@/features/launcher/hero/launcher-surface-theme";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

export function LauncherPage() {
  const { t } = useI18n();
  const { theme } = useTheme();
  const {
    agents,
    conversations,
    hasError,
    hasLoaded,
    isLoading,
    rooms,
  } = useHomeDirectory();
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const setCurrentAgent = useAgentStore((state) => state.set_current_agent);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialQuery = (searchParams.get("initial") ?? "").trim().slice(0, 4000);
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

  if (hasError && !hasLoaded) {
    return (
      <div
        className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden px-6"
        style={getLauncherSurfaceThemeStyle(theme)}
      >
        <UiStateBlock
          actions={(
            <UiButton onClick={refreshHomeDirectory} size="sm">
              {t("sidebar.retry")}
            </UiButton>
          )}
          className="w-full max-w-lg"
          description={t("sidebar.directory_load_failed_description")}
          icon={<CircleAlert className="h-6 w-6" />}
          role="alert"
          title={t("sidebar.directory_load_failed")}
          tone="danger"
          variant="card"
        />
      </div>
    );
  }

  return (
    <div
      className="relative flex min-h-0 flex-1 overflow-hidden"
      style={getLauncherSurfaceThemeStyle(theme)}
    >
      {hasError && hasLoaded ? (
        <HomeDirectoryRefreshErrorNotice
          className="absolute left-1/2 top-4 z-40 w-[min(32rem,calc(100%-2rem))] -translate-x-1/2 shadow-sm"
          onRetry={refreshHomeDirectory}
        />
      ) : null}
      <LauncherConsole
        agents={agents}
        conversations={conversations}
        currentAgentId={currentAgentId}
        initialQuery={initialQuery}
        onOpenMainAgentDm={handleOpenMainAgentDm}
        onOpenRoute={openNavigationRoute}
        onSelectAgent={handleSelectAgent}
        rooms={rooms}
      />
    </div>
  );
}

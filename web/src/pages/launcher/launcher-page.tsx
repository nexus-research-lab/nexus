"use client";

/**
 * INPUT: 共享 Home 目录、主题、当前 Agent、可选初始草稿与 Launcher 导航命令。
 * OUTPUT: 加载、目录降级、DM ensure 结果核对和带可确认草稿的正常 Console 页面状态。
 * POS: Launcher 页面装配层；目录请求归 Home 资源，结果未知的私聊准备不得直接重放。
 */
import { useCallback, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { getDefaultAgentId } from "@/config/runtime-options";
import { HomeDirectoryRefreshErrorNotice } from "@/features/home/home-directory-refresh-error-notice";
import {
  refreshHomeDirectory,
  useHomeDirectory,
} from "@/features/home/home-directory-resource";
import { LauncherConsole } from "@/features/launcher/console/launcher-console";
import {
  projectLauncherOperationFailure,
  type LauncherOperationFailure,
} from "@/features/launcher/console/launcher-operation-failure";
import { getLauncherSurfaceThemeStyle } from "@/features/launcher/hero/launcher-surface-theme";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { projectMutationFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

type LauncherNavigationFailure =
  | Extract<LauncherOperationFailure, { kind: "main_agent_missing" }>
  | ({ agentId: string; initialPrompt?: string } & Extract<LauncherOperationFailure, { kind: "direct_room" }>);

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
  const [navigationFailure, setNavigationFailure] = useState<LauncherNavigationFailure | null>(null);
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
    async (agentId: string, initialPrompt?: string) => {
      try {
        const { context, route } = await resolveDirectRoomNavigationTarget(
          agentId,
          initialPrompt,
        );
        setCurrentAgent(agentId);
        setActivePanelItem(context.room.id);
        setNavigationFailure(null);
        openNavigationRoute(route);
      } catch (error) {
        const projected = projectMutationFailure(
          error,
          t("launcher.failure.direct_room_message"),
        );
        setNavigationFailure({
          agentId,
          effect: projected.effect,
          initialPrompt,
          kind: "direct_room",
        });
      }
    },
    [openNavigationRoute, setActivePanelItem, setCurrentAgent, t],
  );

  const handleOpenMainAgentDm = useCallback(
    (initialPrompt?: string) => {
      if (!defaultAgentId) {
        setNavigationFailure({ kind: "main_agent_missing" });
        return;
      }
      void openAgentDm(defaultAgentId, initialPrompt);
    },
    [defaultAgentId, openAgentDm],
  );

  const handleSelectAgent = useCallback(
    (agentId: string) => {
      void openAgentDm(agentId);
    },
    [openAgentDm],
  );

  const recoverNavigationFailure = useCallback(() => {
    if (
      navigationFailure?.kind === "direct_room" &&
      navigationFailure.effect === "not_applied"
    ) {
      void openAgentDm(
        navigationFailure.agentId,
        navigationFailure.initialPrompt,
      );
      return;
    }
    openNavigationRoute(AppRouteBuilders.home());
  }, [navigationFailure, openAgentDm, openNavigationRoute]);

  const navigationFeedback = useMemo(
    () => navigationFailure
      ? projectLauncherOperationFailure(
          t,
          navigationFailure,
          recoverNavigationFailure,
        )
      : null,
    [navigationFailure, recoverNavigationFailure, t],
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
        <UiResourceState
          className="w-full max-w-lg"
          impact={t("sidebar.directory_load_failed_impact")}
          primaryAction={{
            label: t("sidebar.retry"),
            onClick: refreshHomeDirectory,
          }}
          state="error"
          title={t("sidebar.directory_load_failed")}
          urgency="polite"
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
        feedback={navigationFeedback}
        initialQuery={initialQuery}
        onOpenMainAgentDm={handleOpenMainAgentDm}
        onOpenRoute={openNavigationRoute}
        onSelectAgent={handleSelectAgent}
        rooms={rooms}
      />
    </div>
  );
}

"use client";

import { lazy, Suspense, useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

import { get_default_agent_id, is_main_agent } from "@/config/options";
import { LauncherConsole } from "@/features/launcher/launcher-console";
import { get_launcher_surface_theme_style } from "@/features/launcher/launcher-surface-theme";
import { useLauncherPageController } from "@/hooks/launcher/use-launcher-page-controller";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { useTheme } from "@/shared/theme/theme-context";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { SIDEBAR_SYSTEM_ITEM_IDS, useSidebarStore } from "@/store/sidebar";
import {
  AgentIdentityDraft,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";

const AgentOptions = lazy(() =>
  import("@/shared/ui/dialog/agent-options").then((m) => ({ default: m.AgentOptions })),
);
const ConfirmDialog = lazy(() =>
  import("@/shared/ui/dialog/confirm-dialog").then((m) => ({ default: m.ConfirmDialog })),
);

export function LauncherPage() {
  const { theme } = useTheme();
  const controller = useLauncherPageController();
  const navigate = useNavigate();
  const setActivePanelItem = useSidebarStore(
    (state) => state.set_active_panel_item,
  );
  const defaultAgentId = get_default_agent_id();
  const [pendingDeleteAgent, setPendingDeleteAgent] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const openNavigationRoute = useCallback(
    (route: string) => {
      navigate(route);
    },
    [navigate],
  );

  const openAgentDm = useCallback(
    (agentId: string, initialPrompt?: string) => {
      const nextActiveItemId = is_main_agent(agentId)
        ? SIDEBAR_SYSTEM_ITEM_IDS.nexus
        : agentId;
      setActivePanelItem(nextActiveItemId);

      void resolve_direct_room_navigation_target(agentId, initialPrompt)
        .then(({ context, route }) => {
          controller.handle_select_agent(agentId);
          setActivePanelItem(
            is_main_agent(agentId)
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

  const handleSaveAgentOptions = useCallback(
    async (
      _title: string,
      options: AgentConfigOptions,
      identity: AgentIdentityDraft,
    ) => {
      const shouldOpenRoomAfterCreate = controller.dialog_mode === "create";
      await controller.handle_save_agent_options(_title, options, identity);

      if (!shouldOpenRoomAfterCreate) {
        return;
      }

      const nextAgentId = useAgentStore.getState().current_agent_id;
      if (!nextAgentId) {
        return;
      }

      const { context, route } =
        await resolve_direct_room_navigation_target(nextAgentId);
      setActivePanelItem(context.room.id);
      openNavigationRoute(route);
    },
    [controller, openNavigationRoute, setActivePanelItem],
  );

  const handleRequestDeleteAgent = useCallback(
    (agentId: string) => {
      const targetAgent = controller.agents.find(
        (agent) => agent.id === agentId,
      );
      controller.set_is_dialog_open(false);
      setPendingDeleteAgent({
        id: agentId,
        name: targetAgent?.name ?? "该 Agent",
      });
    },
    [controller],
  );

  const handleConfirmDeleteAgent = useCallback(async () => {
    if (!pendingDeleteAgent) {
      return;
    }

    await controller.handle_delete_agent(pendingDeleteAgent.id);
    setPendingDeleteAgent(null);
  }, [controller, pendingDeleteAgent]);

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  return (
    <>
      <div
        className="relative flex min-h-0 flex-1 overflow-hidden"
        style={get_launcher_surface_theme_style(theme)}
      >
        <LauncherConsole
          agents={controller.agents}
          rooms={controller.rooms}
          conversations={controller.conversations}
          current_agent_id={controller.current_agent_id}
          on_open_main_agent_dm={handleOpenMainAgentDm}
          on_open_route={openNavigationRoute}
          on_select_agent={handleSelectAgent}
        />
      </div>

      <Suspense fallback={null}>
        {controller.is_dialog_open ? (
          <AgentOptions
            agent_id={controller.editing_agent_id ?? undefined}
            mode={controller.dialog_mode}
            is_open={controller.is_dialog_open}
            on_close={() => {
              controller.set_is_dialog_open(false);
            }}
            on_delete={handleRequestDeleteAgent}
            on_save={handleSaveAgentOptions}
            on_validate_name={controller.handle_validate_agent_name}
            initial_avatar={controller.dialog_initial_avatar}
            initial_description={controller.dialog_initial_description}
            initial_title={controller.dialog_initial_title}
            initial_options={controller.dialog_initial_options}
            initial_vibe_tags={controller.dialog_initial_vibe_tags}
          />
        ) : null}

        {pendingDeleteAgent ? (
          <ConfirmDialog
            confirm_text="删除成员"
            is_open={Boolean(pendingDeleteAgent)}
            message={`删除「${pendingDeleteAgent?.name ?? "该 Agent"}」后，该成员将不再出现在当前前端列表中。已有历史协作不会自动删除。`}
            on_cancel={() => setPendingDeleteAgent(null)}
            on_confirm={() => {
              void handleConfirmDeleteAgent();
            }}
            title="删除成员"
            variant="danger"
          />
        ) : null}
      </Suspense>
    </>
  );
}

"use client";

import { lazy, Suspense, useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

import { getDefaultAgentId, isMainAgent } from "@/config/options";
import { LauncherConsole } from "@/features/launcher/launcher-console";
import { getLauncherSurfaceThemeStyle } from "@/features/launcher/launcher-surface-theme";
import { useLauncherPageController } from "@/hooks/launcher/use-launcher-page-controller";
import { resolveDirectRoomNavigationTarget } from "@/lib/conversation/direct-room-navigation";
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
  const defaultAgentId = getDefaultAgentId();
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

  const handleSaveAgentOptions = useCallback(
    async (
      _title: string,
      options: AgentConfigOptions,
      identity: AgentIdentityDraft,
    ) => {
      const shouldOpenRoomAfterCreate = controller.dialogMode === "create";
      await controller.handleSaveAgentOptions(_title, options, identity);

      if (!shouldOpenRoomAfterCreate) {
        return;
      }

      const nextAgentId = useAgentStore.getState().current_agent_id;
      if (!nextAgentId) {
        return;
      }

      const { context, route } =
        await resolveDirectRoomNavigationTarget(nextAgentId);
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
      controller.setIsDialogOpen(false);
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
        style={getLauncherSurfaceThemeStyle(theme)}
      >
        <LauncherConsole
          agents={controller.agents}
          rooms={controller.rooms}
          conversations={controller.conversations}
          currentAgentId={controller.current_agent_id}
          onOpenMainAgentDm={handleOpenMainAgentDm}
          onOpenRoute={openNavigationRoute}
          onSelectAgent={handleSelectAgent}
        />
      </div>

      <Suspense fallback={null}>
        {controller.isDialogOpen ? (
          <AgentOptions
            agentId={controller.editingAgentId ?? undefined}
            mode={controller.dialogMode}
            isOpen={controller.isDialogOpen}
            onClose={() => {
              controller.setIsDialogOpen(false);
            }}
            onDelete={handleRequestDeleteAgent}
            onSave={handleSaveAgentOptions}
            onValidateName={controller.handleValidateAgentName}
            initialAvatar={controller.dialogInitialAvatar}
            initialDescription={controller.dialogInitialDescription}
            initialTitle={controller.dialogInitialTitle}
            initialOptions={controller.dialogInitialOptions}
            initialVibeTags={controller.dialogInitialVibeTags}
          />
        ) : null}

        {pendingDeleteAgent ? (
          <ConfirmDialog
            confirmText="删除成员"
            isOpen={Boolean(pendingDeleteAgent)}
            message={`删除「${pendingDeleteAgent?.name ?? "该 Agent"}」后，该成员将不再出现在当前前端列表中。已有历史协作不会自动删除。`}
            onCancel={() => setPendingDeleteAgent(null)}
            onConfirm={() => {
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

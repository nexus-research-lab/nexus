import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { AgentOptions } from "@/components/dialog/agent-options";
import { LauncherConsole } from "@/features/launcher-search/launcher-console";
import { HomeLoadingScreen } from "@/components/home/home-loading-screen";
import { useLauncherPageController } from "@/hooks/use-launcher-page-controller";
import { AppStage } from "@/shared/ui/app-stage";
import { useAgentStore } from "@/store/agent";
import { SessionOptions } from "@/types/session";

export function LauncherPage() {
  const controller = useLauncherPageController();
  const navigate = useNavigate();

  const handleSelectAgent = useCallback((agentId: string) => {
    controller.handleAgentSelect(agentId);
    navigate(AppRouteBuilders.room(agentId));
  }, [controller, navigate]);

  const handleOpenSession = useCallback((sessionKey: string, agentId?: string) => {
    controller.handleOpenConversationFromLauncher(sessionKey, agentId);
    const routeRoomId = agentId ?? controller.currentAgentId;
    if (routeRoomId) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, sessionKey));
    }
  }, [controller, navigate]);

  const handleOpenContactsPage = useCallback(() => {
    navigate(AppRouteBuilders.contacts());
  }, [navigate]);

  const handleOpenNexus = useCallback(() => {
    navigate(AppRouteBuilders.nexus());
  }, [navigate]);

  const handleSaveAgentOptions = useCallback(async (title: string, options: SessionOptions) => {
    const shouldOpenRoomAfterCreate = controller.dialogMode === "create";
    await controller.handleSaveAgentOptions(title, options);

    if (!shouldOpenRoomAfterCreate) {
      return;
    }

    const nextAgentId = useAgentStore.getState().current_agent_id;
    if (nextAgentId) {
      navigate(AppRouteBuilders.room(nextAgentId));
    }
  }, [controller, navigate]);

  if (!controller.isHydrated) {
    return <HomeLoadingScreen />;
  }

  return (
    <AppStage>
      <LauncherConsole
        agents={controller.agents}
        conversations={controller.conversations}
        current_agent_id={controller.currentAgentId}
        on_open_contacts_page={handleOpenContactsPage}
        on_open_nexus={handleOpenNexus}
        on_select_agent={handleSelectAgent}
        on_open_conversation={handleOpenSession}
        on_create_agent={controller.handleOpenCreateAgent}
        on_edit_agent={controller.handleEditAgent}
        on_delete_agent={controller.handleDeleteAgent}
      />

      <AgentOptions
        mode={controller.dialogMode}
        isOpen={controller.isDialogOpen}
        onClose={() => controller.setIsDialogOpen(false)}
        onSave={handleSaveAgentOptions}
        onValidateName={controller.handleValidateAgentName}
        initialTitle={controller.dialogInitialTitle}
        initialOptions={controller.dialogInitialOptions}
      />
    </AppStage>
  );
}

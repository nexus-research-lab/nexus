import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { LauncherConsole } from "@/features/launcher-search/launcher-console";
import { useLauncherPageController } from "@/hooks/use-launcher-page-controller";
import { AppStage } from "@/shared/ui/app-stage";
import { AgentOptions } from "@/shared/ui/agent-options-dialog";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { SessionOptions } from "@/types/session";

export function LauncherPage() {
  const controller = useLauncherPageController();
  const navigate = useNavigate();

  const handleSelectAgent = useCallback((agentId: string) => {
    controller.handleAgentSelect(agentId);
    navigate(AppRouteBuilders.room(agentId));
  }, [controller, navigate]);

  const handleOpenConversation = useCallback((conversationId: string, agentId?: string) => {
    controller.handleOpenConversationFromLauncher(conversationId, agentId);
    const routeRoomId = agentId ?? controller.current_agent_id;
    if (routeRoomId) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, conversationId));
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
    return <AppLoadingScreen />;
  }

  return (
    <AppStage>
      <LauncherConsole
        agents={controller.agents}
        conversations={controller.conversations}
        current_agent_id={controller.current_agent_id}
        on_open_contacts_page={handleOpenContactsPage}
        on_open_nexus={handleOpenNexus}
        on_select_agent={handleSelectAgent}
        on_open_conversation={handleOpenConversation}
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
        initialTitle={controller.dialog_initial_title}
        initialOptions={controller.dialog_initial_options}
      />
    </AppStage>
  );
}

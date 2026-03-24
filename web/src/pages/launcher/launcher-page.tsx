import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { LauncherConsole } from "@/features/launcher-search/launcher-console";
import { useLauncherPageController } from "@/hooks/use-launcher-page-controller";
import { AppStage } from "@/shared/ui/app-stage";
import { AgentOptions } from "@/shared/ui/agent-options-dialog";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { AgentOptions as AgentConfigOptions } from "@/types/agent";

export function LauncherPage() {
  const controller = useLauncherPageController();
  const navigate = useNavigate();

  const handleSelectAgent = useCallback((agent_id: string) => {
    controller.handle_select_agent(agent_id);
    navigate(AppRouteBuilders.room(agent_id));
  }, [controller, navigate]);

  const handleOpenConversation = useCallback((conversation_id: string, agent_id?: string) => {
    controller.handle_open_conversation_from_launcher(conversation_id, agent_id);
    const route_room_id = agent_id ?? controller.current_agent_id;
    if (route_room_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, conversation_id));
    }
  }, [controller, navigate]);

  const handleOpenContactsPage = useCallback(() => {
    navigate(AppRouteBuilders.contacts());
  }, [navigate]);

  const handleOpenNexus = useCallback(() => {
    navigate(AppRouteBuilders.nexus());
  }, [navigate]);

  const handleSaveAgentOptions = useCallback(async (title: string, options: AgentConfigOptions) => {
    const should_open_room_after_create = controller.dialog_mode === "create";
    await controller.handle_save_agent_options(title, options);

    if (!should_open_room_after_create) {
      return;
    }

    const next_agent_id = useAgentStore.getState().current_agent_id;
    if (next_agent_id) {
      navigate(AppRouteBuilders.room(next_agent_id));
    }
  }, [controller, navigate]);

  if (!controller.is_hydrated) {
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
        on_create_agent={controller.handle_open_create_agent}
        on_edit_agent={controller.handle_edit_agent}
        on_delete_agent={controller.handle_delete_agent}
      />

      <AgentOptions
        mode={controller.dialog_mode}
        is_open={controller.is_dialog_open}
        on_close={() => controller.set_is_dialog_open(false)}
        on_save={handleSaveAgentOptions}
        on_validate_name={controller.handle_validate_agent_name}
        initial_title={controller.dialog_initial_title}
        initial_options={controller.dialog_initial_options}
      />
    </AppStage>
  );
}

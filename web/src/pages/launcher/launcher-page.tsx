import { useCallback } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { LauncherAppConversationPanel } from "@/features/launcher/launcher-app-conversation-panel";
import { LauncherConsole } from "@/features/launcher/launcher-console";
import { useLauncherPageController } from "@/hooks/use-launcher-page-controller";
import { cn } from "@/lib/utils";
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
      <div className="relative flex min-h-0 flex-1 gap-5 overflow-hidden">
        <div
          className={cn(
            "min-w-0 flex-1 transition-all duration-500 ease-out",
            controller.is_app_conversation_open && "lg:max-w-[calc(100%-390px)] lg:-translate-x-6",
          )}
        >
          <LauncherConsole
            agents={controller.agents}
            conversations={controller.conversations}
            current_agent_id={controller.current_agent_id}
            on_open_contacts_page={handleOpenContactsPage}
            on_open_app_conversation={controller.open_app_conversation}
            on_select_agent={handleSelectAgent}
            on_open_conversation={handleOpenConversation}
            on_create_agent={controller.handle_open_create_agent}
            on_edit_agent={controller.handle_edit_agent}
            on_delete_agent={controller.handle_delete_agent}
            surface={controller.surface}
          />
        </div>

        {controller.is_app_conversation_open ? (
          <div className="absolute inset-x-3 bottom-4 top-[96px] z-40 lg:static lg:inset-auto lg:block lg:w-[380px] lg:shrink-0 lg:pb-8 lg:pt-4">
            <LauncherAppConversationPanel
              app_conversation_draft={controller.app_conversation_draft}
              app_conversation_messages={controller.app_conversation_messages}
              on_clear_conversation={controller.clear_app_conversation}
              on_change_draft={controller.set_app_conversation_draft}
              on_close={controller.close_app_conversation}
              on_open_contacts_page={handleOpenContactsPage}
              on_submit={controller.submit_app_conversation}
            />
          </div>
        ) : null}
      </div>

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

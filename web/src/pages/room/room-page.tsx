import { useCallback, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { RoomWorkspaceShell } from "@/features/room-conversation/room-workspace-shell";
import { RoomRouteEntry } from "@/features/room-conversation/room-route-entry";
import { useRoomPageController } from "@/hooks/use-room-page-controller";
import { AgentOptions } from "@/shared/ui/agent-options-dialog";
import { AppStage } from "@/shared/ui/app-stage";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { RoomRouteParams } from "@/types/route";

export function RoomPage() {
  const params = useParams<RoomRouteParams>();
  const navigate = useNavigate();
  const controller = useRoomPageController({
    room_id: params.room_id,
    conversation_id: params.conversation_id,
  });

  const handleBackToLauncher = useCallback(() => {
    controller.handle_back_to_directory();
    navigate(AppRouteBuilders.launcher());
  }, [controller, navigate]);

  const handleSelectAgent = useCallback((agent_id: string) => {
    controller.handle_select_agent(agent_id);
  }, [controller]);

  const handleSelectConversation = useCallback((conversation_id: string) => {
    controller.handle_select_conversation(conversation_id);
    const route_room_id = params.room_id;
    if (route_room_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, conversation_id));
    }
  }, [controller, navigate, params.room_id]);

  useEffect(() => {
    if (
      controller.is_hydrated &&
      params.room_id &&
      !params.conversation_id &&
      controller.current_conversation_id
    ) {
      navigate(
        AppRouteBuilders.room_conversation(
          params.room_id,
          controller.current_conversation_id,
        ),
        {replace: true},
      );
    }
  }, [
    controller.current_conversation_id,
    controller.is_hydrated,
    navigate,
    params.conversation_id,
    params.room_id,
  ]);

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  if (controller.current_room && controller.current_agent) {
    return (
      <AppStage>
        <RoomWorkspaceShell
          active_workspace_path={controller.active_workspace_path}
          agent_cost_summary={controller.agent_cost_summary}
          current_agent={controller.current_agent}
          current_agent_id={controller.current_agent_id}
          room_members={controller.room_members}
          current_room_title={controller.current_room_title}
          current_room_conversations={controller.current_room_conversations}
          current_conversation={controller.current_conversation}
          current_conversation_id={controller.current_conversation_id}
          current_todos={controller.current_todos}
          editor_width_percent={controller.editor_width_percent}
          is_editor_open={controller.is_editor_open}
          is_resizing_editor={controller.is_resizing_editor}
          is_conversation_busy={controller.is_conversation_busy}
          on_back_to_directory={handleBackToLauncher}
          on_close_workspace_pane={controller.handle_close_workspace_pane}
          on_delete_conversation={controller.handle_delete_conversation}
          on_edit_agent={controller.handle_edit_agent}
          on_loading_change={controller.set_is_conversation_busy}
          on_create_conversation={controller.handle_create_conversation}
          on_open_create_agent={controller.handle_open_create_agent}
          on_open_workspace_file={controller.handle_open_workspace_file}
          on_select_agent={handleSelectAgent}
          on_select_conversation={handleSelectConversation}
          on_conversation_snapshot_change={controller.handle_conversation_snapshot_change}
          on_start_editor_resize={controller.handle_start_editor_resize}
          on_todos_change={controller.set_current_todos}
          conversation_cost_summary={controller.conversation_cost_summary}
          workspace_split_ref={controller.workspace_split_ref}
        />

        <AgentOptions
          mode={controller.dialog_mode}
          is_open={controller.is_dialog_open}
          on_close={() => controller.set_is_dialog_open(false)}
          on_save={controller.handle_save_agent_options}
          on_validate_name={controller.handle_validate_agent_name}
          initial_title={controller.dialog_initial_title}
          initial_options={controller.dialog_initial_options}
        />
      </AppStage>
    );
  }

  return (
    <AppStage>
      <div className="relative flex min-h-0 flex-1 flex-col px-4 py-4 sm:px-6 sm:py-6">
        <section className="workspace-shell relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[30px] p-4 sm:p-6">
          <RoomRouteEntry
            agents={controller.agents}
            conversations={controller.conversations}
            conversation_id={params.conversation_id}
            room_id={params.room_id}
          />
        </section>
      </div>
    </AppStage>
  );
}

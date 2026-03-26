import { useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { OpenRoomShell } from "@/features/open-room/open-room-shell";
import { ProtocolRoomShell } from "@/features/protocol-room/protocol-room-shell";
import { RoomWorkspaceShell } from "@/features/room-conversation/room-workspace-shell";
import { RoomRouteEntry } from "@/features/room-conversation/room-route-entry";
import { useRoomPageController } from "@/hooks/use-room-page-controller";
import { AgentOptions } from "@/shared/ui/agent-options-dialog";
import { AppStage } from "@/shared/ui/app-stage";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { getConversationStoreSnapshot } from "@/store/conversation";
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
    navigate(AppRouteBuilders.room(agent_id));
  }, [controller, navigate]);

  const handleSelectConversation = useCallback((conversation_id: string) => {
    controller.handle_select_conversation(conversation_id);
    const route_room_id = controller.current_agent_id ?? params.room_id;
    if (route_room_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, conversation_id));
    }
  }, [controller, navigate, params.room_id]);

  const handleCreateConversation = useCallback(async () => {
    await controller.handle_create_conversation();
    const route_room_id = controller.current_agent_id ?? params.room_id;
    const next_session_key = getConversationStoreSnapshot().current_conversation_id;
    if (route_room_id && next_session_key) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, next_session_key));
    }
  }, [controller, navigate, params.room_id]);

  if (
    !controller.is_hydrated
    || (params.room_id && (!controller.is_checked || !controller.is_open_room_checked))
  ) {
    return <AppLoadingScreen />;
  }

  if (controller.is_protocol_room && controller.room) {
    return (
      <AppStage>
        <ProtocolRoomShell
          detail={controller.detail}
          error={controller.error}
          is_loading={controller.is_room_loading || controller.is_run_loading}
          on_control={controller.handle_control}
          on_create_run={controller.handle_create_run}
          on_refresh={controller.handle_refresh}
          on_select_channel={controller.handle_select_channel}
          on_select_run={controller.handle_select_run}
          on_set_viewer={controller.handle_set_viewer}
          on_submit_action={controller.handle_submit_action}
          pending_requests={controller.pending_requests}
          room={controller.room}
          room_agent_members={controller.room_agent_members}
          runs={controller.runs}
          selected_channel={controller.selected_channel}
          selected_channel_events={controller.selected_channel_events}
          selected_channel_id={controller.selected_channel_id}
          viewer_agent_id={controller.viewer_agent_id}
          ws_state={controller.ws_state}
        />
      </AppStage>
    );
  }

  if (controller.is_open_room && controller.open_room_view) {
    return (
      <AppStage>
        <OpenRoomShell
          error={controller.open_room_error}
          is_loading={controller.is_open_room_loading}
          on_post_action={controller.handle_open_room_action}
          on_post_message={controller.handle_open_room_message}
          on_refresh={controller.handle_open_room_refresh}
          on_run_phase={controller.handle_open_room_run_phase}
          on_run_until_finished={controller.handle_open_room_run_until_finished}
          on_start={controller.handle_open_room_start}
          on_tick={controller.handle_open_room_tick}
          view={controller.open_room_view}
        />
      </AppStage>
    );
  }

  if (controller.current_agent) {
    return (
      <AppStage>
        <RoomWorkspaceShell
          active_workspace_path={controller.active_workspace_path}
          agent_cost_summary={controller.agent_cost_summary}
          agents={controller.agents}
          current_agent={controller.current_agent}
          current_agent_id={controller.current_agent_id}
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
          on_create_conversation={handleCreateConversation}
          on_open_create_agent={controller.handle_open_create_agent}
          on_open_workspace_file={controller.handle_open_workspace_file}
          on_select_agent={handleSelectAgent}
          on_select_conversation={handleSelectConversation}
          on_conversation_snapshot_change={controller.handle_conversation_snapshot_change}
          on_start_editor_resize={controller.handle_start_editor_resize}
          on_todos_change={controller.set_current_todos}
          recent_agents={controller.recent_agents}
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

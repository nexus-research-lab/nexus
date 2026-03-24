import { useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { RoomWorkspaceShell } from "@/features/room-conversation/room-workspace-shell";
import { useRoomPageController } from "@/hooks/use-room-page-controller";
import { RouteScaffold } from "@/shared/ui/route-scaffold";
import { AgentOptions } from "@/shared/ui/agent-options-dialog";
import { AppStage } from "@/shared/ui/app-stage";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { useSessionStore } from "@/store/session";

export function RoomPage() {
  const params = useParams<{ roomId?: string; conversationId?: string }>();
  const navigate = useNavigate();
  const controller = useRoomPageController({
    roomId: params.roomId,
    conversationId: params.conversationId,
  });

  const handleBackToLauncher = useCallback(() => {
    controller.handle_back_to_directory();
    navigate(AppRouteBuilders.launcher());
  }, [controller, navigate]);

  const handleSelectAgent = useCallback((agentId: string) => {
    controller.handle_select_agent(agentId);
    navigate(AppRouteBuilders.room(agentId));
  }, [controller, navigate]);

  const handleSelectConversation = useCallback((conversationId: string) => {
    controller.handle_select_conversation(conversationId);
    const routeRoomId = controller.current_agent_id ?? params.roomId;
    if (routeRoomId) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, conversationId));
    }
  }, [controller, navigate, params.roomId]);

  const handleCreateConversation = useCallback(async () => {
    await controller.handle_create_conversation();
    const routeRoomId = controller.current_agent_id ?? params.roomId;
    const nextSessionKey = useSessionStore.getState().current_session_key;
    if (routeRoomId && nextSessionKey) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, nextSessionKey));
    }
  }, [controller, navigate, params.roomId]);

  if (!controller.isHydrated) {
    return <AppLoadingScreen />;
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
          is_session_busy={controller.is_session_busy}
          on_back_to_directory={handleBackToLauncher}
          on_close_workspace_pane={controller.handle_close_workspace_pane}
          on_delete_conversation={controller.handle_delete_conversation}
          on_edit_agent={controller.handle_edit_agent}
          on_loading_change={controller.setIsSessionBusy}
          on_create_conversation={handleCreateConversation}
          on_open_create_agent={controller.handle_open_create_agent}
          on_open_workspace_file={controller.handle_open_workspace_file}
          on_select_agent={handleSelectAgent}
          on_select_conversation={handleSelectConversation}
          on_conversation_snapshot_change={controller.handle_conversation_snapshot_change}
          on_start_editor_resize={controller.handle_start_editor_resize}
          on_todos_change={controller.setCurrentTodos}
          recent_agents={controller.recent_agents}
          session_cost_summary={controller.session_cost_summary}
          workspace_split_ref={controller.workspace_split_ref}
        />

        <AgentOptions
        mode={controller.dialogMode}
          isOpen={controller.isDialogOpen}
          onClose={() => controller.setIsDialogOpen(false)}
          onSave={controller.handle_save_agent_options}
          onValidateName={controller.handle_validate_agent_name}
          initialTitle={controller.dialog_initial_title}
          initialOptions={controller.dialog_initial_options}
        />
      </AppStage>
    );
  }

  return (
    <RouteScaffold
      badge="ROOM"
      title="协作空间骨架"
      description="这里会成为真正的 room 页面。下一阶段会把当前 workspace 逐步迁入这里，并把左侧从 Rooms 改成 Conversations，把成员、上下文和推进状态都按 room-first 组织。"
      meta={
        <div className="flex gap-3">
          <div className="workspace-card rounded-[20px] px-4 py-3 text-right">
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              Room
            </p>
            <p className="mt-1 text-sm font-semibold text-slate-950/84">{params.roomId ?? "-"}</p>
          </div>
          {params.conversationId ? (
            <div className="workspace-card rounded-[20px] px-4 py-3 text-right">
              <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
                Conversation
              </p>
              <p className="mt-1 text-sm font-semibold text-slate-950/84">{params.conversationId}</p>
            </div>
          ) : null}
        </div>
      }
    >
      <div className="grid flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
        <aside className="workspace-card rounded-[28px] px-5 py-5">
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
            Conversations
          </p>
          <p className="mt-3 text-sm leading-6 text-slate-700/60">
            当前阶段先建立 room 路由入口。下一阶段这里会取代今天错误的 Rooms 列表，明确只展示当前 room 下的 conversations。
          </p>
        </aside>

        <section className="workspace-card rounded-[28px] px-5 py-5">
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
            Collaboration Flow
          </p>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700/60">
            下一阶段会把现在的 chat/workspace 内容迁入这里，并拆出 1v1 room 与多人 room 的差异化表达。输入区会收敛成真正的 room composer，不再包含 Agent / Room / Ask App 模式切换。
          </p>
        </section>

        <aside className="workspace-card rounded-[28px] px-5 py-5">
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
            Members & Context
          </p>
          <p className="mt-3 text-sm leading-6 text-slate-700/60">
            右侧会根据单成员 room 和多成员 room 展示不同的信息层次，包括成员状态、任务推进和上下文边界。
          </p>
        </aside>
      </div>
    </RouteScaffold>
  );
}

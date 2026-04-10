import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useAgentConversation } from "@/hooks/agent";
import { LauncherAppConversationPanel } from "@/features/launcher/launcher-app-conversation-panel";
import { LauncherConsole } from "@/features/launcher/launcher-console";
import { getLauncherSurfaceThemeStyle } from "@/features/launcher/launcher-surface-theme";
import { useLauncherPageController } from "@/hooks/use-launcher-page-controller";
import { deleteConversation } from "@/lib/agent-api";
import { createRoom, ensureDirectRoom } from "@/lib/room-api";
import { cn } from "@/lib/utils";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import { AgentOptions as AgentConfigOptions } from "@/types/agent";
import { getSessionControlStatusText } from "@/types/agent-conversation";
import { getDefaultAgentId } from "@/config/options";

export function LauncherPage() {
  const { t } = useI18n();
  const { theme } = useTheme();
  const app_agent_id = getDefaultAgentId();
  const controller = useLauncherPageController();
  const navigate = useNavigate();
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const [should_bootstrap_room_after_create, set_should_bootstrap_room_after_create] = useState(false);
  const [pending_room_title, set_pending_room_title] = useState<string>("");
  const [pending_delete_agent, set_pending_delete_agent] = useState<{ id: string; name: string } | null>(null);
  const consumed_route_prompt_ref = useRef<string | null>(null);
  const app_panel_load_signature_ref = useRef<string | null>(null);
  const pending_app_prompt_ref = useRef<string | null>(null);
  const app_conversation_identity = useMemo(() => ({
    session_key: controller.app_session_key,
    agent_id: app_agent_id,
    chat_type: "dm" as const,
  }), [app_agent_id, controller.app_session_key]);
  const app_conversation = useAgentConversation({
    identity: app_conversation_identity,
  });
  const {
    bind_session_key,
    load_session,
    send_message,
    clear_session,
    messages: app_conversation_messages,
    error: app_conversation_error,
    is_loading: app_conversation_loading,
    session_control_state,
    session_observer_count,
    ws_state: app_conversation_ws_state,
    pending_permissions,
    send_permission_response,
    stop_generation,
  } = app_conversation;
  const app_conversation_can_control = session_control_state !== "observer";
  const app_conversation_control_status_text = useMemo(
    () => getSessionControlStatusText(session_control_state, session_observer_count),
    [session_control_state, session_observer_count],
  );
  useEffect(() => {
    if (!controller.is_app_conversation_open) {
      app_panel_load_signature_ref.current = null;
      return;
    }

    if (!controller.is_hydrated) {
      return;
    }

    bind_session_key(controller.app_session_key);

    const next_load_signature = `${controller.app_session_key}:open`;
    if (app_panel_load_signature_ref.current === next_load_signature) {
      return;
    }

    app_panel_load_signature_ref.current = next_load_signature;
    void load_session(controller.app_session_key);
  }, [
    bind_session_key,
    controller.app_session_key,
    controller.is_hydrated,
    controller.is_app_conversation_open,
    load_session,
  ]);

  const flush_pending_app_prompt = useCallback(async () => {
    const pending_prompt = pending_app_prompt_ref.current?.trim() ?? "";
    if (!pending_prompt || app_conversation_ws_state !== "connected" || !app_conversation_can_control) {
      return;
    }

    pending_app_prompt_ref.current = null;
    try {
      await send_message(pending_prompt);
    } catch (error) {
      pending_app_prompt_ref.current = pending_prompt;
      controller.set_app_conversation_draft((current_draft) => (
        current_draft.trim() ? current_draft : pending_prompt
      ));
      throw error;
    }
  }, [app_conversation_can_control, app_conversation_ws_state, controller, send_message]);

  const handle_submit_app_conversation = useCallback(async (next_prompt: string) => {
    const trimmed_prompt = next_prompt.trim();
    if (!trimmed_prompt) {
      return;
    }

    pending_app_prompt_ref.current = trimmed_prompt;
    controller.set_app_conversation_draft("");
    controller.clear_route_app_prompt();

    try {
      bind_session_key(controller.app_session_key);
      await flush_pending_app_prompt();
      void controller.refresh_conversations();
    } catch (error) {
      pending_app_prompt_ref.current = null;
      controller.set_app_conversation_draft((current_draft) => (
        current_draft.trim() ? current_draft : trimmed_prompt
      ));
      throw error;
    }
  }, [bind_session_key, controller, flush_pending_app_prompt]);

  useEffect(() => {
    if (app_conversation_ws_state !== "connected" || !pending_app_prompt_ref.current || !app_conversation_can_control) {
      return;
    }

    void flush_pending_app_prompt();
  }, [app_conversation_can_control, app_conversation_ws_state, flush_pending_app_prompt]);

  useEffect(() => {
    if (app_conversation_ws_state !== "connected") {
      return;
    }
    if (!controller.route_app_prompt) {
      consumed_route_prompt_ref.current = null;
      return;
    }
    if (consumed_route_prompt_ref.current === controller.route_app_prompt) {
      return;
    }
    consumed_route_prompt_ref.current = controller.route_app_prompt;
    void handle_submit_app_conversation(controller.route_app_prompt);
  }, [
    app_conversation_ws_state,
    controller.route_app_prompt,
    handle_submit_app_conversation,
  ]);

  const handle_select_agent = useCallback((agent_id: string) => {
    void ensureDirectRoom(agent_id).then((context) => {
      controller.handle_select_agent(agent_id);
      set_active_panel_item(context.room.id);
      navigate(
        AppRouteBuilders.room_conversation(
          context.room.id,
          context.conversation.id,
        ),
      );
    });
  }, [controller, navigate, set_active_panel_item]);

  const handle_clear_app_session = useCallback(async () => {
    try {
      await deleteConversation(controller.app_session_key);
    } catch (error) {
      const message = error instanceof Error ? error.message : "";
      if (message !== "Session not found") {
        throw error;
      }
    }
    app_panel_load_signature_ref.current = null;
    clear_session();
    controller.set_app_conversation_draft("");
    await controller.refresh_conversations();
  }, [clear_session, controller]);

  const handle_save_agent_options = useCallback(async (title: string, options: AgentConfigOptions) => {
    const should_open_room_after_create = controller.dialog_mode === "create";
    await controller.handle_save_agent_options(title, options);

    if (!should_open_room_after_create) {
      set_pending_room_title("");
      set_should_bootstrap_room_after_create(false);
      return;
    }

    const next_agent_id = useAgentStore.getState().current_agent_id;
    if (!next_agent_id) {
      set_pending_room_title("");
      set_should_bootstrap_room_after_create(false);
      return;
    }

    if (!should_bootstrap_room_after_create) {
      set_pending_room_title("");
      set_should_bootstrap_room_after_create(false);
      const context = await ensureDirectRoom(next_agent_id);
      set_active_panel_item(context.room.id);
      navigate(
        AppRouteBuilders.room_conversation(
          context.room.id,
          context.conversation.id,
        ),
      );
      return;
    }

    const room_context = await createRoom({
      agent_ids: [next_agent_id],
      title: pending_room_title || title || t("launcher.new_collaboration"),
      name: pending_room_title || title || t("launcher.new_collaboration"),
    });
    set_pending_room_title("");
    set_should_bootstrap_room_after_create(false);
    set_active_panel_item(room_context.room.id);
    navigate(
      AppRouteBuilders.room_conversation(
        room_context.room.id,
        room_context.conversation.id,
      ),
    );
  }, [controller, navigate, pending_room_title, set_active_panel_item, should_bootstrap_room_after_create, t]);

  const handle_request_delete_agent = useCallback((agent_id: string) => {
    const target_agent = controller.agents.find((agent) => agent.agent_id === agent_id);
    set_pending_room_title("");
    set_should_bootstrap_room_after_create(false);
    controller.set_is_dialog_open(false);
    set_pending_delete_agent({
      id: agent_id,
      name: target_agent?.name ?? "该 Agent",
    });
  }, [controller]);

  const handle_confirm_delete_agent = useCallback(async () => {
    if (!pending_delete_agent) {
      return;
    }

    await controller.handle_delete_agent(pending_delete_agent.id);
    set_pending_delete_agent(null);
  }, [controller, pending_delete_agent]);

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  return (
    <>
      <div
        className="relative flex min-h-0 flex-1 gap-1 overflow-hidden"
        style={getLauncherSurfaceThemeStyle(theme)}
      >
        <div
          className={cn(
            "flex min-w-0 flex-1 transition-[transform,opacity,filter] duration-500 ease-out",
            controller.surface === "app" && "saturate-[0.96]",
          )}
          onClick={() => controller.is_app_conversation_open && controller.close_app_conversation()}
        >
          <LauncherConsole
            app_conversation_draft={controller.app_conversation_draft}
            app_conversation_loading={app_conversation_loading}
            app_conversation_can_control={app_conversation_can_control}
            app_conversation_control_status_text={app_conversation_control_status_text}
            agents={controller.agents}
            conversations={controller.conversations}
            rooms={controller.rooms}
            current_agent_id={controller.current_agent_id}
            on_change_app_conversation_draft={controller.set_app_conversation_draft}
            is_app_conversation_open={controller.is_app_conversation_open}
            on_open_app_conversation={controller.open_app_conversation}
            on_close_app_conversation={controller.close_app_conversation}
            on_select_agent={handle_select_agent}
            on_stop_app_conversation={stop_generation}
            on_submit_app_conversation={handle_submit_app_conversation}
            surface={controller.surface}
          />
        </div>

        {controller.is_app_conversation_open ? (
          <div className="absolute inset-x-3 bottom-4 top-24 z-40 lg:static lg:inset-auto lg:block lg:w-[512px] lg:shrink-0 lg:pb-8">
            <div
              className={cn(
                "h-full transition-[transform,opacity,filter] duration-500 ease-out",
                controller.is_app_conversation_open
                  ? "translate-x-0 scale-100 opacity-100 blur-0"
                  : "translate-x-[34px] scale-[0.94] opacity-0 blur-[8px]",
              )}
            >
              <LauncherAppConversationPanel
                app_conversation_messages={app_conversation_messages}
                error={app_conversation_error}
                is_info_mode={controller.surface === "app"}
                is_loading={app_conversation_loading}
                session_key={controller.app_session_key}
                ws_state={app_conversation_ws_state}
                can_respond_to_permissions={app_conversation_can_control}
                permission_read_only_reason="当前窗口是观察视图，控制权在另一窗口"
                on_clear_session={handle_clear_app_session}
                on_close={controller.close_app_conversation}
                on_permission_response={send_permission_response}
                pending_permissions={pending_permissions}
              />
            </div>
          </div>
        ) : null}
      </div>

      <AgentOptions
        agent_id={controller.editing_agent_id ?? undefined}
        mode={controller.dialog_mode}
        is_open={controller.is_dialog_open}
        on_close={() => {
          set_pending_room_title("");
          set_should_bootstrap_room_after_create(false);
          controller.set_is_dialog_open(false);
        }}
        on_delete={handle_request_delete_agent}
        on_save={handle_save_agent_options}
        on_validate_name={controller.handle_validate_agent_name}
        initial_title={
          should_bootstrap_room_after_create
            ? pending_room_title || controller.dialog_initial_title
            : controller.dialog_initial_title
        }
        initial_options={controller.dialog_initial_options}
      />

      <ConfirmDialog
        confirm_text="删除成员"
        is_open={Boolean(pending_delete_agent)}
        message={`删除「${pending_delete_agent?.name ?? "该 Agent"}」后，该成员将不再出现在当前前端列表中。已有历史协作不会自动删除。`}
        on_cancel={() => set_pending_delete_agent(null)}
        on_confirm={() => {
          void handle_confirm_delete_agent();
        }}
        title="删除成员"
        variant="danger"
      />
    </>
  );
}

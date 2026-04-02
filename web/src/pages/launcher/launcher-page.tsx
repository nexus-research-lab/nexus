import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { useAgentConversation } from "@/hooks/agent";
import { LauncherAppConversationPanel } from "@/features/launcher/launcher-app-conversation-panel";
import { LauncherConsole } from "@/features/launcher/launcher-console";
import { useLauncherPageController } from "@/hooks/use-launcher-page-controller";
import { createConversation, deleteConversation } from "@/lib/agent-api";
import { buildWsDmSessionKey } from "@/lib/session-key";
import { createRoom, ensureDirectRoom } from "@/lib/room-api";
import { cn } from "@/lib/utils";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { AgentOptions as AgentConfigOptions } from "@/types/agent";
import { getDefaultAgentId } from "@/config/options";

export function LauncherPage() {
  const app_agent_id = getDefaultAgentId();
  const controller = useLauncherPageController();
  const navigate = useNavigate();
  const [should_bootstrap_room_after_create, set_should_bootstrap_room_after_create] = useState(false);
  const [pending_room_title, set_pending_room_title] = useState<string>("");
  const consumed_route_prompt_ref = useRef<string | null>(null);
  const skip_app_session_load_ref = useRef<string | null>(null);
  const hydrated_app_session_key_ref = useRef<string | null>(null);
  const app_conversation = useAgentConversation({
    agent_id: app_agent_id,
  });

  useEffect(() => {
    const next_session_key = controller.app_session_key;

    if (!next_session_key) {
      hydrated_app_session_key_ref.current = null;
      app_conversation.bind_session_key(null);
      return;
    }

    if (skip_app_session_load_ref.current === next_session_key) {
      skip_app_session_load_ref.current = null;
      hydrated_app_session_key_ref.current = next_session_key;
      app_conversation.bind_session_key(next_session_key);
      return;
    }

    if (!controller.is_app_conversation_open) {
      return;
    }

    if (hydrated_app_session_key_ref.current === next_session_key) {
      return;
    }

    hydrated_app_session_key_ref.current = next_session_key;
    app_conversation.bind_session_key(next_session_key);
    void app_conversation.load_session(next_session_key);
  }, [
    app_conversation,
    controller.app_session_key,
    controller.is_app_conversation_open,
  ]);

  const ensure_app_session_key = useCallback(async () => {
    if (controller.app_session_key) {
      app_conversation.bind_session_key(controller.app_session_key);
      return controller.app_session_key;
    }

    const existing_app_conversation = controller.conversations
      .filter((conversation) => conversation.agent_id === app_agent_id)
      .sort((left, right) => right.last_activity_at - left.last_activity_at)[0];

    if (existing_app_conversation) {
      controller.set_app_session_key(existing_app_conversation.session_key);
      app_conversation.bind_session_key(existing_app_conversation.session_key);
      return existing_app_conversation.session_key;
    }

    const created_conversation = await createConversation(
      buildWsDmSessionKey(`launcher-app-${app_agent_id}`, app_agent_id),
      {
        agent_id: app_agent_id,
        title: "Nexus",
      },
    );
    skip_app_session_load_ref.current = created_conversation.session_key;
    controller.set_app_session_key(created_conversation.session_key);
    app_conversation.bind_session_key(created_conversation.session_key);
    return created_conversation.session_key;
  }, [app_agent_id, app_conversation, controller]);

  const handle_submit_app_conversation = useCallback(async (next_prompt: string) => {
    const trimmed_prompt = next_prompt.trim();
    if (!trimmed_prompt) {
      return;
    }

    controller.set_app_conversation_draft("");
    controller.clear_route_app_prompt();

    try {
      const session_key = await ensure_app_session_key();
      app_conversation.bind_session_key(session_key);
      await app_conversation.send_message(trimmed_prompt);
    } catch (error) {
      // 发送失败时，仅在输入框仍为空时回填，避免覆盖用户新输入。
      controller.set_app_conversation_draft((current_draft) => (
        current_draft.trim() ? current_draft : trimmed_prompt
      ));
      throw error;
    }
  }, [app_conversation, controller, ensure_app_session_key]);

  useEffect(() => {
    if (app_conversation.ws_state !== "connected") {
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
    app_conversation.ws_state,
    controller.route_app_prompt,
    handle_submit_app_conversation,
  ]);

  const handle_select_agent = useCallback((agent_id: string) => {
    void ensureDirectRoom(agent_id).then((context) => {
      controller.handle_select_agent(agent_id);
      navigate(
        AppRouteBuilders.room_conversation(
          context.room.id,
          context.conversation.id,
        ),
      );
    });
  }, [controller, navigate]);

  const handle_clear_app_session = useCallback(async () => {
    if (controller.app_session_key) {
      await deleteConversation(controller.app_session_key);
    }
    hydrated_app_session_key_ref.current = null;
    skip_app_session_load_ref.current = null;
    controller.clear_app_session_key();
    app_conversation.clear_session();
    controller.set_app_conversation_draft("");
  }, [app_conversation, controller]);

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
      title: pending_room_title || title || "新协作",
      name: pending_room_title || title || "新协作",
    });
    set_pending_room_title("");
    set_should_bootstrap_room_after_create(false);
    navigate(
      AppRouteBuilders.room_conversation(
        room_context.room.id,
        room_context.conversation.id,
      ),
    );
  }, [controller, navigate, pending_room_title, should_bootstrap_room_after_create]);

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  return (
    <>
      <div className="relative flex min-h-0 flex-1 gap-1 overflow-hidden">
        <div
          className={cn(
            "pointer-events-none absolute inset-y-[8%] right-[22%] z-10 hidden w-[34%] rounded-full bg-[radial-gradient(circle,rgba(174,208,255,0.24),rgba(174,208,255,0.08)_36%,transparent_76%)] blur-3xl transition-all duration-500 lg:block",
            controller.is_app_conversation_open
              ? "translate-x-0 opacity-100"
              : "translate-x-10 opacity-0",
          )}
        />
        <div
          className="launcher-surface-bridge hidden lg:block"
          data-open={controller.is_app_conversation_open ? "true" : "false"}
        />

        <div
          className={cn(
            "launcher-surface-left min-w-0 flex-1",
            controller.is_app_conversation_open &&
            "lg:max-w-[calc(100%-430px)] lg:scale-[0.985] lg:opacity-[0.96]",
          )}
          data-surface={controller.surface}
          onClick={() => controller.is_app_conversation_open && controller.close_app_conversation()}
        >
          <LauncherConsole
            agents={controller.agents}
            conversations={controller.conversations}
            current_agent_id={controller.current_agent_id}
            is_app_conversation_open={controller.is_app_conversation_open}
            on_open_app_conversation={controller.open_app_conversation}
            on_close_app_conversation={controller.close_app_conversation}
            on_select_agent={handle_select_agent}
            surface={controller.surface}
          />
        </div>

        {controller.is_app_conversation_open ? (
          <div className="absolute inset-x-3 bottom-4 top-24 z-40 lg:static lg:inset-auto lg:block lg:w-[420px] lg:shrink-0 lg:pb-8">
            <div
              className="launcher-app-panel-shell h-full"
              data-open={controller.is_app_conversation_open ? "true" : "false"}
            >
              <LauncherAppConversationPanel
                app_conversation_draft={controller.app_conversation_draft}
                app_conversation_messages={app_conversation.messages}
                error={app_conversation.error}
                is_loading={app_conversation.is_loading}
                ws_state={app_conversation.ws_state}
                on_clear_session={handle_clear_app_session}
                on_change_draft={controller.set_app_conversation_draft}
                on_close={controller.close_app_conversation}
                on_permission_response={app_conversation.send_permission_response}
                on_stop_generation={app_conversation.stop_generation}
                on_submit={handle_submit_app_conversation}
                pending_permission={app_conversation.pending_permission}
              />
            </div>
          </div>
        ) : null}
      </div>

      <AgentOptions
        mode={controller.dialog_mode}
        is_open={controller.is_dialog_open}
        on_close={() => {
          set_pending_room_title("");
          set_should_bootstrap_room_after_create(false);
          controller.set_is_dialog_open(false);
        }}
        on_save={handle_save_agent_options}
        on_validate_name={controller.handle_validate_agent_name}
        initial_title={
          should_bootstrap_room_after_create
            ? pending_room_title || controller.dialog_initial_title
            : controller.dialog_initial_title
        }
        initial_options={controller.dialog_initial_options}
      />
    </>
  );
}

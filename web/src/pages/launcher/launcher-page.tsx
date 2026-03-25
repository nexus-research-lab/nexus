import { useCallback, useState } from "react";
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
import { useAppConversationStore } from "@/store/app-conversation";
import { getConversationStoreSnapshot } from "@/store/conversation";
import { AgentOptions as AgentConfigOptions } from "@/types/agent";

function build_room_title_from_prompt(prompt: string | undefined) {
  const normalized_prompt = (prompt ?? "")
    .trim()
    .replace(/[。！？!?,，；;：:\s]+$/g, "")
    .replace(/^(帮我|请|我想|我要|我需要|帮忙|麻烦你)/, "")
    .replace(/^(创建|新建|开始|组织|整理)(一个|一条|个)?/, "")
    .replace(/(协作|room|Room|任务)$/g, "")
    .trim();

  if (!normalized_prompt) {
    return "新协作";
  }

  return normalized_prompt.slice(0, 24);
}

export function LauncherPage() {
  const controller = useLauncherPageController();
  const navigate = useNavigate();
  const [should_bootstrap_room_after_create, set_should_bootstrap_room_after_create] = useState(false);
  const [pending_room_title, set_pending_room_title] = useState<string>("");
  const push_app_message = useAppConversationStore((state) => state.push_app_message);
  const latest_user_prompt = [...controller.app_conversation_messages]
    .reverse()
    .find((message) => message.role === "user")?.body;

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

  const handleOpenContactsPageFromApp = useCallback(() => {
    push_app_message("正在打开 Contacts。");
    handleOpenContactsPage();
  }, [handleOpenContactsPage, push_app_message]);

  const conversations_with_owners = controller.conversations
    .map((conversation) => ({
      conversation,
      owner: conversation.agent_id
        ? controller.agents.find((agent) => agent.agent_id === conversation.agent_id) ?? null
        : null,
    }))
    .sort((left, right) => right.conversation.last_activity_at - left.conversation.last_activity_at);

  const handleCreateRoom = useCallback(() => {
    const next_room_title = build_room_title_from_prompt(latest_user_prompt);
    push_app_message("我会先创建一个新的协作 room，并在完成后直接带你进入第一条对话。");
    set_pending_room_title(next_room_title);
    set_should_bootstrap_room_after_create(true);
    controller.handle_open_create_agent();
  }, [controller, latest_user_prompt, push_app_message]);

  const handleCreateAgent = useCallback(() => {
    set_pending_room_title("");
    set_should_bootstrap_room_after_create(false);
    controller.handle_open_create_agent();
  }, [controller]);

  const handleOpenConversationFromApp = useCallback((conversation_id: string, agent_id?: string) => {
    const matched_conversation = conversations_with_owners.find(
      ({ conversation }) => conversation.session_key === conversation_id,
    );
    const title = matched_conversation?.conversation.title || "最近协作";
    push_app_message(`正在带你回到“${title}”，继续已有协作。`);
    handleOpenConversation(conversation_id, agent_id);
  }, [conversations_with_owners, handleOpenConversation, push_app_message]);

  const handleOpenAgentRoomFromApp = useCallback((agent_id: string) => {
    const matched_agent = controller.agents.find((agent) => agent.agent_id === agent_id);
    push_app_message(
      `正在带你进入 ${matched_agent?.name ?? "目标成员"} 的协作 room。`,
    );
    handleSelectAgent(agent_id);
  }, [controller.agents, handleSelectAgent, push_app_message]);

  const handleSaveAgentOptions = useCallback(async (title: string, options: AgentConfigOptions) => {
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
      navigate(AppRouteBuilders.room(next_agent_id));
      return;
    }

    const conversation_store = getConversationStoreSnapshot();
    const next_conversation_id = await conversation_store.create_conversation({
      title: "New Chat",
      agent_id: next_agent_id,
    });
    conversation_store.set_current_conversation(next_conversation_id);
    push_app_message(`新的协作 room 已创建完成，正在进入 ${title} 的第一条对话。`);
    set_pending_room_title("");
    set_should_bootstrap_room_after_create(false);
    navigate(AppRouteBuilders.room_conversation(next_agent_id, next_conversation_id));
  }, [controller, navigate, push_app_message, should_bootstrap_room_after_create]);

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  return (
    <AppStage>
      <div className="relative flex min-h-0 flex-1 gap-5 overflow-hidden">
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
              "lg:max-w-[calc(100%-390px)] lg:-translate-x-6 lg:scale-[0.985] lg:opacity-[0.96]",
          )}
          data-surface={controller.surface}
        >
          <LauncherConsole
            agents={controller.agents}
            conversations={controller.conversations}
            current_agent_id={controller.current_agent_id}
            on_open_contacts_page={handleOpenContactsPage}
            on_open_app_conversation={controller.open_app_conversation}
            on_select_agent={handleSelectAgent}
            on_open_conversation={handleOpenConversation}
            on_create_agent={handleCreateAgent}
            on_edit_agent={controller.handle_edit_agent}
            on_delete_agent={controller.handle_delete_agent}
            surface={controller.surface}
          />
        </div>

        {controller.is_app_conversation_open ? (
          <div className="absolute inset-x-3 bottom-4 top-[96px] z-40 lg:static lg:inset-auto lg:block lg:w-[380px] lg:shrink-0 lg:pb-8 lg:pt-4">
            <div
              className="launcher-app-panel-shell h-full"
              data-open={controller.is_app_conversation_open ? "true" : "false"}
            >
              <LauncherAppConversationPanel
                agents={controller.agents}
                app_conversation_draft={controller.app_conversation_draft}
              app_conversation_messages={controller.app_conversation_messages}
              conversations_with_owners={conversations_with_owners}
              on_create_room={handleCreateRoom}
              on_clear_conversation={controller.clear_app_conversation}
                on_change_draft={controller.set_app_conversation_draft}
                on_close={controller.close_app_conversation}
              on_open_agent_room={handleOpenAgentRoomFromApp}
              on_open_conversation={handleOpenConversationFromApp}
              on_open_contacts_page={handleOpenContactsPageFromApp}
              on_submit={controller.submit_app_conversation}
              suggested_room_title={build_room_title_from_prompt(latest_user_prompt)}
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
        on_save={handleSaveAgentOptions}
        on_validate_name={controller.handle_validate_agent_name}
        initial_title={
          should_bootstrap_room_after_create
            ? pending_room_title || controller.dialog_initial_title
            : controller.dialog_initial_title
        }
        initial_options={controller.dialog_initial_options}
      />
    </AppStage>
  );
}

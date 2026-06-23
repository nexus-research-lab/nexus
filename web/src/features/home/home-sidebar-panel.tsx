/**
 * 聊天式侧边栏内容。
 *
 * 左侧面板从导航树收敛为三个真实工作入口：
 * - 聊天：统一承载 Room 与 DM。
 * - 联系人：管理 Agent，并提供发起 DM 的快捷动作。
 * - 能力：由侧边栏顶层 Tab 承载，不再混在聊天列表里。
 */

import {
  MessageSquarePlus,
  Plus,
  UserPlus,
  Users2,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { create_room, delete_room } from "@/lib/api/room-api";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import {
  build_chat_notification_target_key,
  get_active_chat_target_from_path,
} from "./chat-notification-target";
import {
  build_conversation_items,
  build_sidebar_item_notification_key,
  get_sidebar_item_unread_state,
  is_active_sidebar_chat_item,
  is_main_agent_dm_room,
  normalize_query,
  type SidebarConversationItem,
} from "./home-sidebar-conversation-model";
import { useSidebarDirectory } from "./home-sidebar-directory";
import {
  ContactRow,
  ConversationRow,
  SidebarListLoadingRows,
  SidebarSearchField,
} from "./home-sidebar-list-rows";

interface DeleteTarget {
  id: string;
  name: string;
  room_type: "room" | "dm";
}

export const ChatSidebarPanelContent = memo(function ChatSidebarPanelContent() {
  const { t } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);
  const chat_unread_counts = useSidebarStore((s) => s.chat_unread_counts);
  const chat_unread_targets = useSidebarStore((s) => s.chat_unread_targets);
  const chat_unread_timestamps = useSidebarStore((s) => s.chat_unread_timestamps);
  const clear_chat_notifications_for_target = useSidebarStore(
    (s) => s.clear_chat_notifications_for_target,
  );
  const clear_chat_notifications_for_room = useSidebarStore(
    (s) => s.clear_chat_notifications_for_room,
  );
  const set_nexus_room_id = useSidebarStore((s) => s.set_nexus_room_id);
  const agent_runtime_statuses = useAgentStore((s) => s.agent_runtime_statuses);
  const { agents, conversations, is_loading, refresh_directory, rooms } = useSidebarDirectory();
  const [query, set_query] = useState("");
  const [delete_target, set_delete_target] = useState<DeleteTarget | null>(null);
  const [is_create_room_open, set_is_create_room_open] = useState(false);
  const [is_creating_room, set_is_creating_room] = useState(false);
  const untitled_room_label = t("home.untitled_room");
  const has_agents = agents.length > 0;

  const nexus_dm_room = useMemo(
    () => rooms.find((room) => is_main_agent_dm_room(room)) ?? null,
    [rooms],
  );
  const active_chat_target = useMemo(
    () => get_active_chat_target_from_path(location.pathname),
    [location.pathname],
  );

  useEffect(() => {
    set_nexus_room_id(nexus_dm_room?.id ?? null);
  }, [nexus_dm_room, set_nexus_room_id]);

  const raw_items = useMemo(
    () => build_conversation_items({
      agents,
      agent_runtime_statuses,
      conversations,
      format_running_tasks_summary: (count) => t("sidebar.running_tasks_summary", { count }),
      rooms,
      untitled_room_label,
    }).map((item) => {
      const notification_key = build_sidebar_item_notification_key(item);
      const unread_state = get_sidebar_item_unread_state({
        chat_unread_counts,
        chat_unread_targets,
        chat_unread_timestamps,
        notification_key,
        room_id: item.room_id,
        session_key: item.session_key,
      });
      return {
        ...item,
        notification_key,
        ...unread_state,
      };
    }),
    [
      agents,
      agent_runtime_statuses,
      chat_unread_counts,
      chat_unread_targets,
      chat_unread_timestamps,
      conversations,
      rooms,
      t,
      untitled_room_label,
    ],
  );
  const items = useMemo(
    () => raw_items.map((item) => {
      const visible_unread_state = is_active_sidebar_chat_item(item, active_chat_target)
        ? {
          unread_conversation_id: null,
          unread_count: 0,
          unread_target_key: null,
        }
        : {
          unread_conversation_id: item.unread_conversation_id ?? null,
          unread_count: item.unread_count ?? 0,
          unread_target_key: item.unread_target_key ?? null,
        };
      return {
        ...item,
        ...visible_unread_state,
      };
    }),
    [active_chat_target, raw_items],
  );

  const filtered_items = useMemo(() => {
    const normalized_query = normalize_query(query);
    if (!normalized_query) {
      return items;
    }
    return items.filter((item) => {
      const member_names = item.members.map((member) => member.name).join(" ");
      return `${item.title} ${item.summary} ${member_names}`.toLowerCase().includes(normalized_query);
    });
  }, [items, query]);

  const navigate_to_room = useCallback(async (item: SidebarConversationItem) => {
    const route_room_id = item.route_room_id ?? item.room_id;
    if (!route_room_id) {
      return;
    }
    const target_conversation_id = item.unread_conversation_id || item.conversation_id;
    if (item.room_id) {
      clear_chat_notifications_for_room(item.room_id);
    }
    clear_chat_notifications_for_target(item.unread_target_key || item.notification_key);
    set_active_item(item.id);
    if (target_conversation_id) {
      navigate(AppRouteBuilders.room_conversation(route_room_id, target_conversation_id));
      return;
    }
    navigate(AppRouteBuilders.room(route_room_id));
  }, [
    clear_chat_notifications_for_room,
    clear_chat_notifications_for_target,
    navigate,
    set_active_item,
  ]);

  const handle_create_room = useCallback(() => {
    set_is_create_room_open(true);
  }, []);

  const handle_confirm_create_room = useCallback(async (
    agent_ids: string[],
    name: string,
    avatar?: string,
    skill_names?: string[],
    host_agent_id?: string | null,
    host_auto_reply_enabled?: boolean,
    private_messages_enabled?: boolean,
  ) => {
    set_is_creating_room(true);
    try {
      const context = await create_room({
        agent_ids,
        name,
        avatar,
        skill_names,
        host_agent_id,
        host_auto_reply_enabled,
        private_messages_enabled,
      });
      set_is_create_room_open(false);
      refresh_directory();
      navigate(AppRouteBuilders.room(context.room.id));
    } finally {
      set_is_creating_room(false);
    }
  }, [navigate, refresh_directory]);

  const handle_delete_room = useCallback(async (target: DeleteTarget) => {
    const deleted_room_id = target.id;
    await delete_room(deleted_room_id);
    if (active_item_id === deleted_room_id) {
      set_active_item(null);
    }
    refresh_directory();
  }, [active_item_id, refresh_directory, set_active_item]);

  const handle_confirm_delete_room = useCallback(() => {
    const target = delete_target;
    if (!target) {
      return;
    }

    set_delete_target(null);
    void handle_delete_room(target).catch((error) => {
      console.error("[Sidebar] Failed to delete room", error);
      refresh_directory();
    });
  }, [delete_target, handle_delete_room, refresh_directory]);

  const empty_description = has_agents
    ? t("home.rooms_empty_description")
    : t("home.rooms_empty_no_agents_description");
  const empty_action = has_agents
    ? t("home.rooms_empty_action")
    : t("home.rooms_empty_no_agents_action");
  const handle_empty_action = has_agents
    ? handle_create_room
    : () => navigate(AppRouteBuilders.contacts());

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-tour-anchor={SIDEBAR_TOUR_ANCHORS.chat_list}>
      <SidebarSearchField
        action={(
          <button
            className="flex h-9 w-9 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] text-(--icon-muted) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={handle_create_room}
            title={t("home.create_room")}
            type="button"
          >
            <Plus className="h-4 w-4" />
          </button>
        )}
        on_change={set_query}
        placeholder={t("sidebar.search_conversations")}
        value={query}
      />

      {is_loading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-1 px-2 pb-2">
          {filtered_items.length > 0 ? (
            filtered_items.map((item) => (
              <ConversationRow
                is_active={active_item_id === item.id || (item.room_id ? active_item_id === item.room_id : false)}
                item={item}
                key={item.id}
                on_click={() => {
                  void navigate_to_room(item);
                }}
                on_delete={item.can_delete && item.room_id ? () => set_delete_target({
                  id: item.room_id ?? item.id,
                  name: item.title,
                  room_type: item.kind,
                }) : undefined}
              />
            ))
          ) : (
            <SidebarEmptyGuide
              action_label={empty_action}
              description={empty_description}
              icon={MessageSquarePlus}
              on_action={handle_empty_action}
              title={query ? t("sidebar.no_matching_conversations") : t("home.rooms_empty_title")}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={delete_target !== null}
        message={t("home.delete_message", { name: delete_target?.name ?? "" })}
        on_cancel={() => set_delete_target(null)}
        on_confirm={handle_confirm_delete_room}
        title={t("home.delete_confirm")}
        variant="danger"
      />

      <CreateRoomDialog
        agents={agents.map((agent) => ({
          agent_id: agent.id,
          name: agent.name,
          avatar: agent.avatar,
        }))}
        is_creating={is_creating_room}
        is_open={is_create_room_open}
        on_cancel={() => set_is_create_room_open(false)}
        on_confirm={(ids, name, avatar, skill_names, host_agent_id, host_auto_reply_enabled, private_messages_enabled) =>
          void handle_confirm_create_room(ids, name, avatar, skill_names, host_agent_id, host_auto_reply_enabled, private_messages_enabled)}
      />
    </div>
  );
});

export const ContactsSidebarPanelContent = memo(function ContactsSidebarPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);
  const clear_chat_notifications_for_target = useSidebarStore(
    (s) => s.clear_chat_notifications_for_target,
  );
  const agent_runtime_statuses = useAgentStore((s) => s.agent_runtime_statuses);
  const { agents, is_loading } = useSidebarDirectory();
  const [query, set_query] = useState("");
  const active_agent_id = location.pathname === AppRouteBuilders.contacts()
    ? new URLSearchParams(location.search).get("agent")
    : null;

  const filtered_agents = useMemo(() => {
    const normalized_query = normalize_query(query);
    if (!normalized_query) {
      return agents;
    }
    return agents.filter((agent) => agent.name.toLowerCase().includes(normalized_query));
  }, [agents, query]);

  const navigate_to_contacts = useCallback(() => {
    set_active_item(null);
    if (location.pathname !== AppRouteBuilders.contacts() || location.search) {
      navigate(AppRouteBuilders.contacts());
    }
  }, [location.pathname, location.search, navigate, set_active_item]);

  const navigate_to_agent_detail = useCallback((agent_id: string) => {
    set_active_item(agent_id);
    navigate(AppRouteBuilders.contact_agent(agent_id));
  }, [navigate, set_active_item]);

  const navigate_to_agent_dm = useCallback(async (agent_id: string) => {
    const target = await resolve_direct_room_navigation_target(agent_id);
    clear_chat_notifications_for_target(build_chat_notification_target_key({
      conversation_id: target.context.conversation.id,
      room_id: target.context.room.id,
    }));
    set_active_item(target.context.room.id);
    navigate(target.route);
  }, [clear_chat_notifications_for_target, navigate, set_active_item]);

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-tour-anchor={SIDEBAR_TOUR_ANCHORS.contacts_list}>
      <SidebarSearchField
        action={(
          <button
            className="flex h-9 w-9 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] text-(--icon-muted) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={navigate_to_contacts}
            title={t("sidebar.manage_contacts")}
            type="button"
          >
            <UserPlus className="h-4 w-4" />
          </button>
        )}
        on_change={set_query}
        placeholder={t("sidebar.search_contacts")}
        value={query}
      />

      {is_loading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-1 px-2 pb-2">
          {filtered_agents.length > 0 ? (
            filtered_agents.map((agent) => {
              const running_task_count = agent_runtime_statuses[agent.id]?.running_task_count ?? 0;
              return (
                <ContactRow
                  agent={agent}
                  is_active={active_agent_id === agent.id}
                  is_working={running_task_count > 0}
                  key={agent.id}
                  on_chat={() => void navigate_to_agent_dm(agent.id)}
                  on_open_directory={() => navigate_to_agent_detail(agent.id)}
                  running_task_count={running_task_count}
                />
              );
            })
          ) : (
            <SidebarEmptyGuide
              action_label={t("sidebar.manage_contacts")}
              description={t("sidebar.contacts_empty_description")}
              icon={Users2}
              on_action={navigate_to_contacts}
              title={query ? t("sidebar.no_matching_contacts") : t("sidebar.no_contacts")}
            />
          )}
        </div>
      )}
    </div>
  );
});

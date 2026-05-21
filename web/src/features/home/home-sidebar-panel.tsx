/**
 * 聊天式侧边栏内容。
 *
 * 左侧面板从导航树收敛为三个真实工作入口：
 * - 聊天：统一承载 Room 与 DM。
 * - 联系人：管理 Agent，并提供发起 DM 的快捷动作。
 * - 能力：由侧边栏顶层 Tab 承载，不再混在聊天列表里。
 */

import {
  Hash,
  MessageCircle,
  MessageSquarePlus,
  Plus,
  Search,
  Trash2,
  UserPlus,
  Users2,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { get_agent_ws_url, is_main_agent } from "@/config/options";
import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import { create_room, delete_room, subscribe_room_directory_updates } from "@/lib/api/room-api";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { cn, get_icon_avatar_src, get_room_avatar_icon_id } from "@/lib/utils";
import { useWebSocket } from "@/lib/websocket";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { AGENT_LIST_UPDATED_EVENT_NAME, useAgentStore } from "@/store/agent";
import {
  type ChatNotificationTargetState,
  useSidebarStore,
} from "@/store/sidebar";
import {
  build_chat_notification_target_key,
  get_active_chat_target_from_path,
  is_chat_notification_target_active,
  type ActiveChatNotificationTarget,
} from "./chat-notification-target";
import type { AgentRuntimeStatus } from "@/types/agent/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomMemberSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import type { EventMessage } from "@/types/conversation/message";

interface SidebarDirectoryState {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
  is_loading: boolean;
  refresh_directory: () => void;
}

interface SidebarConversationItem {
  id: string;
  kind: "room" | "dm";
  title: string;
  summary: string;
  time_label: string;
  members: LauncherRoomMemberSummary[];
  avatar?: string | null;
  room_id?: string;
  conversation_id?: string;
  session_key?: string;
  agent_id?: string;
  last_activity_at: number;
  message_count: number;
  notification_key?: string | null;
  running_task_count: number;
  unread_conversation_id?: string | null;
  unread_count?: number;
  unread_target_key?: string | null;
  can_delete: boolean;
}

interface DeleteTarget {
  id: string;
  name: string;
  room_type: "room" | "dm";
}

const SIDEBAR_ROW_CLASS_NAME =
  "group/item relative flex min-h-[68px] w-full cursor-pointer items-center gap-3 rounded-[14px] px-3 py-2.5 text-left transition-[background,color,transform] duration-(--motion-duration-fast)";

interface SidebarDirectorySnapshot {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
}

let sidebar_directory_cache: SidebarDirectorySnapshot | null = null;

function normalize_query(value: string): string {
  return value.trim().toLowerCase();
}

function is_active_sidebar_chat_item(
  item: SidebarConversationItem,
  active_target: ActiveChatNotificationTarget | null,
): boolean {
  return is_chat_notification_target_active(active_target, {
    key: item.notification_key,
    room_id: item.room_id,
  });
}

function to_timestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function format_sidebar_time(timestamp: number): string {
  if (!timestamp) {
    return "";
  }

  const date = new Date(timestamp);
  const now = new Date();
  const today_start = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const item_day_start = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const day_delta = Math.floor((today_start - item_day_start) / 86400000);

  if (day_delta <= 0) {
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }
  if (day_delta === 1) {
    return "昨天";
  }
  if (day_delta < 7) {
    return `周${"日一二三四五六"[date.getDay()]}`;
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
}

function render_agent_avatar(
  name: string,
  avatar?: string | null,
  class_name?: string,
) {
  const avatar_src = get_icon_avatar_src(avatar);
  if (avatar_src) {
    return (
      <img
        alt={name}
        className={cn("h-full w-full rounded-full object-cover", class_name)}
        src={avatar_src}
      />
    );
  }

  return (
    <span
      className={cn(
        "flex h-full w-full items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[12px] font-semibold text-(--text-strong) shadow-(--surface-avatar-shadow)",
        class_name,
      )}
    >
      {name.trim().slice(0, 1).toUpperCase()}
    </span>
  );
}

function CompositeRoomAvatar({
  avatar,
  members,
  title,
}: {
  avatar?: string | null;
  members: LauncherRoomMemberSummary[];
  title: string;
}) {
  const visible_members = members.slice(0, 4);
  if (visible_members.length === 0) {
    const room_avatar_id = get_room_avatar_icon_id(title, title, avatar ?? undefined);
    const room_avatar_src = get_icon_avatar_src(room_avatar_id, "room");
    return (
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-muted) shadow-(--surface-avatar-shadow)">
        {room_avatar_src ? (
          <img alt={title} className="h-full w-full rounded-[10px] object-cover" src={room_avatar_src} />
        ) : (
          <Hash className="h-4 w-4" />
        )}
      </span>
    );
  }

  return (
    <span
      className={cn(
        "grid h-10 w-10 shrink-0 gap-[2px] overflow-hidden rounded-[10px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_88%,white)] p-[2px] shadow-(--surface-avatar-shadow)",
        visible_members.length === 1 ? "grid-cols-1 grid-rows-1" : "grid-cols-2 grid-rows-2",
        visible_members.length === 2 && "grid-rows-1",
      )}
    >
      {visible_members.map((member) => (
        <span className="min-h-0 min-w-0 overflow-hidden rounded-[6px]" key={member.id}>
          {render_agent_avatar(member.name, member.avatar, "rounded-[6px]")}
        </span>
      ))}
    </span>
  );
}

function DirectAvatar({
  agent,
  is_working,
}: {
  agent: LauncherRoomMemberSummary;
  is_working: boolean;
}) {
  return (
    <span
      className={cn(
        "relative flex h-10 w-10 shrink-0 items-center justify-center rounded-full",
        is_working && "after:absolute after:inset-[-3px] after:rounded-full after:border after:border-[color:color-mix(in_srgb,var(--primary)_48%,transparent)] after:shadow-[0_0_0_3px_color-mix(in_srgb,var(--primary)_8%,transparent)]",
      )}
    >
      {render_agent_avatar(agent.name, agent.avatar)}
    </span>
  );
}

function SidebarSearchField({
  action,
  on_change,
  placeholder,
  value,
}: {
  action?: ReactNode;
  on_change: (value: string) => void;
  placeholder: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-2 px-2.5 pb-2">
      <label className="relative block min-w-0 flex-1">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-(--icon-muted)" />
        <input
          className="h-9 w-full rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] pl-8 pr-3 text-[13px] text-(--text-strong) outline-none transition-[border-color,background] duration-(--motion-duration-fast) placeholder:text-(--text-soft) focus:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_92%,transparent)] focus:bg-(--surface-elevated-background) focus:shadow-none"
          onChange={(event) => on_change(event.target.value)}
          placeholder={placeholder}
          type="search"
          value={value}
        />
      </label>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

function SidebarListLoadingRows({ count = 4 }: { count?: number }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-1 px-2 pb-2">
      {Array.from({ length: count }, (_, index) => (
        <div
          className="flex min-h-[68px] w-full items-center gap-3 rounded-[14px] px-3 py-2.5"
          key={index}
        >
          <span className="h-10 w-10 shrink-0 animate-pulse rounded-[10px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_74%,transparent)]" />
          <span className="min-w-0 flex-1 space-y-2">
            <span className="block h-3.5 w-24 animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_76%,transparent)]" />
            <span className="block h-3 w-36 animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)]" />
          </span>
        </div>
      ))}
    </div>
  );
}

function useSidebarDirectory(): SidebarDirectoryState {
  const ws_url = get_agent_ws_url();
  const apply_agent_runtime_status = useAgentStore((s) => s.apply_agent_runtime_status);
  const [agents, set_agents] = useState<LauncherAgentSummary[]>(() => sidebar_directory_cache?.agents ?? []);
  const [rooms, set_rooms] = useState<LauncherRoomSummary[]>(() => sidebar_directory_cache?.rooms ?? []);
  const [conversations, set_conversations] = useState<LauncherConversationSummary[]>(
    () => sidebar_directory_cache?.conversations ?? [],
  );
  const [is_loading, set_is_loading] = useState(sidebar_directory_cache === null);

  const refresh_directory = useCallback(() => {
    if (sidebar_directory_cache === null) {
      set_is_loading(true);
    }
    void get_launcher_bootstrap_api().then((payload) => {
      sidebar_directory_cache = {
        agents: payload.agents,
        rooms: payload.rooms,
        conversations: payload.conversations,
      };
      set_agents(payload.agents);
      set_rooms(payload.rooms);
      set_conversations(payload.conversations);
      set_is_loading(false);
    }).catch((error) => {
      console.error("[HomeSidebarPanel] 加载侧边栏目录失败:", error);
      if (sidebar_directory_cache === null) {
        set_agents([]);
        set_rooms([]);
        set_conversations([]);
      }
      set_is_loading(false);
    });
  }, []);

  useEffect(() => {
    refresh_directory();
  }, [refresh_directory]);

  useEffect(() => subscribe_room_directory_updates(refresh_directory), [refresh_directory]);

  useEffect(() => {
    window.addEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refresh_directory);
    return () => {
      window.removeEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refresh_directory);
    };
  }, [refresh_directory]);

  const agent_ids = useMemo(() => agents.map((agent) => agent.id), [agents]);
  const agent_id_set = useMemo(() => new Set(agent_ids), [agent_ids]);
  const handle_runtime_message = useCallback((message: unknown) => {
    const event = message as EventMessage;
    if (event.event_type !== "agent_runtime_event") {
      return;
    }
    if (!event.agent_id || !agent_id_set.has(event.agent_id)) {
      return;
    }
    const payload = event.data as AgentRuntimeStatus | undefined;
    if (!payload?.agent_id) {
      return;
    }
    apply_agent_runtime_status(payload);
  }, [agent_id_set, apply_agent_runtime_status]);

  const { state: runtime_ws_state, send: runtime_ws_send } = useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_runtime_message,
  });

  useEffect(() => {
    if (runtime_ws_state !== "connected" || agent_ids.length === 0) {
      return;
    }

    for (const agent_id of agent_ids) {
      runtime_ws_send({
        type: "subscribe_workspace",
        agent_id,
        watch_files: false,
      });
    }

    return () => {
      for (const agent_id of agent_ids) {
        runtime_ws_send({
          type: "unsubscribe_workspace",
          agent_id,
        });
      }
    };
  }, [agent_ids, runtime_ws_send, runtime_ws_state]);

  return {
    agents,
    rooms,
    conversations,
    is_loading,
    refresh_directory,
  };
}

function is_main_agent_dm_room(room: LauncherRoomSummary): boolean {
  if (room.room_type !== "dm") {
    return false;
  }
  return Boolean(room.dm_target_agent_id && is_main_agent(room.dm_target_agent_id));
}

function build_latest_conversation_by_room_id(
  conversations: LauncherConversationSummary[],
): Map<string, LauncherConversationSummary> {
  const result = new Map<string, LauncherConversationSummary>();
  for (const conversation of conversations) {
    if (!conversation.room_id) {
      continue;
    }
    const current = result.get(conversation.room_id);
    if (!current || to_timestamp(conversation.last_activity) > to_timestamp(current.last_activity)) {
      result.set(conversation.room_id, conversation);
    }
  }
  return result;
}

function is_launcher_conversation_active(
  conversation?: LauncherConversationSummary,
): boolean {
  if (!conversation) {
    return false;
  }
  return conversation.is_active === true || conversation.status === "active";
}

function running_task_count_for_sidebar_conversation({
  agent_runtime_statuses,
  dm_agent_id,
  is_dm,
  latest,
}: {
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  dm_agent_id?: string;
  is_dm: boolean;
  latest?: LauncherConversationSummary;
}): number {
  if (is_dm) {
    return dm_agent_id ? (agent_runtime_statuses[dm_agent_id]?.running_task_count ?? 0) : 0;
  }

  return is_launcher_conversation_active(latest) ? 1 : 0;
}

function build_conversation_items({
  agents,
  agent_runtime_statuses,
  conversations,
  format_running_tasks_summary,
  rooms,
  untitled_room_label,
}: {
  agents: LauncherAgentSummary[];
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  conversations: LauncherConversationSummary[];
  format_running_tasks_summary: (count: number) => string;
  rooms: LauncherRoomSummary[];
  untitled_room_label: string;
}): SidebarConversationItem[] {
  const agent_by_id = new Map(agents.map((agent) => [agent.id, agent]));
  const latest_by_room_id = build_latest_conversation_by_room_id(conversations);
  const items: SidebarConversationItem[] = [];

  for (const room of rooms) {
    if (is_main_agent_dm_room(room)) {
      continue;
    }
    const latest = latest_by_room_id.get(room.id);
    if (!latest) {
      continue;
    }
    const last_activity_at = to_timestamp(latest.last_activity);
    const is_dm = room.room_type === "dm";
    const dm_agent = room.dm_target_agent_id ? agent_by_id.get(room.dm_target_agent_id) : undefined;
    const members = is_dm
      ? dm_agent ? [{ id: dm_agent.id, name: dm_agent.name, avatar: dm_agent.avatar }] : []
      : room.members ?? [];
    const running_task_count = running_task_count_for_sidebar_conversation({
      agent_runtime_statuses,
      dm_agent_id: room.dm_target_agent_id,
      is_dm,
      latest,
    });
    const title = is_dm
      ? dm_agent?.name ?? room.name?.trim() ?? "DM"
      : room.name?.trim() || untitled_room_label;

    items.push({
      id: room.id,
      kind: is_dm ? "dm" : "room",
      title,
      summary: running_task_count > 0
        ? format_running_tasks_summary(running_task_count)
        : latest.title.trim(),
      time_label: format_sidebar_time(last_activity_at),
      members,
      avatar: room.avatar,
      room_id: room.id,
      conversation_id: latest.conversation_id,
      session_key: latest.session_key,
      agent_id: room.dm_target_agent_id,
      last_activity_at,
      message_count: latest.message_count ?? 0,
      running_task_count,
      can_delete: true,
    });
  }

  return items.sort((left, right) => {
    if (left.last_activity_at !== right.last_activity_at) {
      return right.last_activity_at - left.last_activity_at;
    }
    return left.title.localeCompare(right.title, "zh-CN");
  });
}

function get_sidebar_item_unread_state({
  chat_unread_counts,
  chat_unread_targets,
  chat_unread_timestamps,
  notification_key,
  room_id,
  session_key,
}: {
  chat_unread_counts: Record<string, number>;
  chat_unread_targets: Record<string, ChatNotificationTargetState>;
  chat_unread_timestamps: Record<string, number>;
  notification_key?: string | null;
  room_id?: string | null;
  session_key?: string | null;
}): {
  unread_conversation_id: string | null;
  unread_count: number;
  unread_target_key: string | null;
} {
  const normalized_room_id = room_id?.trim();
  let unread_count = 0;
  let unread_target: ChatNotificationTargetState | null = null;
  let unread_target_timestamp = -1;
  const counted_keys = new Set<string>();

  if (normalized_room_id) {
    for (const [key, target] of Object.entries(chat_unread_targets)) {
      if (target.room_id !== normalized_room_id) {
        continue;
      }
      const count = chat_unread_counts[key] ?? 0;
      if (count <= 0) {
        continue;
      }
      counted_keys.add(key);
      unread_count += count;
      const timestamp = chat_unread_timestamps[key] ?? 0;
      if (timestamp >= unread_target_timestamp) {
        unread_target = target;
        unread_target_timestamp = timestamp;
      }
    }

    const room_key = `room:${normalized_room_id}`;
    const room_conversation_key_prefix = `${room_key}:conversation:`;
    for (const [key, count] of Object.entries(chat_unread_counts)) {
      if (counted_keys.has(key) || count <= 0) {
        continue;
      }
      if (key !== room_key && !key.startsWith(room_conversation_key_prefix)) {
        continue;
      }
      unread_count += count;
      const timestamp = chat_unread_timestamps[key] ?? 0;
      if (timestamp >= unread_target_timestamp) {
        unread_target = chat_unread_targets[key] ?? {
          conversation_id: key.startsWith(room_conversation_key_prefix)
            ? key.slice(room_conversation_key_prefix.length)
            : null,
          key,
          room_id: normalized_room_id,
        };
        unread_target_timestamp = timestamp;
      }
    }
  } else if (notification_key) {
    unread_count = chat_unread_counts[notification_key] ?? 0;
    if (unread_count > 0) {
      unread_target = chat_unread_targets[notification_key] ?? {
        key: notification_key,
        room_id,
      };
    }
  }

  const session_notification_key = build_chat_notification_target_key({ session_key });
  if (session_notification_key && !counted_keys.has(session_notification_key)) {
    const session_unread_count = chat_unread_counts[session_notification_key] ?? 0;
    if (session_unread_count > 0) {
      unread_count += session_unread_count;
      const timestamp = chat_unread_timestamps[session_notification_key] ?? 0;
      if (timestamp >= unread_target_timestamp) {
        unread_target = chat_unread_targets[session_notification_key] ?? {
          conversation_id: null,
          key: session_notification_key,
          room_id,
          session_key,
        };
        unread_target_timestamp = timestamp;
      }
    }
  }

  return {
    unread_conversation_id: unread_target?.conversation_id ?? null,
    unread_count,
    unread_target_key: unread_target?.key ?? null,
  };
}

function ConversationRow({
  item,
  is_active,
  on_click,
  on_delete,
}: {
  item: SidebarConversationItem;
  is_active: boolean;
  on_click: () => void;
  on_delete?: () => void;
}) {
  const { t } = useI18n();
  const is_working = item.running_task_count > 0;

  return (
    <div
      className={cn(
        SIDEBAR_ROW_CLASS_NAME,
        is_active
          ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-elevated-background))] text-(--text-strong) shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_12%,transparent)]"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      )}
      onClick={on_click}
      role="button"
      tabIndex={0}
    >
      {is_active ? (
        <span className="absolute left-0 top-1/2 h-9 w-[3px] -translate-y-1/2 rounded-full bg-(--primary)" />
      ) : null}

      {item.kind === "room" ? (
        <CompositeRoomAvatar avatar={item.avatar} members={item.members} title={item.title} />
      ) : (
        <DirectAvatar
          agent={item.members[0] ?? { id: item.id, name: item.title, avatar: item.avatar ?? undefined }}
          is_working={is_working}
        />
      )}

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-[14px] font-semibold">{item.title}</span>
          {item.time_label || on_delete ? (
            <span className="relative flex h-7 w-10 shrink-0 items-center justify-end">
              {item.time_label ? (
                <span
                  className={cn(
                    "text-[11px] tabular-nums text-(--text-soft) transition-opacity duration-(--motion-duration-fast)",
                    on_delete && "group-hover/item:opacity-0",
                  )}
                >
                  {item.time_label}
                </span>
              ) : null}
              {on_delete ? (
                <button
                  className="absolute right-0 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-[10px] border border-transparent text-(--icon-muted) opacity-0 transition-[background,color,border-color,opacity] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive) group-hover/item:opacity-100"
                  onClick={(event) => {
                    event.stopPropagation();
                    on_delete();
                  }}
                  title={t("common.delete")}
                  type="button"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              ) : null}
            </span>
          ) : null}
        </div>
        <div className="mt-1 flex min-w-0 items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-muted)">
            {item.summary}
          </span>
          {is_working ? (
            <span className="shrink-0 rounded-full bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] px-2 py-0.5 text-[11px] font-medium text-(--primary)">
              {t("status.working")}
            </span>
          ) : null}
          {item.unread_count ? (
            <span className="flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-(--destructive) px-1.5 text-[11px] font-semibold text-white">
              {item.unread_count > 99 ? "99+" : item.unread_count}
            </span>
          ) : null}
        </div>
      </div>

    </div>
  );
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
      const notification_key = build_chat_notification_target_key({
        conversation_id: item.conversation_id,
        room_id: item.room_id,
        session_key: item.session_key,
      });
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

  const navigate_to_room = useCallback((item: SidebarConversationItem) => {
    if (!item.room_id) {
      return;
    }
    const target_conversation_id = item.unread_conversation_id || item.conversation_id;
    clear_chat_notifications_for_room(item.room_id);
    clear_chat_notifications_for_target(item.unread_target_key || item.notification_key);
    set_active_item(item.room_id);
    if (target_conversation_id) {
      navigate(AppRouteBuilders.room_conversation(item.room_id, target_conversation_id));
      return;
    }
    navigate(AppRouteBuilders.room(item.room_id));
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
                  navigate_to_room(item);
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
        on_confirm={(ids, name, avatar, skill_names, host_agent_id, host_auto_reply_enabled) =>
          void handle_confirm_create_room(ids, name, avatar, skill_names, host_agent_id, host_auto_reply_enabled)}
      />
    </div>
  );
});

function ContactRow({
  agent,
  is_active,
  is_working,
  on_chat,
  on_open_directory,
  running_task_count,
}: {
  agent: LauncherAgentSummary;
  is_active: boolean;
  is_working: boolean;
  on_chat: () => void;
  on_open_directory: () => void;
  running_task_count: number;
}) {
  const { t } = useI18n();
  const description = agent.description?.trim();
  const subtitle = is_working
    ? t("sidebar.running_tasks_short", { count: running_task_count })
    : (description || t("sidebar.contact_no_description"));

  return (
    <div
      className={cn(
        SIDEBAR_ROW_CLASS_NAME,
        is_active
          ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-elevated-background))] text-(--text-strong) shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_12%,transparent)]"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      )}
      onClick={on_open_directory}
      role="button"
      tabIndex={0}
    >
      {is_active ? (
        <span className="absolute left-0 top-1/2 h-9 w-[3px] -translate-y-1/2 rounded-full bg-(--primary)" />
      ) : null}
      <DirectAvatar
        agent={{ id: agent.id, name: agent.name, avatar: agent.avatar }}
        is_working={is_working}
      />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-[14px] font-semibold">{agent.name}</span>
          {is_working ? (
            <span className="shrink-0 rounded-full bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] px-2 py-0.5 text-[11px] font-medium text-(--primary)">
              {t("status.working")}
            </span>
          ) : null}
        </div>
        <p className="mt-1 truncate text-[12px] leading-5 text-(--text-muted)">
          {subtitle}
        </p>
      </div>
      <button
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-(--icon-muted) opacity-0 transition-[background,color,opacity,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default) group-hover/item:opacity-100"
        onClick={(event) => {
          event.stopPropagation();
          on_chat();
        }}
        title={t("sidebar.start_chat")}
        type="button"
      >
        <MessageCircle className="h-4 w-4" />
      </button>
    </div>
  );
}

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

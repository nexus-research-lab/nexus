/**
 * Home 面板内容
 *
 * 工作台侧边栏面板，包含 4 个分区：
 * - Starred（置顶项，localStorage 管理）
 * - Rooms（所有非 DM 类型的 Room）
 * - Direct Messages（除主智能体外的 DM）
 * - Agents（普通成员列表）
 *
 * 数据源复用现有 API：listRooms() + useAgentStore。
 */

import {
  Hash,
  Plus,
  Star,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getAgentWsUrl, isMainAgent } from "@/config/options";
import { get_dm_display_name } from "@/lib/dm-utils";
import { getIconAvatarSrc, getRoomAvatarIconId } from "@/lib/utils";
import { useWebSocket } from "@/lib/websocket";
import { CreateRoomDialog } from "@/features/room-members/create-room-dialog";
import { createRoom, deleteRoom, listRooms, subscribe_room_list_updates } from "@/lib/room-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { CollapsibleSection, SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import type { Agent, AgentRuntimeStatus } from "@/types/agent";
import type { EventMessage } from "@/types/message";
import { RoomAggregate } from "@/types/room";

// ==================== 置顶项 localStorage 管理 ====================

const STARRED_STORAGE_KEY = "nexus-sidebar-starred";

interface StarredItem {
  id: string;
  type: "room" | "dm" | "agent";
  name: string;
}

/** 从 localStorage 读取置顶项 */
function load_starred_items(): StarredItem[] {
  try {
    const raw = localStorage.getItem(STARRED_STORAGE_KEY);
    return raw ? (JSON.parse(raw) as StarredItem[]) : [];
  } catch {
    return [];
  }
}

// ==================== 辅助函数 ====================

/** 获取 Room 的时间戳用于排序 */
function get_room_timestamp(room: RoomAggregate): number {
  return new Date(
    room.room.updated_at ?? room.room.created_at ?? 0,
  ).getTime();
}

function render_agent_avatar_icon(agent_name: string, avatar?: string | null) {
  const avatar_src = getIconAvatarSrc(avatar);
  if (avatar_src) {
    return (
      <img
        alt={agent_name}
        className="h-4 w-4 rounded-full object-cover"
        src={avatar_src}
      />
    );
  }

  return (
    <span className="flex h-4 w-4 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
      {agent_name.trim().slice(0, 1).toUpperCase()}
    </span>
  );
}

function resolve_dm_agent(room: RoomAggregate, agents: Agent[]) {
  const agent_member = room.members.find((member) => member.member_type === "agent");
  if (!agent_member?.member_agent_id) {
    return null;
  }

  return agents.find((agent) => agent.agent_id === agent_member.member_agent_id) ?? null;
}

function resolve_dm_agent_id(room: RoomAggregate): string | null {
  const agent_member = room.members.find((member) => member.member_type === "agent");
  return agent_member?.member_agent_id ?? null;
}

function is_main_agent_dm_room(room: RoomAggregate): boolean {
  if (room.room.room_type !== "dm") {
    return false;
  }
  const agent_id = resolve_dm_agent_id(room);
  return Boolean(agent_id && isMainAgent(agent_id));
}

// ==================== 主组件 ====================

export const HomePanelContent = memo(function HomePanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const ws_url = getAgentWsUrl();
  const agents = useAgentStore((s) => s.agents);
  const agent_runtime_statuses = useAgentStore((s) => s.agent_runtime_statuses);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const load_agent_runtime_statuses = useAgentStore((s) => s.load_agent_runtime_statuses);
  const apply_agent_runtime_status = useAgentStore((s) => s.apply_agent_runtime_status);
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);
  const set_nexus_room_id = useSidebarStore((s) => s.set_nexus_room_id);

  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [starred] = useState<StarredItem[]>(load_starred_items);

  // 对话框状态
  const [delete_target, set_delete_target] = useState<{
    id: string;
    name: string;
    room_type: "room" | "dm";
  } | null>(null);
  const [is_create_room_open, set_is_create_room_open] = useState(false);
  const [is_creating_room, set_is_creating_room] = useState(false);
  const untitled_room_label = t("home.untitled_room");
  const untitled_dm_label = t("home.untitled_dm");

  /** 刷新 Room 列表 */
  const refresh_rooms = useCallback(() => {
    void listRooms(200).then(set_rooms);
  }, []);

  // 初始化加载数据
  useEffect(() => {
    void load_agents();
    refresh_rooms();
  }, [load_agents, refresh_rooms]);

  useEffect(() => subscribe_room_list_updates(refresh_rooms), [refresh_rooms]);

  const agent_ids = useMemo(() => agents.map((agent) => agent.agent_id), [agents]);
  const has_agents = agent_ids.length > 0;
  const agent_id_set = useMemo(() => new Set(agent_ids), [agent_ids]);
  const regular_agents = useMemo(
    () => agents.filter((agent) => !isMainAgent(agent.agent_id)),
    [agents],
  );

  useEffect(() => {
    if (!has_agents) {
      return;
    }
    void load_agent_runtime_statuses();
  }, [has_agents, load_agent_runtime_statuses, agent_ids]);

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
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_runtime_message,
  });

  useEffect(() => {
    if (runtime_ws_state !== "connected" || !has_agents) {
      return;
    }

    for (const agent_id of agent_ids) {
      runtime_ws_send({
        type: "subscribe_workspace",
        agent_id,
      });
    }
    void load_agent_runtime_statuses();

    return () => {
      for (const agent_id of agent_ids) {
        runtime_ws_send({
          type: "unsubscribe_workspace",
          agent_id,
        });
      }
    };
  }, [agent_ids, has_agents, load_agent_runtime_statuses, runtime_ws_send, runtime_ws_state]);

  // 分离 Room 和 DM
  const { normal_rooms, nexus_dm_room, regular_dm_rooms } = useMemo(() => {
    const sorted = [...rooms].sort(
      (a, b) => get_room_timestamp(b) - get_room_timestamp(a),
    );
    const dm_rooms = sorted.filter((r) => r.room.room_type === "dm");
    return {
      normal_rooms: sorted.filter((r) => r.room.room_type !== "dm"),
      nexus_dm_room: dm_rooms.find((room) => is_main_agent_dm_room(room)) ?? null,
      regular_dm_rooms: dm_rooms.filter((room) => !is_main_agent_dm_room(room)),
    };
  }, [rooms]);

  useEffect(() => {
    // 中文注释：Nexus 已提升为 header 一级入口，这里只同步其真实 DM room_id，供 header 激活态复用。
    set_nexus_room_id(nexus_dm_room?.room.id ?? null);
  }, [nexus_dm_room, set_nexus_room_id]);

  // 导航到 Room
  const navigate_to_room = useCallback(
    (room_id: string) => {
      set_active_item(room_id);
      navigate(AppRouteBuilders.room(room_id));
    },
    [navigate, set_active_item],
  );

  // 导航到 Agent（联系人详情）
  const navigate_to_agent = useCallback(
    (agent_id: string) => {
      set_active_item(agent_id);
      navigate(AppRouteBuilders.contact_profile(agent_id));
    },
    [navigate, set_active_item],
  );

  // 新建 Room — 弹出对话框
  const handle_create_room = useCallback(() => {
    set_is_create_room_open(true);
  }, []);

  // 确认创建 Room
  const handle_confirm_create_room = useCallback(async (
    agent_ids: string[],
    name: string,
    avatar?: string,
  ) => {
    set_is_creating_room(true);
    try {
      const context = await createRoom({ agent_ids, name, avatar });
      set_is_create_room_open(false);
      refresh_rooms();
      // 创建后直接导航到新 Room
      navigate(AppRouteBuilders.room(context.room.id));
    } finally {
      set_is_creating_room(false);
    }
  }, [navigate, refresh_rooms]);

  // 删除 Room
  const handle_delete_room = useCallback(async () => {
    if (!delete_target) return;
    const deleted_room_id = delete_target.id;
    await deleteRoom(deleted_room_id);
    set_delete_target(null);
    // 中文注释：删除当前激活房间时不立即跳转，留给路由层做失效判断。
    if (active_item_id === deleted_room_id) {
      set_active_item(null);
    }
    refresh_rooms();
  }, [active_item_id, delete_target, refresh_rooms, set_active_item]);

  return (
    <div className="flex flex-col">
      {/* Starred 分区 */}
      {starred.length > 0 ? (
        <CollapsibleSection
          count={starred.length}
          section_id="home-starred"
          title={t("home.starred")}
        >
          {starred.map((item) => (
            <SidebarListItem
              key={item.id}
              icon={<Star className="h-4 w-4 text-amber-400" />}
              is_active={active_item_id === item.id}
              label={item.name}
              on_click={() => {
                set_active_item(item.id);
                if (item.type === "agent") {
                  navigate(AppRouteBuilders.contact_profile(item.id));
                } else {
                  navigate(AppRouteBuilders.room(item.id));
                }
              }}
            />
          ))}
        </CollapsibleSection>
      ) : null}

      {/* Rooms 分区 — 带新建按钮 */}
      <CollapsibleSection
        action_icon={<Plus className="h-4 w-4" />}
        action_title={t("home.create_room")}
        count={normal_rooms.length}
        on_action={handle_create_room}
        section_id="home-rooms"
        title={t("home.rooms")}
      >
        {normal_rooms.length > 0 ? (
          normal_rooms.map((room) => (
            <SidebarListItem
              key={room.room.id}
              icon={(() => {
                const room_avatar_id = getRoomAvatarIconId(
                  room.room.id,
                  room.room.name,
                  room.room.avatar,
                );
                const room_avatar_src = getIconAvatarSrc(room_avatar_id);

                return room_avatar_src ? (
                  <img
                    alt={room.room.name?.trim() || untitled_room_label}
                    className="h-4 w-4 rounded-[4px] object-contain"
                    src={room_avatar_src}
                  />
                ) : (
                  <Hash className="h-4 w-4" />
                );
              })()}
              is_active={active_item_id === room.room.id}
              label={room.room.name?.trim() || untitled_room_label}
              on_click={() => navigate_to_room(room.room.id)}
              on_delete={() => set_delete_target({
                id: room.room.id,
                name: room.room.name?.trim() || untitled_room_label,
                room_type: "room",
              })}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[13px] text-(--text-soft)">{t("home.no_rooms")}</p>
        )}
      </CollapsibleSection>

      {/* Direct Messages 分区 */}
      <CollapsibleSection
        count={regular_dm_rooms.length}
        section_id="home-dms"
        title={t("home.direct_messages")}
      >
        {regular_dm_rooms.length > 0 ? (
          regular_dm_rooms.map((room) => (
            <SidebarListItem
              key={room.room.id}
              icon={(() => {
                const dm_agent = resolve_dm_agent(room, agents);
                if (dm_agent) {
                  return render_agent_avatar_icon(dm_agent.name, dm_agent.avatar);
                }

                return render_agent_avatar_icon(
                  get_dm_display_name(room, agents, untitled_dm_label),
                  null,
                );
              })()}
              is_active={active_item_id === room.room.id}
              label={get_dm_display_name(room, agents, untitled_dm_label)}
              on_click={() => navigate_to_room(room.room.id)}
              on_delete={() => set_delete_target({
                id: room.room.id,
                name: get_dm_display_name(room, agents, untitled_dm_label),
                room_type: "dm",
              })}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[13px] text-(--text-soft)">{t("home.no_dms")}</p>
        )}
      </CollapsibleSection>

      {/* Agents 分区 */}
      <CollapsibleSection
        count={regular_agents.length}
        section_id="home-agents"
        title={t("home.agents")}
      >
        {regular_agents.length > 0 ? (
          regular_agents.map((agent) => (
            <SidebarListItem
              key={agent.agent_id}
              icon={render_agent_avatar_icon(agent.name, agent.avatar)}
              is_active={active_item_id === agent.agent_id}
              label={agent.name}
              meta={(() => {
                const runtime = agent_runtime_statuses[agent.agent_id];
                const running_task_count = runtime?.running_task_count
                  ?? (agent.status === "running" ? 1 : 0);
                return running_task_count > 0
                  ? `${running_task_count} 任务`
                  : t("status.idle");
              })()}
              on_click={() => navigate_to_agent(agent.agent_id)}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[13px] text-(--text-soft)">{t("home.no_agents")}</p>
        )}
      </CollapsibleSection>

      {/* 删除确认对话框 */}
      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={delete_target !== null}
        message={t("home.delete_message", { name: delete_target?.name ?? "" })}
        on_cancel={() => set_delete_target(null)}
        on_confirm={() => void handle_delete_room()}
        title={t("home.delete_confirm")}
        variant="danger"
      />

      {/* 创建 Room 对话框 */}
      <CreateRoomDialog
        agents={regular_agents}
        is_creating={is_creating_room}
        is_open={is_create_room_open}
        on_cancel={() => set_is_create_room_open(false)}
        on_confirm={(ids, name, avatar) => void handle_confirm_create_room(ids, name, avatar)}
      />
    </div>
  );
});

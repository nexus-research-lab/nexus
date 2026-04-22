/**
 * Home 面板内容
 *
 * 工作台侧边栏面板，包含 2 个分区：
 * - Rooms（所有非 DM 类型的 Room）
 * - Agents（成员目录，标题进入成员管理，子项进入对应私信）
 *
 * 数据源使用轻量 bootstrap 摘要，避免为了侧边栏列表拉全量 Agent/Room 数据。
 */

import {
  Hash,
  MessageSquarePlus,
  Plus,
  UserPlus,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_agent_ws_url, is_main_agent } from "@/config/options";
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { get_icon_avatar_src, get_room_avatar_icon_id } from "@/lib/utils";
import { useWebSocket } from "@/lib/websocket";
import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { create_room, delete_room, subscribe_room_directory_updates } from "@/lib/api/room-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { CollapsibleSection, SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { AGENT_LIST_UPDATED_EVENT_NAME, useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import type { AgentRuntimeStatus } from "@/types/agent/agent";
import type { LauncherAgentSummary, LauncherRoomSummary } from "@/types/app/launcher";
import type { EventMessage } from "@/types/conversation/message";

// ==================== 辅助函数 ====================

/** 获取 Room 的时间戳用于排序 */
function get_room_timestamp(room: LauncherRoomSummary): number {
  return new Date(
    room.updated_at ?? room.created_at ?? 0,
  ).getTime();
}

function render_agent_avatar_icon(agent_name: string, avatar?: string | null) {
  const avatar_src = get_icon_avatar_src(avatar);
  if (avatar_src) {
    return (
      <img
        alt={agent_name}
        className="h-5 w-5 rounded-full object-cover"
        src={avatar_src}
      />
    );
  }

  return (
    <span className="flex h-5 w-5 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[9px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
      {agent_name.trim().slice(0, 1).toUpperCase()}
    </span>
  );
}

function is_main_agent_dm_room(room: LauncherRoomSummary): boolean {
  if (room.room_type !== "dm") {
    return false;
  }
  return Boolean(room.dm_target_agent_id && is_main_agent(room.dm_target_agent_id));
}

// ==================== 主组件 ====================

export const HomePanelContent = memo(function HomePanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const ws_url = get_agent_ws_url();
  const agent_runtime_statuses = useAgentStore((s) => s.agent_runtime_statuses);
  const apply_agent_runtime_status = useAgentStore((s) => s.apply_agent_runtime_status);
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);
  const set_nexus_room_id = useSidebarStore((s) => s.set_nexus_room_id);
  const [agents, set_agents] = useState<LauncherAgentSummary[]>([]);
  const [rooms, set_rooms] = useState<LauncherRoomSummary[]>([]);

  // 对话框状态
  const [delete_target, set_delete_target] = useState<{
    id: string;
    name: string;
    room_type: "room" | "dm";
  } | null>(null);
  const [is_create_room_open, set_is_create_room_open] = useState(false);
  const [is_creating_room, set_is_creating_room] = useState(false);
  const untitled_room_label = t("home.untitled_room");

  const refresh_directory = useCallback(async () => {
    try {
      const payload = await get_launcher_bootstrap_api();
      set_agents(payload.agents);
      set_rooms(payload.rooms);
    } catch (error) {
      console.error("[HomePanelContent] 加载侧边栏目录失败:", error);
      set_agents([]);
      set_rooms([]);
    }
  }, []);

  // 初始化加载数据
  useEffect(() => {
    void refresh_directory();
  }, [refresh_directory]);

  useEffect(() => subscribe_room_directory_updates(() => {
    void refresh_directory();
  }), [refresh_directory]);

  useEffect(() => {
    const handle_agent_list_updated = () => {
      void refresh_directory();
    };
    window.addEventListener(AGENT_LIST_UPDATED_EVENT_NAME, handle_agent_list_updated);
    return () => {
      window.removeEventListener(AGENT_LIST_UPDATED_EVENT_NAME, handle_agent_list_updated);
    };
  }, [refresh_directory]);

  const agent_ids = useMemo(() => agents.map((agent) => agent.id), [agents]);
  const has_agents = agent_ids.length > 0;
  const agent_id_set = useMemo(() => new Set(agent_ids), [agent_ids]);
  const regular_agents = agents;
  const create_room_agents = useMemo(
    () => regular_agents.map((agent) => ({
      agent_id: agent.id,
      name: agent.name,
      avatar: agent.avatar,
    })),
    [regular_agents],
  );
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

    return () => {
      for (const agent_id of agent_ids) {
        runtime_ws_send({
          type: "unsubscribe_workspace",
          agent_id,
        });
      }
    };
  }, [agent_ids, has_agents, runtime_ws_send, runtime_ws_state]);

  // 分离 Room 和 DM
  const { normal_rooms, nexus_dm_room, regular_dm_rooms } = useMemo(() => {
    const sorted = [...rooms].sort(
      (a, b) => get_room_timestamp(b) - get_room_timestamp(a),
    );
    const dm_rooms = sorted.filter((room) => room.room_type === "dm");
    return {
      normal_rooms: sorted.filter((room) => room.room_type !== "dm"),
      nexus_dm_room: dm_rooms.find((room) => is_main_agent_dm_room(room)) ?? null,
      regular_dm_rooms: dm_rooms.filter((room) => !is_main_agent_dm_room(room)),
    };
  }, [rooms]);
  const regular_dm_rooms_by_agent_id = useMemo(
    () =>
      new Map(
        regular_dm_rooms
          .filter((room): room is LauncherRoomSummary & { dm_target_agent_id: string } =>
            Boolean(room.dm_target_agent_id),
          )
          .map((room) => [room.dm_target_agent_id, room]),
      ),
    [regular_dm_rooms],
  );

  useEffect(() => {
    // Nexus 已提升为 header 一级入口，这里只同步其真实 DM room_id，供 header 激活态复用。
    set_nexus_room_id(nexus_dm_room?.id ?? null);
  }, [nexus_dm_room, set_nexus_room_id]);

  // 导航到 Room
  const navigate_to_room = useCallback(
    (room_id: string) => {
      set_active_item(room_id);
      navigate(AppRouteBuilders.room(room_id));
    },
    [navigate, set_active_item],
  );

  // 导航到 Agent 对应私信
  const navigate_to_agent_dm = useCallback(
    async (agent_id: string) => {
      const target = await resolve_direct_room_navigation_target(agent_id);
      set_active_item(target.context.room.id);
      navigate(target.route);
    },
    [navigate, set_active_item],
  );

  // 导航到 Agent 管理目录
  const navigate_to_agents_directory = useCallback(() => {
    set_active_item(null);
    navigate(AppRouteBuilders.contacts());
  }, [navigate, set_active_item]);

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
      const context = await create_room({ agent_ids, name, avatar });
      set_is_create_room_open(false);
      void refresh_directory();
      // 创建后直接导航到新 Room
      navigate(AppRouteBuilders.room(context.room.id));
    } finally {
      set_is_creating_room(false);
    }
  }, [navigate, refresh_directory]);

  // 删除 Room
  const handle_delete_room = useCallback(async () => {
    if (!delete_target) return;
    const deleted_room_id = delete_target.id;
    await delete_room(deleted_room_id);
    set_delete_target(null);
    // 删除当前激活房间时不立即跳转，留给路由层做失效判断。
    if (active_item_id === deleted_room_id) {
      set_active_item(null);
    }
    void refresh_directory();
  }, [active_item_id, delete_target, refresh_directory, set_active_item]);

  const rooms_empty_description = has_agents
    ? t("home.rooms_empty_description")
    : t("home.rooms_empty_no_agents_description");
  const rooms_empty_action = has_agents
    ? t("home.rooms_empty_action")
    : t("home.rooms_empty_no_agents_action");
  const handle_rooms_empty_action = has_agents
    ? handle_create_room
    : navigate_to_agents_directory;

  return (
    <div className="flex flex-col">
      {/* Rooms 分区 — 带新建按钮 */}
      <div data-tour-anchor={SIDEBAR_TOUR_ANCHORS.rooms}>
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
                key={room.id}
                icon={(() => {
                  const room_avatar_id = get_room_avatar_icon_id(
                    room.id,
                    room.name,
                    room.avatar,
                  );
                  const room_avatar_src = get_icon_avatar_src(room_avatar_id, "room");

                  return room_avatar_src ? (
                    <img
                      alt={room.name?.trim() || untitled_room_label}
                      className="h-5 w-5 rounded-[5px] object-cover"
                      src={room_avatar_src}
                    />
                  ) : (
                    <Hash className="h-4 w-4" />
                  );
                })()}
                is_active={active_item_id === room.id}
                label={room.name?.trim() || untitled_room_label}
                on_click={() => navigate_to_room(room.id)}
                on_delete={() => set_delete_target({
                  id: room.id,
                  name: room.name?.trim() || untitled_room_label,
                  room_type: "room",
                })}
              />
            ))
          ) : (
            <SidebarEmptyGuide
              action_label={rooms_empty_action}
              description={rooms_empty_description}
              icon={MessageSquarePlus}
              on_action={handle_rooms_empty_action}
              title={t("home.rooms_empty_title")}
            />
          )}
        </CollapsibleSection>
      </div>

      {/* Agents 分区 */}
      <div data-tour-anchor={SIDEBAR_TOUR_ANCHORS.agents}>
        <CollapsibleSection
          count={regular_agents.length}
          is_title_active={location.pathname === AppRouteBuilders.contacts()}
          on_title_click={navigate_to_agents_directory}
          section_id="home-agents"
          title={t("home.agents")}
        >
          {regular_agents.length > 0 ? (
            regular_agents.map((agent) => (
              <SidebarListItem
                key={agent.id}
                icon={render_agent_avatar_icon(agent.name, agent.avatar)}
                is_active={active_item_id === regular_dm_rooms_by_agent_id.get(agent.id)?.id}
                label={agent.name}
                meta={(() => {
                  const running_task_count = agent_runtime_statuses[agent.id]?.running_task_count ?? 0;
                  return running_task_count > 0
                    ? `${running_task_count} 任务`
                    : t("status.idle");
                })()}
                on_click={() => void navigate_to_agent_dm(agent.id)}
              />
            ))
          ) : (
            <SidebarEmptyGuide
              action_label={t("home.agents_empty_action")}
              description={t("home.agents_empty_description")}
              icon={UserPlus}
              on_action={navigate_to_agents_directory}
              title={t("home.agents_empty_title")}
            />
          )}
        </CollapsibleSection>
      </div>

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
        agents={create_room_agents}
        is_creating={is_creating_room}
        is_open={is_create_room_open}
        on_cancel={() => set_is_create_room_open(false)}
        on_confirm={(ids, name, avatar) => void handle_confirm_create_room(ids, name, avatar)}
      />
    </div>
  );
});

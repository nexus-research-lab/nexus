/**
 * Home 面板内容
 *
 * 工作台侧边栏面板，包含 4 个可折叠分区：
 * - Starred（置顶项，localStorage 管理）
 * - Rooms（所有非 DM 类型的 Room）
 * - Direct Messages（所有 DM 类型的 Room）
 * - Agents（所有 Agent 列表）
 *
 * 数据源复用现有 API：listRooms() + useAgentStore。
 */

import {
  Bot,
  Hash,
  MessageCircleMore,
  Plus,
  Star,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_dm_display_name } from "@/lib/dm-utils";
import { CreateRoomDialog } from "@/features/room-members/create-room-dialog";
import { createRoom, deleteRoom, listRooms, updateRoom } from "@/lib/room-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import { CollapsibleSection, SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
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

// ==================== 主组件 ====================

export const HomePanelContent = memo(function HomePanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);

  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [starred] = useState<StarredItem[]>(load_starred_items);

  // 对话框状态
  const [delete_target, set_delete_target] = useState<{ id: string; name: string } | null>(null);
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

  // 分离 Room 和 DM
  const { normal_rooms, dm_rooms } = useMemo(() => {
    const sorted = [...rooms].sort(
      (a, b) => get_room_timestamp(b) - get_room_timestamp(a),
    );
    return {
      normal_rooms: sorted.filter((r) => r.room.room_type !== "dm"),
      dm_rooms: sorted.filter((r) => r.room.room_type === "dm"),
    };
  }, [rooms]);

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
  const handle_confirm_create_room = useCallback(async (agent_ids: string[], name: string) => {
    set_is_creating_room(true);
    try {
      const context = await createRoom({ agent_ids, name });
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
    await deleteRoom(delete_target.id);
    set_delete_target(null);
    refresh_rooms();
  }, [delete_target, refresh_rooms]);

  return (
    <div className="flex flex-col gap-1">
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
        action_icon={<Plus className="h-3.5 w-3.5" />}
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
              icon={<Hash className="h-4 w-4" />}
              is_active={active_item_id === room.room.id}
              label={room.room.name?.trim() || untitled_room_label}
              on_click={() => navigate_to_room(room.room.id)}
              on_delete={() => set_delete_target({ id: room.room.id, name: room.room.name?.trim() || untitled_room_label })}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[13px] text-slate-400">{t("home.no_rooms")}</p>
        )}
      </CollapsibleSection>

      {/* Direct Messages 分区 */}
      <CollapsibleSection
        count={dm_rooms.length}
        section_id="home-dms"
        title={t("home.direct_messages")}
      >
        {dm_rooms.length > 0 ? (
          dm_rooms.map((room) => (
            <SidebarListItem
              key={room.room.id}
              icon={<MessageCircleMore className="h-4 w-4" />}
              is_active={active_item_id === room.room.id}
              label={get_dm_display_name(room, agents, untitled_dm_label)}
              on_click={() => navigate_to_room(room.room.id)}
              on_delete={() => set_delete_target({ id: room.room.id, name: get_dm_display_name(room, agents, untitled_dm_label) })}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[13px] text-slate-400">{t("home.no_dms")}</p>
        )}
      </CollapsibleSection>

      {/* Agents 分区 */}
      <CollapsibleSection
        count={agents.length}
        section_id="home-agents"
        title={t("home.agents")}
      >
        {agents.length > 0 ? (
          agents.map((agent) => (
            <SidebarListItem
              key={agent.agent_id}
              icon={<Bot className="h-4 w-4" />}
              is_active={active_item_id === agent.agent_id}
              label={agent.name}
              meta={agent.status === "running" ? "●" : t("status.idle")}
              on_click={() => navigate_to_agent(agent.agent_id)}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[13px] text-[color:var(--text-soft)]">{t("home.no_agents")}</p>
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
        agents={agents}
        is_creating={is_creating_room}
        is_open={is_create_room_open}
        on_cancel={() => set_is_create_room_open(false)}
        on_confirm={(ids, name) => void handle_confirm_create_room(ids, name)}
      />
    </div>
  );
});

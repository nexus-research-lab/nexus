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
  ChevronDown,
  ChevronRight,
  Hash,
  MessageCircleMore,
  Star,
  Waypoints,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { listRooms } from "@/lib/room-api";
import { cn, formatRelativeTime } from "@/lib/utils";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import { Agent } from "@/types/agent";
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

// ==================== 可折叠 Section 组件 ====================

interface CollapsibleSectionProps {
  section_id: string;
  title: string;
  count: number;
  children: React.ReactNode;
}

/** 可折叠分区，折叠状态由 sidebar store 管理 */
function CollapsibleSection({
  section_id,
  title,
  count,
  children,
}: CollapsibleSectionProps) {
  const is_collapsed = useSidebarStore(
    (s) => s.collapsed_sections[section_id] ?? false,
  );
  const toggle = useSidebarStore((s) => s.toggle_section);

  return (
    <section className="border-b border-white/10 pb-1">
      <button
        className="flex w-full items-center gap-1.5 px-2 py-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-500 transition-colors hover:text-slate-700"
        onClick={() => toggle(section_id)}
        type="button"
      >
        {is_collapsed ? (
          <ChevronRight className="h-3 w-3" />
        ) : (
          <ChevronDown className="h-3 w-3" />
        )}
        <span className="flex-1 text-left">{title}</span>
        <span className="text-[10px] text-slate-400">{count}</span>
      </button>

      {!is_collapsed ? (
        <div className="flex flex-col gap-0.5 pb-1">{children}</div>
      ) : null}
    </section>
  );
}

// ==================== 列表条目组件 ====================

interface PanelItemProps {
  icon: React.ReactNode;
  label: string;
  meta?: string;
  is_active?: boolean;
  on_click: () => void;
}

function PanelItem({ icon, label, meta, is_active, on_click }: PanelItemProps) {
  return (
    <button
      className={cn(
        "flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-[12px] transition-all duration-150",
        is_active
          ? "bg-white/60 font-semibold text-slate-900 shadow-sm"
          : "text-slate-600 hover:bg-white/30 hover:text-slate-800",
      )}
      onClick={on_click}
      type="button"
    >
      <span className="flex h-5 w-5 shrink-0 items-center justify-center text-slate-500">
        {icon}
      </span>
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {meta ? (
        <span className="shrink-0 text-[10px] text-slate-400">{meta}</span>
      ) : null}
    </button>
  );
}

// ==================== 辅助函数 ====================

/** 获取 DM 的显示名称（优先使用 Agent 名称） */
function get_dm_display_name(room: RoomAggregate, agents: Agent[]): string {
  const agent_member = room.members.find((m) => m.member_type === "agent");
  if (agent_member?.member_agent_id) {
    const matched = agents.find(
      (a) => a.agent_id === agent_member.member_agent_id,
    );
    if (matched) return matched.name;
  }
  return room.room.name?.trim() || "未命名 DM";
}

/** 获取 Room 的时间戳用于排序 */
function get_room_timestamp(room: RoomAggregate): number {
  return new Date(
    room.room.updated_at ?? room.room.created_at ?? 0,
  ).getTime();
}

// ==================== 主组件 ====================

export const HomePanelContent = memo(function HomePanelContent() {
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);

  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [starred] = useState<StarredItem[]>(load_starred_items);

  // 初始化加载数据
  useEffect(() => {
    void load_agents();
    let cancelled = false;
    void listRooms(200).then((data) => {
      if (!cancelled) set_rooms(data);
    });
    return () => {
      cancelled = true;
    };
  }, [load_agents]);

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

  return (
    <div className="flex flex-col gap-1">
      {/* Starred 分区 */}
      {starred.length > 0 ? (
        <CollapsibleSection
          count={starred.length}
          section_id="home-starred"
          title="Starred"
        >
          {starred.map((item) => (
            <PanelItem
              key={item.id}
              icon={<Star className="h-3.5 w-3.5 text-amber-400" />}
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

      {/* Rooms 分区 */}
      <CollapsibleSection
        count={normal_rooms.length}
        section_id="home-rooms"
        title="Rooms"
      >
        {normal_rooms.length > 0 ? (
          normal_rooms.map((room) => (
            <PanelItem
              key={room.room.id}
              icon={<Hash className="h-3.5 w-3.5" />}
              is_active={active_item_id === room.room.id}
              label={room.room.name?.trim() || "未命名协作"}
              meta={formatRelativeTime(get_room_timestamp(room))}
              on_click={() => navigate_to_room(room.room.id)}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[11px] text-slate-400">暂无协作空间</p>
        )}
      </CollapsibleSection>

      {/* Direct Messages 分区 */}
      <CollapsibleSection
        count={dm_rooms.length}
        section_id="home-dms"
        title="Direct Messages"
      >
        {dm_rooms.length > 0 ? (
          dm_rooms.map((room) => (
            <PanelItem
              key={room.room.id}
              icon={<MessageCircleMore className="h-3.5 w-3.5" />}
              is_active={active_item_id === room.room.id}
              label={get_dm_display_name(room, agents)}
              meta={formatRelativeTime(get_room_timestamp(room))}
              on_click={() => navigate_to_room(room.room.id)}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[11px] text-slate-400">暂无私信</p>
        )}
      </CollapsibleSection>

      {/* Agents 分区 */}
      <CollapsibleSection
        count={agents.length}
        section_id="home-agents"
        title="Agents"
      >
        {agents.length > 0 ? (
          agents.map((agent) => (
            <PanelItem
              key={agent.agent_id}
              icon={<Bot className="h-3.5 w-3.5" />}
              is_active={active_item_id === agent.agent_id}
              label={agent.name}
              meta={agent.status === "running" ? "●" : "idle"}
              on_click={() => navigate_to_agent(agent.agent_id)}
            />
          ))
        ) : (
          <p className="px-2 py-2 text-[11px] text-slate-400">暂无成员</p>
        )}
      </CollapsibleSection>
    </div>
  );
});

/**
 * 工作台聚合页（/app）
 *
 * 主内容区包含：
 * - Page Header：标题"工作台" + 快捷操作按钮
 * - 待处理事项区：pending permissions + 未读消息
 * - 最近对话区：按 last_activity_at 降序排列的 Room + DM 列表
 * - 快捷入口区：新建 Room / 新建 DM / Nexus
 */

import {
  Clock3,
  Hash,
  MessageCircleMore,
  Plus,
  Search,
  Sparkles,
  Waypoints,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { listRooms } from "@/lib/room-api";
import { get_room_timestamp, sort_rooms_by_recency } from "@/lib/room-utils";
import { cn, formatRelativeTime } from "@/lib/utils";
import { WorkspacePageFrame } from "@/shared/ui/workspace-page-frame";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { useAgentStore } from "@/store/agent";
import { Agent } from "@/types/agent";
import { RoomAggregate } from "@/types/room";

/** 获取 DM 显示名称 */
function get_dm_name(room: RoomAggregate, agents: Agent[]): string {
  const agent_member = room.members.find((m) => m.member_type === "agent");
  if (agent_member?.member_agent_id) {
    const matched = agents.find(
      (a) => a.agent_id === agent_member.member_agent_id,
    );
    if (matched) return matched.name;
  }
  return room.room.name?.trim() || "未命名 DM";
}

// ==================== 最近对话条目 ====================

interface RecentItemProps {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  time: string;
  on_click: () => void;
}

function RecentItem({ icon, title, subtitle, time, on_click }: RecentItemProps) {
  return (
    <button
      className="flex w-full items-center gap-3 rounded-2xl px-4 py-3 text-left transition-all duration-200 hover:bg-white/50"
      onClick={on_click}
      type="button"
    >
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-white/20 bg-white/16 text-slate-500 backdrop-blur-sm">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-slate-800">{title}</p>
        <p className="mt-0.5 truncate text-xs text-slate-500">{subtitle}</p>
      </div>
      <div className="flex shrink-0 items-center gap-1 text-[11px] text-slate-400">
        <Clock3 className="h-3 w-3" />
        <span>{time}</span>
      </div>
    </button>
  );
}

// ==================== 快捷入口 ====================

interface QuickActionProps {
  icon: React.ReactNode;
  label: string;
  on_click: () => void;
}

function QuickAction({ icon, label, on_click }: QuickActionProps) {
  return (
    <button
      className={cn(
        "flex flex-col items-center gap-2 rounded-2xl px-6 py-4 transition-all duration-200",
        "workspace-chip hover:bg-white/28 hover:shadow-sm",
      )}
      onClick={on_click}
      type="button"
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-white/20 bg-white/16 text-slate-600 backdrop-blur-sm">
        {icon}
      </div>
      <span className="text-xs font-medium text-slate-600">{label}</span>
    </button>
  );
}

// ==================== 主组件 ====================

export function HomePage() {
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);

  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [loading, set_loading] = useState(true);

  // 初始化加载
  useEffect(() => {
    void load_agents();
    let cancelled = false;
    void listRooms(200)
      .then((data) => {
        if (!cancelled) set_rooms(data);
      })
      .finally(() => {
        if (!cancelled) set_loading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [load_agents]);

  // 最近对话列表（按时间降序，最多 20 条）
  const recent_rooms = useMemo(() => {
    return sort_rooms_by_recency(rooms).slice(0, 20);
  }, [rooms]);

  // 导航到 Room
  const open_room = useCallback(
    (room_id: string) => {
      navigate(AppRouteBuilders.room(room_id));
    },
    [navigate],
  );

  // Header trailing 操作区
  const header_trailing = (
    <>
      <WorkspacePillButton
        onClick={() => navigate(AppRouteBuilders.launcher())}
      >
        <Search className="h-3.5 w-3.5" />
        搜索
      </WorkspacePillButton>
      <WorkspacePillButton
        onClick={() => navigate(AppRouteBuilders.contacts())}
      >
        <Plus className="h-3.5 w-3.5" />
        新建
      </WorkspacePillButton>
    </>
  );

  return (
    <WorkspacePageFrame content_padding_class_name="p-0">
        {/* 页面头部 */}
        <WorkspaceSurfaceHeader
          badge="HOME"
          leading={<Waypoints className="h-4 w-4 text-slate-800/72" />}
          subtitle={
            <span className="truncate">
              {rooms.length} 个协作空间 · {agents.length} 位成员
            </span>
          }
          title="工作台"
          trailing={header_trailing}
        />

        {/* 主内容区 */}
        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
          {loading ? (
            <div className="flex min-h-[200px] items-center justify-center text-sm text-slate-400">
              加载中...
            </div>
          ) : (
            <div className="flex flex-col gap-6">
              {/* 快捷入口区 */}
              <section>
                <h3 className="mb-3 text-xs font-semibold uppercase tracking-[0.1em] text-slate-500">
                  快捷入口
                </h3>
                <div className="flex flex-wrap gap-3">
                  <QuickAction
                    icon={<Plus className="h-5 w-5" />}
                    label="新建 Room"
                    on_click={() => navigate(AppRouteBuilders.launcher_app())}
                  />
                  <QuickAction
                    icon={<MessageCircleMore className="h-5 w-5" />}
                    label="新建 DM"
                    on_click={() => navigate(AppRouteBuilders.contacts())}
                  />
                  <QuickAction
                    icon={<Sparkles className="h-5 w-5" />}
                    label="Nexus"
                    on_click={() => navigate(AppRouteBuilders.launcher_app())}
                  />
                </div>
              </section>

              {/* 最近对话区 */}
              <section>
                <h3 className="mb-3 text-xs font-semibold uppercase tracking-[0.1em] text-slate-500">
                  最近对话
                </h3>
                {recent_rooms.length > 0 ? (
                  <div className="workspace-card flex flex-col gap-0.5 rounded-2xl p-1">
                    {recent_rooms.map((room) => {
                      const is_dm = room.room.room_type === "dm";
                      const title = is_dm
                        ? get_dm_name(room, agents)
                        : room.room.name?.trim() || "未命名协作";
                      const member_count = room.members.filter(
                        (m) => m.member_type === "agent",
                      ).length;
                      const subtitle = is_dm
                        ? "1v1 协作"
                        : `${member_count} 位成员`;
                      const timestamp = get_room_timestamp(room);

                      return (
                        <RecentItem
                          key={room.room.id}
                          icon={
                            is_dm ? (
                              <MessageCircleMore className="h-4 w-4" />
                            ) : (
                              <Hash className="h-4 w-4" />
                            )
                          }
                          on_click={() => open_room(room.room.id)}
                          subtitle={subtitle}
                          time={
                            timestamp > 0
                              ? formatRelativeTime(timestamp)
                              : "刚刚"
                          }
                          title={title}
                        />
                      );
                    })}
                  </div>
                ) : (
                  <div className="workspace-card flex min-h-[120px] items-center justify-center rounded-2xl px-6 text-center">
                    <p className="text-sm text-slate-500">
                      还没有对话记录，从快捷入口开始吧
                    </p>
                  </div>
                )}
              </section>
            </div>
          )}
        </div>
    </WorkspacePageFrame>
  );
}

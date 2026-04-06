/**
 * DMs 面板内容
 *
 * WhatsApp 风格 DM 列表：搜索框 + DM 条目列表。
 * 每个条目显示 Agent 头像 + 名称 + 最后消息预览 + 时间。
 * 点击导航到对应 Room 的对话页。
 *
 * 数据源复用 listRooms() 过滤 room_type=dm + useAgentStore。
 */

import { MessageCircleMore, Search, Trash2 } from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_dm_display_name } from "@/lib/dm-utils";
import { deleteRoom, listRooms } from "@/lib/room-api";
import { cn, formatRelativeTime } from "@/lib/utils";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import { Agent } from "@/types/agent";
import { RoomAggregate } from "@/types/room";

/** 获取 DM Agent 名称首字母作为头像 */
function get_dm_avatar_letter(room: RoomAggregate, agents: Agent[]): string {
  const name = get_dm_display_name(room, agents);
  return name.charAt(0).toUpperCase();
}

export const DmsPanelContent = memo(function DmsPanelContent() {
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const active_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_item = useSidebarStore((s) => s.set_active_panel_item);

  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [search_query, set_search_query] = useState("");
  const [delete_target, set_delete_target] = useState<{ id: string; name: string } | null>(null);

  // 刷新列表
  const refresh = useCallback(() => {
    void listRooms(200).then(set_rooms);
  }, []);

  // 初始化加载
  useEffect(() => {
    void load_agents();
    refresh();
  }, [load_agents, refresh]);

  // 过滤 DM 并按时间排序
  const dm_rooms = useMemo(() => {
    const dms = rooms.filter((r) => r.room.room_type === "dm");
    // 按更新时间降序
    const sorted = [...dms].sort((a, b) => {
      const ta = new Date(
        a.room.updated_at ?? a.room.created_at ?? 0,
      ).getTime();
      const tb = new Date(
        b.room.updated_at ?? b.room.created_at ?? 0,
      ).getTime();
      return tb - ta;
    });
    // 搜索过滤
    if (!search_query.trim()) return sorted;
    const q = search_query.toLowerCase();
    return sorted.filter((room) =>
      get_dm_display_name(room, agents).toLowerCase().includes(q),
    );
  }, [agents, rooms, search_query]);

  // 点击 DM 条目
  const handle_click = useCallback(
    (room_id: string) => {
      set_active_item(room_id);
      navigate(AppRouteBuilders.room(room_id));
    },
    [navigate, set_active_item],
  );

  return (
    <div className="flex flex-col gap-2">
      {/* 搜索框 */}
      <div className="relative px-1">
        <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
        <input
          className="w-full rounded-lg border border-[var(--surface-panel-subtle-border)] bg-[var(--surface-panel-subtle-background)] py-1.5 pl-7 pr-2 text-[12px] text-slate-700 placeholder:text-slate-400 focus:bg-[var(--surface-interactive-hover-background)] focus:outline-none"
          onChange={(e) => set_search_query(e.target.value)}
          placeholder="搜索私信..."
          type="text"
          value={search_query}
        />
      </div>

      {/* DM 列表 */}
      <div className="flex flex-col gap-0.5">
        {dm_rooms.length > 0 ? (
          dm_rooms.map((room) => {
            const name = get_dm_display_name(room, agents);
            const avatar = get_dm_avatar_letter(room, agents);
            const timestamp = new Date(
              room.room.updated_at ?? room.room.created_at ?? 0,
            ).getTime();
            const is_active = active_item_id === room.room.id;

            return (
              <button
                key={room.room.id}
                className={cn(
                  "group box-border flex w-full items-center gap-2.5 rounded-lg border border-transparent px-2 py-2 text-left transition-all duration-150",
                  is_active
                    ? ""
                    : "hover:bg-[var(--surface-interactive-hover-background)]",
                )}
                style={is_active ? {
                  background: "var(--surface-interactive-active-background)",
                  borderColor: "var(--surface-interactive-active-border)",
                } : undefined}
                onClick={() => handle_click(room.room.id)}
                type="button"
              >
                {/* Agent 头像 */}
                <div
                  className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-[11px] font-bold"
                  style={{
                    background: "var(--surface-avatar-background)",
                    color: "var(--surface-avatar-foreground)",
                  }}
                >
                  {avatar}
                </div>

                {/* 名称 + 预览 */}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-1">
                    <span className="truncate text-[12px] font-semibold text-slate-800">
                      {name}
                    </span>
                    <span className="shrink-0 text-[10px] text-slate-400">
                      {timestamp > 0 ? formatRelativeTime(timestamp) : ""}
                    </span>
                  </div>
                  <p className="mt-0.5 truncate text-[11px] text-slate-500">
                    1v1
                  </p>
                </div>

                {/* 删除按钮 */}
                <span
                  className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-slate-400 opacity-0 transition-all hover:text-red-500 group-hover:opacity-100"
                  onClick={(e) => { e.stopPropagation(); set_delete_target({ id: room.room.id, name }); }}
                  role="button"
                  tabIndex={-1}
                >
                  <Trash2 className="h-3 w-3" />
                </span>
              </button>
            );
          })
        ) : (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <MessageCircleMore className="h-5 w-5 text-slate-300" />
            <p className="text-[11px] text-slate-400">
              {search_query ? "没有匹配的私信" : "暂无私信"}
            </p>
          </div>
        )}
      </div>

      {/* 删除确认对话框 */}
      <ConfirmDialog
        confirm_text="删除"
        is_open={delete_target !== null}
        message={`确定要删除与「${delete_target?.name ?? ""}」的私信吗？所有对话记录将被永久删除。`}
        on_cancel={() => set_delete_target(null)}
        on_confirm={() => {
          if (!delete_target) return;
          void deleteRoom(delete_target.id).then(() => {
            set_delete_target(null);
            refresh();
          });
        }}
        title="删除私信"
        variant="danger"
      />
    </div>
  );
});

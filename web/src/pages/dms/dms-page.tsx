/**
 * DMs 页面
 *
 * 当有 DM 时自动重定向到最近 DM 的 RoomPage（复用 ConversationWorkspace）。
 * 当没有 DM 时显示空状态引导用户从侧边栏选择 Agent 开始私聊。
 */

import { useEffect, useMemo, useState } from "react";
import { MessageCircle } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { listRooms, getRoomContexts } from "@/lib/room-api";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { AppStage } from "@/shared/ui/app-stage";
import { WorkspacePageFrame } from "@/shared/ui/workspace-page-frame";
import { RoomAggregate } from "@/types/room";

/** 按更新时间降序排列 */
function sort_rooms_desc(rooms: RoomAggregate[]) {
  return [...rooms].sort((left, right) => {
    const left_timestamp = new Date(left.room.updated_at ?? left.room.created_at ?? 0).getTime();
    const right_timestamp = new Date(right.room.updated_at ?? right.room.created_at ?? 0).getTime();
    return right_timestamp - left_timestamp;
  });
}

/** DM 空状态组件 */
function DmsEmptyState() {
  return (
    <div className="relative flex min-h-0 flex-1 items-center justify-center px-4 py-6 sm:px-6">
      <section className="max-w-md text-center">
        {/* 图标 */}
        <div className="workspace-chip mx-auto flex h-16 w-16 items-center justify-center rounded-2xl">
          <MessageCircle className="h-7 w-7 text-slate-500/80" />
        </div>

        {/* 标题 */}
        <h2 className="mt-5 text-xl font-bold tracking-tight text-slate-900/90">
          选择一个对话开始聊天
        </h2>

        {/* 描述 */}
        <p className="mt-2 text-sm leading-6 text-slate-500">
          从左侧面板选择一个 Agent 开始私聊
        </p>
      </section>
    </div>
  );
}

export function DmsPage() {
  const navigate = useNavigate();
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [is_loading, set_is_loading] = useState(true);

  // 加载所有 Room 数据
  useEffect(() => {
    let is_cancelled = false;

    async function bootstrap() {
      try {
        const next_rooms = await listRooms(200);
        if (is_cancelled) return;
        set_rooms(next_rooms);
      } finally {
        if (!is_cancelled) set_is_loading(false);
      }
    }

    void bootstrap();
    return () => { is_cancelled = true; };
  }, []);

  // 找到最近的 DM Room
  const latest_dm_room = useMemo(() => (
    sort_rooms_desc(rooms).find((room) => room.room.room_type === "dm") ?? null
  ), [rooms]);

  // 自动重定向到最近的 DM 对话（复用 RoomPage 的 ConversationWorkspace）
  useEffect(() => {
    if (is_loading || !latest_dm_room) return;

    let is_cancelled = false;
    const target_room = latest_dm_room;

    async function open_latest_dm() {
      const contexts = await getRoomContexts(target_room.room.id);
      if (is_cancelled) return;

      if (contexts[0]?.conversation?.id) {
        navigate(
          AppRouteBuilders.room_conversation(
            target_room.room.id,
            contexts[0].conversation.id,
          ),
          { replace: true },
        );
        return;
      }

      navigate(AppRouteBuilders.room(target_room.room.id), { replace: true });
    }

    void open_latest_dm();
    return () => { is_cancelled = true; };
  }, [is_loading, latest_dm_room, navigate]);

  // 加载中
  if (is_loading) {
    return <AppLoadingScreen />;
  }

  // 无 DM 时显示空状态
  return (
    <AppStage>
      <WorkspacePageFrame>
        <DmsEmptyState />
      </WorkspacePageFrame>
    </AppStage>
  );
}

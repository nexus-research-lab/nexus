/**
 * DMs 页面
 *
 * 当有 DM 时自动重定向到最近 DM 的 RoomPage（复用 ConversationWorkspace）。
 * 当没有 DM 时显示空状态引导用户从侧边栏选择 Agent 开始私聊。
 * 应用外层布局已提升到路由层，此处只渲染内容区域。
 */

import { useEffect, useMemo, useState } from "react";
import { MessageCircle } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { list_rooms, get_room_contexts, subscribe_room_list_updates } from "@/lib/api/room-api";
import { sort_rooms_by_recency } from "@/lib/conversation/room-utils";
import { WorkspaceCatalogTextAction } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { WorkspaceEmptyState } from "@/shared/ui/workspace/frame/workspace-empty-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { RoomAggregate } from "@/types/conversation/room";

export function DmsPage() {
  const navigate = useNavigate();
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [is_loading, set_is_loading] = useState(true);

  // 加载所有 Room 数据
  useEffect(() => {
    let is_cancelled = false;

    async function bootstrap() {
      try {
        const next_rooms = await list_rooms(200);
        if (is_cancelled) return;
        set_rooms(next_rooms);
      } finally {
        if (!is_cancelled) set_is_loading(false);
      }
    }

    void bootstrap();
    return () => { is_cancelled = true; };
  }, []);

  useEffect(() => subscribe_room_list_updates(() => {
    void list_rooms(200).then(set_rooms);
  }), []);

  // 找到最近的 DM Room
  const latest_dm_room = useMemo(() => (
    sort_rooms_by_recency(rooms).find((room) => room.room.room_type === "dm") ?? null
  ), [rooms]);

  // 自动重定向到最近的 DM 对话
  useEffect(() => {
    if (is_loading || !latest_dm_room) return;

    let is_cancelled = false;
    const target_room = latest_dm_room;

    async function open_latest_dm() {
      const contexts = await get_room_contexts(target_room.room.id);
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

  // 只渲染内容区域 — 外层布局由路由层提供
  return (
    <WorkspacePageFrame>
      <WorkspaceEmptyState
        actions={(
          <WorkspaceCatalogTextAction onClick={() => navigate(AppRouteBuilders.contacts())} tone="primary">
            打开 Contacts
          </WorkspaceCatalogTextAction>
        )}
        description="从左侧侧边栏或 Contacts 选择一个 Agent，即可创建新的 DM。"
        icon={<MessageCircle className="h-6 w-6 text-(--icon-default)" />}
        title="还没有打开中的私聊"
      />
    </WorkspacePageFrame>
  );
}

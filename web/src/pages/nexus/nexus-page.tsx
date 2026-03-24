import { useParams } from "react-router-dom";

import { NexusPlaceholder } from "@/features/nexus-chat/nexus-placeholder";
import { RouteScaffold } from "@/shared/ui/route-scaffold";

export function NexusPage() {
  const params = useParams<{ conversationId?: string }>();

  return (
    <RouteScaffold
      badge="NEXUS"
      title="系统级协作入口"
      description="这里会承接创建成员、创建 room、邀请成员、整理协作网络等系统级动作。当前阶段先建立独立页面边界，下一阶段再接入真实的 Nexus 对话流。"
      meta={
        params.conversationId ? (
          <div className="workspace-card rounded-[20px] px-4 py-3 text-right">
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              Conversation
            </p>
            <p className="mt-1 text-sm font-semibold text-slate-950/84">{params.conversationId}</p>
          </div>
        ) : null
      }
    >
      <NexusPlaceholder conversationId={params.conversationId} />
    </RouteScaffold>
  );
}

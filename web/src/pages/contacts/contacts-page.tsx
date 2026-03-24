import { useParams } from "react-router-dom";

import { ContactsPlaceholder } from "@/features/contacts-list/contacts-placeholder";
import { RouteScaffold } from "@/shared/ui/route-scaffold";

export function ContactsPage() {
  const params = useParams<{ agentId?: string }>();

  return (
    <RouteScaffold
      badge="CONTACTS"
      title="成员与联系人网络"
      description="这里会负责浏览成员、查看 profile、按技能筛选、发起 1v1，以及把成员邀请进 room。当前阶段先建立独立页面边界。"
      meta={
        params.agentId ? (
          <div className="workspace-card rounded-[20px] px-4 py-3 text-right">
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              Agent
            </p>
            <p className="mt-1 text-sm font-semibold text-slate-950/84">{params.agentId}</p>
          </div>
        ) : null
      }
    >
      <ContactsPlaceholder agentId={params.agentId} />
    </RouteScaffold>
  );
}

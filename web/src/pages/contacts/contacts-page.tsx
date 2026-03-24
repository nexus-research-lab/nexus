import { useEffect } from "react";
import { useParams } from "react-router-dom";

import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { RouteScaffold } from "@/shared/ui/route-scaffold";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { ContactsRouteParams } from "@/types/route";

export function ContactsPage() {
  const params = useParams<ContactsRouteParams>();
  const { agents, load_agents_from_server, loading } = useAgentStore();
  const { conversations, load_conversations_from_server } = useConversationStore();

  useEffect(() => {
    void load_agents_from_server();
    void load_conversations_from_server();
  }, [load_agents_from_server, load_conversations_from_server]);

  if (loading && !agents.length) {
    return <AppLoadingScreen />;
  }

  return (
    <RouteScaffold
      badge="CONTACTS"
      title="成员与联系人网络"
      description="Contacts 负责发现成员、查看能力、发起 1v1，以及把成员带入 Nexus 或 room。它不应该继续混在 room 页面里承担联系人管理职责。"
      meta={
        params.agent_id ? (
          <div className="workspace-card rounded-[20px] px-4 py-3 text-right">
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              Agent
            </p>
            <p className="mt-1 text-sm font-semibold text-slate-950/84">{params.agent_id}</p>
          </div>
        ) : null
      }
    >
      <ContactsDirectory
        agents={agents}
        conversations={conversations}
        selected_agent_id={params.agent_id}
      />
    </RouteScaffold>
  );
}

import { useEffect } from "react";
import { useParams } from "react-router-dom";

import { NexusHubOverview } from "@/features/nexus/nexus-hub-overview";
import { RouteScaffold } from "@/shared/ui/route-scaffold";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { NexusRouteParams } from "@/types/route";

export function NexusPage() {
  const params = useParams<NexusRouteParams>();
  const { agents, load_agents_from_server, loading: agents_loading } = useAgentStore();
  const {
    conversations,
    load_conversations_from_server,
    loading: conversations_loading,
  } = useConversationStore();

  useEffect(() => {
    void load_agents_from_server();
    void load_conversations_from_server();
  }, [load_agents_from_server, load_conversations_from_server]);

  if (agents_loading && conversations_loading && !agents.length && !conversations.length) {
    return <AppLoadingScreen />;
  }

  return (
    <RouteScaffold
      badge="NEXUS"
      title="系统级协作与编排"
      description="Nexus 是系统级入口，不属于某个具体成员。它更适合负责创建 room、组织成员、恢复系统级对话，并决定协作如何开始。"
      meta={
        params.conversation_id ? (
          <div className="workspace-card rounded-[20px] px-4 py-3 text-right">
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              Conversation
            </p>
            <p className="mt-1 text-sm font-semibold text-slate-950/84">{params.conversation_id}</p>
          </div>
        ) : null
      }
    >
      <NexusHubOverview
        agents={agents}
        conversations={conversations}
        conversation_id={params.conversation_id}
      />
    </RouteScaffold>
  );
}

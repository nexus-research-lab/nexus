import { useEffect } from "react";
import { useParams } from "react-router-dom";

import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { AppStage } from "@/shared/ui/app-stage";
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
    <AppStage>
      <div className="relative flex min-h-0 flex-1 flex-col px-4 py-4 sm:px-6 sm:py-6">
        <section className="workspace-shell relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[30px] p-4 sm:p-6">
          <ContactsDirectory
            agents={agents}
            conversations={conversations}
            selected_agent_id={params.agent_id}
          />
        </section>
      </div>
    </AppStage>
  );
}

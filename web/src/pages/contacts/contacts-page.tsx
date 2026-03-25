import { useCallback, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { ensureDirectRoom } from "@/lib/room-api";
import { AppStage } from "@/shared/ui/app-stage";
import { AppLoadingScreen } from "@/shared/ui/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { ContactsRouteParams } from "@/types/route";

export function ContactsPage() {
  const params = useParams<ContactsRouteParams>();
  const navigate = useNavigate();
  const { agents, load_agents_from_server, loading } = useAgentStore();
  const { conversations, load_conversations_from_server } = useConversationStore();

  const handle_open_direct_room = useCallback((agent_id: string) => {
    void ensureDirectRoom(agent_id).then((context) => {
      navigate(
        AppRouteBuilders.room_conversation(
          context.room.id,
          context.conversation.id,
        ),
      );
    });
  }, [navigate]);

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
            on_open_direct_room={handle_open_direct_room}
            selected_agent_id={params.agent_id}
          />
        </section>
      </div>
    </AppStage>
  );
}

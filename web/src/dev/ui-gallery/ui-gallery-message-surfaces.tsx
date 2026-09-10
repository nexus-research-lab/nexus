// INPUT: Local private-thread/message fixtures and the current Gallery language.
// OUTPUT: Real private timelines and a user-message editor with observable local commands.
// POS: Development-only reading/editing fixture; no conversation command or resource request is dispatched.

import { useState } from "react";
import { PrivateEventTimeline } from "@/features/agents/private-domain/timeline/agent-private-domain-timeline";
import { MessageUserSection } from "@/features/conversation/shared/message/item/view/user/message-user-section";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { AgentPrivateEvent, AgentPrivateThread } from "@/types/agent/private-domain";
import type { UserMessage } from "@/types/conversation/message/entity";
import { galleryText } from "./ui-gallery-copy";
import { MessageReadingGallery } from "./ui-gallery-message-reading";

const PARTICIPANTS = [{ agent_id: "observer", name: "Nexus" }, { agent_id: "author", name: "Author" }];
const THREAD: AgentPrivateThread = {
  thread_id: "gallery-private-thread", agent_id: "observer", scope: "direct", participant_agent_ids: ["observer", "author"],
  peer_agent_ids: ["author"], participants: PARTICIPANTS, room_name: "Review room", conversation_title: "Report review", message_count: 3,
};
const EVENTS: AgentPrivateEvent[] = (["incoming", "outgoing", "self"] as const).map((direction, index) => ({
  message_id: direction, thread_id: THREAD.thread_id, direction, source_agent_id: index === 0 ? "author" : "observer",
  recipients: index === 1 ? ["author"] : [], participants: PARTICIPANTS, reply_route: { mode: "private" }, timestamp: 1788566400000,
  content: index === 0 ? "Review **the original source** before changing the conclusion." : index === 1 ? "请保留原始证据与文件来源。" : "Keep this note in the current private thread.",
}));
const MESSAGE: UserMessage = {
  message_id: "gallery-message", session_key: "gallery-session", agent_id: "observer", round_id: "gallery-round", role: "user",
  timestamp: 1788566400000, content: "Original message for editing.",
};

export function MessageSurfacesGallery() {
  const localization = useI18n();
  const [message, setMessage] = useState(MESSAGE);
  const [commands, setCommands] = useState<Array<{ round: string; content: string }>>([]);
  return (
    <section className="min-w-0 space-y-4 xl:col-span-2" data-gallery-message-surfaces>
      <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
        {galleryText(localization.locale, "消息阅读与编辑真实视图", "Message reading and editing")}
      </h2>
      <div className="grid min-w-0 gap-4 md:grid-cols-2">
        {[true, false].map((compact) => (
          <PrivateEventTimeline agentId="observer" className="h-80" compact={compact} events={EVENTS} failure={null}
            isLoading={false} key={String(compact)} localization={localization} onRetry={() => undefined} thread={THREAD} />
        ))}
      </div>
      <div className="min-w-0 py-3" data-gallery-message-editor>
        <MessageUserSection compact={false} message={message} onEditUserMessage={(round, content) => {
          setCommands((current) => [...current, { round, content }]);
          setMessage((current) => ({ ...current, content }));
        }} />
      </div>
      <output className={getUiTypographyClassName({ role: "caption", tone: "soft" })} data-gallery-message-commands>{JSON.stringify(commands)}</output>
      <MessageReadingGallery />
    </section>
  );
}

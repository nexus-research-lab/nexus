// INPUT: Browser-only mounting of real desktop/mobile history with local conversation fixtures.
// OUTPUT: Theme/locale/viewport checks, editing and deletion feedback without backend commands.
// POS: Lazy browser test fixture; never imported by production or the regular Gallery entry.

import { createElement as h, useState } from "react";
import { createRoot } from "react-dom/client";

import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";
import { RoomMobileConversationSwitcher } from "@/features/conversation/room/surface/mobile/room-mobile-conversation-switcher";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { RoomConversationView } from "@/types/conversation/conversation";

const conversation = (id: string, title: string): RoomConversationView => ({
  conversation_id: id, title, room_id: "history-fixture", session_key: `fixture:${id}`,
  session_id: null, created_at: 0, last_activity_at: 0, options: {}, is_draft: false,
});
const draft = { ...conversation("draft", "Unstarted draft"), is_draft: true };
const conversations = [
  conversation("alpha", "Alpha"),
  conversation("beta", "ResearchEvidenceAndVerificationWithAnExtendedConversationTitle"),
  { ...conversation("external-session:mail", "Mailbox discussion"), options: {
    external_session: true, channel_type: "feishu", external_identity: {
      channel_type: "feishu", account_hint: "account-with-an-extended-identifier", can_delete: false,
    },
  } },
  draft,
];

function HistoryFixture() {
  const [items, setItems] = useState(conversations);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [selected, setSelected] = useState("alpha");
  const [commands, setCommands] = useState<string[]>([]);
  const record = (command: string) => setCommands((current) => [...current, command]);
  return h("div", { "data-room-history-fixture": true, className: "fixed inset-0 bg-(--background) p-4" },
    h(RoomHistoryMenu, {
      conversationId: selected, conversations: items,
      onSelectConversation: setSelected,
      onCreateConversation: async () => { record("create"); return "draft"; },
      onDeleteConversation: async (id: string) => {
        record(`delete:${id}`);
        if (id === "beta") throw new Error("Fixture unknown result");
        return null;
      },
      onUpdateConversationTitle: async (id: string, title: string) => {
        record(`rename:${id}:${title}`);
        setItems((current) => current.map((item) => item.conversation_id === id ? { ...item, title } : item));
      },
    }),
    h("button", { type: "button", onClick: () => setMobileOpen(true) }, "Open mobile history"),
    h("button", { type: "button", onClick: () => setItems([draft]) }, "Use empty history"),
    h("output", { "data-history-commands": true, className: "block" }, commands.join("|")),
    h("output", { "data-history-selected": true }, selected),
    h(RoomMobileConversationSwitcher, {
      activeConversationId: selected, conversations: items, isOpen: mobileOpen,
      onClose: () => setMobileOpen(false), onSelect: setSelected,
    }),
  );
}

export function mountRoomHistoryFixture() {
  const container = document.createElement("div");
  document.body.append(container);
  createRoot(container).render(h(I18nProvider, null, h(HistoryFixture)));
}

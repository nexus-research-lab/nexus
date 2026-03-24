import { ComponentProps } from "react";

import { ChatInterface } from "@/components/chat/chat-interface";

export type RoomChatPanelProps = ComponentProps<typeof ChatInterface>;

export function RoomChatPanel(props: RoomChatPanelProps) {
  return <ChatInterface {...props} />;
}

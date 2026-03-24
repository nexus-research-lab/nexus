import ChatHeader from "@/components/chat/chat-header";

interface RoomConversationHeaderProps {
  currentAgentName: string | null;
  currentConversationId: string | null;
  currentConversationTitle: string | null;
  isLoading: boolean;
}

export function RoomConversationHeader({
  currentAgentName,
  currentConversationId,
  currentConversationTitle,
  isLoading,
}: RoomConversationHeaderProps) {
  return (
    <ChatHeader
      currentAgentName={currentAgentName}
      isLoading={isLoading}
      sessionKey={currentConversationId}
      sessionTitle={currentConversationTitle}
    />
  );
}

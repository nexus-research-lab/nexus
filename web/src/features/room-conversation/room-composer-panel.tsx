import ChatInput from "@/components/chat/chat-input";

interface RoomComposerPanelProps {
  compact: boolean;
  currentAgentName: string | null;
  isLoading: boolean;
  onSendMessage: (content: string) => void | Promise<void>;
  onStop: () => void;
}

export function RoomComposerPanel({
  compact,
  currentAgentName,
  isLoading,
  onSendMessage,
  onStop,
}: RoomComposerPanelProps) {
  return (
    <ChatInput
      compact={compact}
      currentAgentName={currentAgentName}
      isLoading={isLoading}
      onSendMessage={onSendMessage}
      onStop={onStop}
    />
  );
}

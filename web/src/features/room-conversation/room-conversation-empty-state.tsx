import { EmptyState } from "@/components/chat/empty-state";

interface RoomConversationEmptyStateProps {
  onCreateConversation: () => void;
}

export function RoomConversationEmptyState({
  onCreateConversation,
}: RoomConversationEmptyStateProps) {
  return <EmptyState onNewSession={onCreateConversation} />;
}

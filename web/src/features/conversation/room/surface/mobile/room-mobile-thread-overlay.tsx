import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";

import { useRoomThreadPanel } from "../../group/chat/use-room-thread-panel-data";
import { useGroupThread } from "../../group/thread/group-thread-state";

export function RoomMobileThreadOverlay() {
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (!activeThread || !threadPanelData) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
      <ConversationThreadPanel
        agentAvatar={threadPanelData.agentAvatar}
        agentId={activeThread.agentId}
        agentName={threadPanelData.agentName ?? activeThread.agentId}
        isLoading={threadPanelData.isLoading}
        layout="mobile"
        messages={threadPanelData.messages}
        onClose={closeThread}
        onOpenWorkspaceFile={threadPanelData.onOpenWorkspaceFile}
        onPermissionResponse={threadPanelData.onPermissionResponse}
        onStopMessage={threadPanelData.onStopMessage}
        pendingPermissions={threadPanelData.pendingPermissions}
        roundId={activeThread.roundId}
        userAvatar={threadPanelData.userAvatar}
      />
    </div>
  );
}

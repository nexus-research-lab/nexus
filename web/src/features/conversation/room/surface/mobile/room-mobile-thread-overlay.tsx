import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";

import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../../group/thread/live/use-room-thread-panel";
import { RoomThreadEmptyState } from "../room-thread-empty-state";

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
        agentName={threadPanelData.agentName}
        emptyContent={(
          <RoomThreadEmptyState isLoading={threadPanelData.isLoading} />
        )}
        headerSubtitle={null}
        isLoading={threadPanelData.isLoading}
        layout="mobile"
        messages={threadPanelData.messages}
        onClose={closeThread}
        onOpenWorkspaceFile={threadPanelData.onOpenWorkspaceFile}
        onPermissionResponse={threadPanelData.onPermissionResponse}
        pendingPermissions={threadPanelData.pendingPermissions}
        presentation="inspector"
        roundId={activeThread.roundId}
        unresolvedToolStatus={threadPanelData.unresolvedToolStatus}
      />
    </div>
  );
}

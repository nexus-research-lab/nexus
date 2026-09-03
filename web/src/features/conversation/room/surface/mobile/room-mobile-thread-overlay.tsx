// INPUT: 当前 Room Thread 状态与共享 Thread 面板资源。
// OUTPUT: 使用语义 dialog 层的窄窗全屏 Thread 表面。
// POS: Room 窄窗 Thread 挂载点；不拥有 Thread 选择、消息或关闭状态。

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import { cn } from "@/shared/ui/class-name";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";

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
    <div className={cn(
      "fixed inset-0 [background:var(--surface-popover-background)] backdrop-blur-2xl",
      getUiOverlayLayerClassName("dialog"),
    )}>
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

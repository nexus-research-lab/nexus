import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { cn } from "@/lib/utils";
import type { RoomSurfaceTabKey } from "@/types/conversation/room-surface";

import { useRoomThreadPanel } from "../../group/chat/use-room-thread-panel-data";
import { GroupThreadDetailPanel } from "../../group/thread/group-thread-detail-panel";
import { useGroupThread } from "../../group/thread/group-thread-state";

interface RoomThreadInlinePanelProps {
  activeSurfaceTab: RoomSurfaceTabKey;
  className?: string;
  editorWidthPercent: number;
  onStartEditorResize: () => void;
}

export function RoomThreadInlinePanel({
  activeSurfaceTab,
  className,
  editorWidthPercent,
  onStartEditorResize,
}: RoomThreadInlinePanelProps) {
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (activeSurfaceTab !== "chat" || !activeThread || !threadPanelData) {
    return null;
  }

  return (
    <section
      className={cn(
        "relative ml-2 min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none",
        className,
      )}
      style={{
        width: `${editorWidthPercent}%`,
        minWidth: "360px",
        maxWidth: "560px",
      }}
    >
      <ConversationResizeHandle
        ariaLabel="调整 Thread 面板宽度"
        onMouseDown={onStartEditorResize}
      />

      <GroupThreadDetailPanel
        roundId={activeThread.roundId}
        agentId={activeThread.agentId}
        agentName={threadPanelData.agentName ?? activeThread.agentId}
        agentAvatar={threadPanelData.agentAvatar}
        userAvatar={threadPanelData.userAvatar}
        messages={threadPanelData.messages}
        pendingPermissions={threadPanelData.pendingPermissions}
        onPermissionResponse={threadPanelData.onPermissionResponse}
        onClose={closeThread}
        onStopMessage={threadPanelData.onStopMessage}
        onOpenWorkspaceFile={threadPanelData.onOpenWorkspaceFile}
        isLoading={threadPanelData.isLoading}
        layout="desktop"
      />
    </section>
  );
}

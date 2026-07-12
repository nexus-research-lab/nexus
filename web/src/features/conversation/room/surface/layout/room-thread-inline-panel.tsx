import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import { cn } from "@/shared/ui/class-name";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../../group/thread/live/use-room-thread-panel";

interface RoomThreadInlinePanelProps {
  activeSurfaceTab: RoomSurfaceTabKey;
  className?: string;
  sidePanelWidthPercent: number;
  onStartSidePanelResize: () => void;
}

export function RoomThreadInlinePanel({
  activeSurfaceTab,
  className,
  sidePanelWidthPercent,
  onStartSidePanelResize,
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
        width: `${sidePanelWidthPercent}%`,
        minWidth: "360px",
        maxWidth: "560px",
      }}
    >
      <PanelResizeHandle
        ariaLabel="调整 Thread 面板宽度"
        onResizeStart={onStartSidePanelResize}
      />

      <ConversationThreadPanel
        roundId={activeThread.roundId}
        agentId={activeThread.agentId}
        agentName={threadPanelData.agentName}
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

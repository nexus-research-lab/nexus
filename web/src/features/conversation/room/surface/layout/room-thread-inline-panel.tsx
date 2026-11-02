// INPUT: 当前 Thread、共享右栏百分比及布局调整命令。
// OUTPUT: 持续 Thread 阅读面与可键盘调整的具名分隔条。
// POS: Thread 右栏装配；尺寸限制归 Room 布局模型，消息归 Thread owner。

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../../group/thread/live/use-room-thread-panel";
import { RoomThreadEmptyState } from "../room-thread-empty-state";
import { RoomSidePanelContainer } from "./room-side-panel-container";

interface RoomThreadInlinePanelProps {
  activeSurfaceTab: RoomSurfaceTabKey;
  className?: string;
  sidePanelWidthPercent: number;
  onStartSidePanelResize: () => void;
  onSidePanelWidthChange: (percent: number) => void;
}

export function RoomThreadInlinePanel({
  activeSurfaceTab,
  className,
  sidePanelWidthPercent,
  onStartSidePanelResize,
  onSidePanelWidthChange,
}: RoomThreadInlinePanelProps) {
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (activeSurfaceTab !== "chat" || !activeThread || !threadPanelData) {
    return null;
  }

  return (
    <RoomSidePanelContainer
      className={className}
      kind="thread"
      onSidePanelWidthChange={onSidePanelWidthChange}
      onStartSidePanelResize={onStartSidePanelResize}
      sidePanelWidthPercent={sidePanelWidthPercent}
    >
      <ConversationThreadPanel
        roundId={activeThread.roundId}
        agentId={activeThread.agentId}
        agentName={threadPanelData.agentName}
        agentAvatar={threadPanelData.agentAvatar}
        emptyContent={(
          <RoomThreadEmptyState isLoading={threadPanelData.isLoading} />
        )}
        headerSubtitle={null}
        messages={threadPanelData.messages}
        pendingPermissions={threadPanelData.pendingPermissions}
        onPermissionResponse={threadPanelData.onPermissionResponse}
        onClose={closeThread}
        onOpenWorkspaceFile={threadPanelData.onOpenWorkspaceFile}
        isLoading={threadPanelData.isLoading}
        layout="desktop"
        presentation="inspector"
        unresolvedToolStatus={threadPanelData.unresolvedToolStatus}
      />
    </RoomSidePanelContainer>
  );
}

// INPUT: 当前 Thread、共享右栏百分比及布局调整命令。
// OUTPUT: 持续 Thread 阅读面与可键盘调整的具名分隔条。
// POS: Thread 右栏装配；尺寸限制归 Room 布局模型，消息归 Thread owner。

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../../group/thread/live/use-room-thread-panel";
import { RoomThreadEmptyState } from "../room-thread-empty-state";
import { useRoomSidePanelResize } from "./use-room-side-panel-resize";

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
  const { t } = useI18n();
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();
  const resize = useRoomSidePanelResize("thread", sidePanelWidthPercent, onSidePanelWidthChange);

  if (activeSurfaceTab !== "chat" || !activeThread || !threadPanelData) {
    return null;
  }

  return (
    <>
      <PanelResizeHandle
        ariaLabel={t("room.resize_thread_panel")}
        control={resize.control}
        controls={resize.panelId}
        onResizeStart={onStartSidePanelResize}
        variant="gutter"
      />

      <section
        id={resize.panelId}
        ref={resize.panelRef}
        className={cn(
          "nexus-room-surface-side-panel relative min-h-0 min-w-0 shrink-0 flex-col overflow-hidden",
          className,
        )}
        style={{
          width: `${sidePanelWidthPercent}%`,
          ...resize.widthStyle,
        }}
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
      </section>
    </>
  );
}

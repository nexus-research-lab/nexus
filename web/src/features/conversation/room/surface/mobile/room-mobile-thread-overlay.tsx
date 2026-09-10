// INPUT: 当前 Room Thread 状态与共享 Thread 面板资源。
// OUTPUT: 通过共用窄窗外壳展示按可读 Agent 名称命名的 Thread 模态。
// POS: Room 窄窗 Thread 挂载点；不拥有 Thread 选择、消息或关闭状态。

import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import { useI18n } from "@/shared/i18n/i18n-context";

import { useGroupThread } from "../../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../../group/thread/live/use-room-thread-panel";
import { RoomThreadEmptyState } from "../room-thread-empty-state";
import { RoomMobileOverlayFrame } from "./room-mobile-overlay-frame";

export function RoomMobileThreadOverlay() {
  const { t } = useI18n();
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (!activeThread || !threadPanelData) {
    return null;
  }

  return (
    <RoomMobileOverlayFrame label={t("room.thread_dialog", { name: threadPanelData.agentName })} onClose={closeThread}>
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
    </RoomMobileOverlayFrame>
  );
}

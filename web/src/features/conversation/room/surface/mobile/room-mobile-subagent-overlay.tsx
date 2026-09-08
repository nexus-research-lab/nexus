// INPUT: Room 子智能体来源、成员筛选、精确任务请求与关闭命令。
// OUTPUT: 通过统一窄窗外壳挂载具名任务模态，空 source 不挂载。
// POS: Room 窄窗子智能体挂载点；不拥有列表、任务详情或读取状态。

import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomSubagentTaskSurface } from "../room-subagent-task-surface";
import { RoomMobileOverlayFrame } from "./room-mobile-overlay-frame";

interface RoomMobileSubagentOverlayProps {
  currentAgentId: string;
  onClose: () => void;
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
  requestKey?: number;
  requestedHostAgentId?: string | null;
  requestedTaskToolUseId?: string | null;
  roomMembers: Agent[];
  source: SubagentTaskSource | null;
}

export function RoomMobileSubagentOverlay({
  currentAgentId,
  onClose,
  onOpenWorkspaceFile,
  requestKey,
  requestedHostAgentId,
  requestedTaskToolUseId,
  roomMembers,
  source,
}: RoomMobileSubagentOverlayProps) {
  const { t } = useI18n();
  if (!source) {
    return null;
  }

  return (
    <RoomMobileOverlayFrame label={t("subagents.panel_title")} onClose={onClose}>
      <RoomSubagentTaskSurface
        currentAgentId={currentAgentId}
        layout="mobile"
        onClose={onClose}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        requestKey={requestKey}
        requestedHostAgentId={requestedHostAgentId}
        requestedTaskToolUseId={requestedTaskToolUseId}
        roomMembers={roomMembers}
        source={source}
      />
    </RoomMobileOverlayFrame>
  );
}

// INPUT: Room 子智能体来源、成员筛选、精确任务请求与关闭命令。
// OUTPUT: 使用语义 dialog 层和稳定纵向骨架的窄窗子智能体表面。
// POS: Room 窄窗子智能体挂载点；不拥有列表、任务详情或读取状态。

import { cn } from "@/shared/ui/class-name";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomSubagentTaskSurface } from "../room-subagent-task-surface";

interface RoomMobileSubagentOverlayProps {
  currentAgentId: string;
  onClose: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
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
  if (!source) {
    return null;
  }

  return (
    <div className={cn(
      "fixed inset-0 flex min-h-0 flex-col [background:var(--surface-popover-background)] backdrop-blur-2xl",
      getUiOverlayLayerClassName("dialog"),
    )}>
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
    </div>
  );
}

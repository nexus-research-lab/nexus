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
    <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
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

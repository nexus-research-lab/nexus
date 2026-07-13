import { SubagentTaskSurface } from "@/features/conversation/shared/subagent/subagent-task-surface";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

interface RoomMobileSubagentOverlayProps {
  onClose: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  source: SubagentTaskSource | null;
}

export function RoomMobileSubagentOverlay({
  onClose,
  onOpenWorkspaceFile,
  source,
}: RoomMobileSubagentOverlayProps) {
  if (!source) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
      <SubagentTaskSurface
        layout="mobile"
        onClose={onClose}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        source={source}
      />
    </div>
  );
}

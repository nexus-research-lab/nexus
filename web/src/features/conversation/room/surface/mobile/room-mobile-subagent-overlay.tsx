// INPUT: Room 子智能体来源、成员筛选、精确任务请求与关闭命令。
// OUTPUT: 具名窄窗任务模态，复用焦点/键盘/滚动锁协议并保持稳定纵向骨架。
// POS: Room 窄窗子智能体挂载点；不拥有列表、任务详情或读取状态。

import { useRef } from "react";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useDialogModalBehavior } from "@/shared/ui/dialog/dialog-behavior";
import { cn } from "@/shared/ui/class-name";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomSubagentTaskSurface } from "../room-subagent-task-surface";

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
  const rootRef = useRef<HTMLDivElement>(null);
  useDialogModalBehavior({ enabled: source !== null, onClose, rootRef });
  if (!source) {
    return null;
  }

  return (
    <div
      ref={rootRef}
      role="dialog"
      aria-modal="true"
      data-modal-root="true"
      aria-label={t("subagents.panel_title")}
      tabIndex={-1}
      className={cn(
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

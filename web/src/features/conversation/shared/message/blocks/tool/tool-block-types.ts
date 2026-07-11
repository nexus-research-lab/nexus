import type {
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type {
  PermissionRiskLevel,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";

export type ToolBlockStatus =
  | "pending"
  | "running"
  | "success"
  | "error"
  | "waiting_permission";

export interface ToolPermissionRequest {
  request_id: string;
  tool_input: Record<string, unknown>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  on_allow: (updatedPermissions?: PermissionUpdate[]) => void;
  on_deny: (updatedPermissions?: PermissionUpdate[]) => void;
}

export interface ToolBlockProps {
  toolUse: ToolUseContent;
  toolResult?: ToolResultContent;
  /** 子智能体进度只属于当前工具执行，不进入独立时间线。 */
  liveProgress?: TaskProgressContent | null;
  status?: ToolBlockStatus;
  startTime?: number;
  endTime?: number;
  permissionRequest?: ToolPermissionRequest;
  interactionDisabled?: boolean;
  interactionDisabledReason?: string;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
}

export interface ToolPermissionSuggestion {
  index: number;
  label: string;
}

export type ToolStatusTone =
  | "default"
  | "error"
  | "running"
  | "success"
  | "waiting";

export interface ToolPrimaryInputDetail {
  key: string;
  label: string;
  value: string;
}

export interface ToolBlockViewModel {
  collapsedDetailText: string | null;
  durationText: string;
  expandedDetailText: string | null;
  hasResult: boolean;
  isRunning: boolean;
  isWaiting: boolean;
  liveStatusText: string | null;
  primaryInputDetail: ToolPrimaryInputDetail | null;
  readableSuggestions: ToolPermissionSuggestion[];
  status: ToolBlockStatus;
  statusBadgeClassName: string;
  statusText: string;
  statusTone: ToolStatusTone;
  toolTitle: string;
  waitingActionHint: string;
}

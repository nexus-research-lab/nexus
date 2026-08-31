/**
 * INPUT: ToolBlock 的工具、结果、权限与运行时状态。
 * OUTPUT: DM/Room 工具执行行共用属性及展示状态契约。
 * POS: ToolBlock 视图层协议，不承载 Provider 或 Agent 决策。
 */
import type {
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type {
  PermissionRiskLevel,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";

import type { ExecutionToolVisualKind } from "../../../execution/execution-tool-visual";

export type ToolBlockStatus =
  | "pending"
  | "running"
  | "success"
  | "rejected"
  | "superseded"
  | "error"
  | "stopped"
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
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
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
  expandedInputText: string | null;
  hasResult: boolean;
  liveStatusText: string | null;
  primaryInputDetail: ToolPrimaryInputDetail | null;
  readableSuggestions: ToolPermissionSuggestion[];
  status: ToolBlockStatus;
  statusText: string;
  statusTone: ToolStatusTone;
  toolTitle: string;
  toolVisualKind: ExecutionToolVisualKind;
  waitingActionHint: string;
}

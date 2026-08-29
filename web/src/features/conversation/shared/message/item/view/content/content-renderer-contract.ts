import type { ContentBlock } from "@/types/conversation/message/content";
import type { AgentMention } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { MessageActivityState } from "../../activity/message-activity-state";
import type { AgentMentionDirectory } from "../../../agent-mention-chip";
import type { PendingInteractionOwner } from "../../message-item-projection";
import type { ToolBlockStatus } from "../../../blocks/tool/tool-block-types";

export type UnresolvedToolStatus = Extract<
  ToolBlockStatus,
  "error" | "stopped"
>;

export interface ContentRendererProps {
  canRespondToPermissions?: boolean;
  className?: string;
  content: string | ContentBlock[];
  defaultThinkingExpanded?: boolean;
  fallbackActivityLabel?: string | null;
  fallbackActivityState?: MessageActivityState | null;
  hiddenToolNames?: readonly string[];
  isStreaming?: boolean;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingInteractionOwner?: PendingInteractionOwner;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  renderLeadingSlashCommand?: boolean;
  /** 外层过程栏已拥有瞬时状态时，禁止再追加同义尾随活动。 */
  showTrailingActivity?: boolean;
  showTimelineDots?: boolean;
  streamingBlockIndexes?: ReadonlySet<number>;
  /** 执行已终止但 provider 未返回 tool_result 时的权威消息级收口。 */
  unresolvedToolStatus?: UnresolvedToolStatus;
  workspaceAgentId?: string | null;
  agentMentions?: AgentMention[];
  agentMentionDirectory?: AgentMentionDirectory;
  onOpenAgentContact?: (agentId: string) => void;
}

export interface StructuredContentRendererProps
  extends Omit<ContentRendererProps, "content"> {
  content: ContentBlock[];
}

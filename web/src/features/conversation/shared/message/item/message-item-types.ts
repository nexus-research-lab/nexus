import type { ReactNode } from "react";

import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { AssistantContentMode } from "./message-item-projection";
import type { MessageActivityState } from "./activity/message-activity-state";
import type { AgentMentionDirectory } from "../agent-mention-chip";
import type { UnresolvedToolStatus } from "./view/content/content-renderer-contract";

export interface MessageItemProps {
  animateEntry?: boolean;
  compact?: boolean;
  currentAgentName?: string | null;
  currentAgentAvatar?: string | null;
  workspaceAgentId?: string | null;
  roundId: string;
  messages: Message[];
  isLastRound?: boolean;
  isLoading?: boolean;
  /** Consumer-owned execution evidence projected onto the shared activity vocabulary. */
  activityState?: MessageActivityState | null;
  runtimePhase?: AgentConversationRuntimePhase | null;
  unresolvedToolStatus?: UnresolvedToolStatus;
  pendingPermissions?: PendingPermission[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  hiddenToolNames?: string[];
  onEditUserMessage?: (messageId: string, newContent: string) => void;
  onForkConversation?: () => Promise<void>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onStopMessage?: (msgId: string) => void;
  defaultProcessExpanded?: boolean;
  assistantHeaderAction?: ReactNode;
  /** 没有正文时仍需保留的终态说明，不伪造 assistant 消息。 */
  assistantEmptyState?: ReactNode;
  assistantContentMode?: AssistantContentMode;
  /** 上层已有稳定身份头时可隐藏内部重复头部。 */
  showAssistantHeader?: boolean;
  /** 检查器只读过程时可隐藏主 Feed 已展示的用户输入。 */
  showUserMessages?: boolean;
  className?: string;
  agentMentionDirectory?: AgentMentionDirectory;
}

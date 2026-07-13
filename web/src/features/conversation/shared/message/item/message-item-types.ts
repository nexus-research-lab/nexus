import type { ReactNode } from "react";

import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { AssistantContentMode } from "./message-item-projection";

export interface MessageItemProps {
  compact?: boolean;
  currentAgentName?: string | null;
  currentAgentAvatar?: string | null;
  workspaceAgentId?: string | null;
  currentUserAvatar?: string | null;
  roundId: string;
  messages: Message[];
  isLastRound?: boolean;
  isLoading?: boolean;
  runtimePhase?: AgentConversationRuntimePhase | null;
  pendingPermissions?: PendingPermission[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  hiddenToolNames?: string[];
  onEditUserMessage?: (messageId: string, newContent: string) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onStopMessage?: (msgId: string) => void;
  defaultProcessExpanded?: boolean;
  assistantHeaderAction?: ReactNode;
  assistantContentMode?: AssistantContentMode;
  className?: string;
}

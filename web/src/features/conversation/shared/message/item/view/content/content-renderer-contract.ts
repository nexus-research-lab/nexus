import type { ContentBlock } from "@/types/conversation/message/content";
import type { AgentMention } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { MessageActivityState } from "../../activity/message-activity-state";
import type { AgentMentionDirectory } from "../../../agent-mention-chip";

export interface ContentRendererProps {
  canRespondToPermissions?: boolean;
  className?: string;
  content: string | ContentBlock[];
  fallbackActivityState?: MessageActivityState | null;
  hiddenToolNames?: readonly string[];
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  showTimelineDots?: boolean;
  streamingBlockIndexes?: ReadonlySet<number>;
  workspaceAgentId?: string | null;
  agentMentions?: AgentMention[];
  agentMentionDirectory?: AgentMentionDirectory;
  onOpenAgentContact?: (agentId: string) => void;
}

export interface StructuredContentRendererProps
  extends Omit<ContentRendererProps, "content"> {
  content: ContentBlock[];
}

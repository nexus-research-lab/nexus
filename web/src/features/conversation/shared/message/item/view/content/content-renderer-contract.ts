import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { MessageActivityState } from "../../activity/message-activity-state";

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
}

export interface StructuredContentRendererProps
  extends Omit<ContentRendererProps, "content"> {
  content: ContentBlock[];
}

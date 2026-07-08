/**
 * =====================================================
 * @File   ：message-item-types.ts
 * @Date   ：2026-04-16 15:54
 * @Author ：leemysw
 * 2026-04-16 15:54   Create
 * =====================================================
 */

import type { CSSProperties, ReactNode } from "react";

import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type {
  AssistantMessage,
  ContentBlock,
  Message,
  MessageAttachment,
} from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import type { ContentProjection } from "./message-item-support";
import type { MessageActivityState } from "../ui/message-primitives";

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
  assistantContentMode?: "dm_live" | "dm_archived" | "room_thread" | "room_result";
  className?: string;
}

export interface MessageStatsData {
  duration: string | null;
  tokens: string | null;
  cost: string | null;
  cacheHit: string | null;
}

export interface MessageItemState {
  copiedUser: boolean;
  copiedAssistant: boolean;
  userMessage: Message | undefined;
  userContent: string;
  userAttachments: MessageAttachment[];
  assistantAgentId: string | null;
  model: string | undefined;
  timestamp: number | undefined;
  streamStatus: AssistantMessage["stream_status"] | null;
  stopReason: AssistantMessage["stop_reason"] | null;
  stats: MessageStatsData | null;
  matchedPendingPermissionsByToolUseId: ReadonlyMap<string, PendingPermission>;
  unmatchedPendingPermissions: PendingPermission[];
  directOrderedProjection: ContentProjection;
  processProjection: ContentProjection;
  finalAssistantContent: string | ContentBlock[] | null;
  finalAssistantStreamingIndexes: Set<number>;
  finalAssistantText: string;
  shouldRenderDirectAssistantContent: boolean;
  shouldRenderProcessCallchain: boolean;
  shouldRenderAssistantText: boolean;
  shouldRenderStandaloneActivityStatus: boolean;
  shouldShowAssistantFooter: boolean;
  showCursor: boolean;
  finalAssistantIsStreaming: boolean;
  shouldHideAssistantContent: boolean;
  processSummary: string;
  liveActivityState: MessageActivityState | null;
  isProcessExpanded: boolean;
  toggleProcessExpanded: () => void;
  processAnchorRef: React.RefObject<HTMLElement | null>;
  canCopyAssistant: boolean;
  canStopMessage: boolean;
  handleCopyUser: () => Promise<void>;
  handleCopyAssistant: () => Promise<void>;
  handleStopMessage: () => void;
  contentAreaRef: React.RefObject<HTMLDivElement | null>;
  contentAreaStyle: CSSProperties | undefined;
  mergedContentLength: number;
}

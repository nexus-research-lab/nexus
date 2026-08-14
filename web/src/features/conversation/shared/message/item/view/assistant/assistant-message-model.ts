/**
 * INPUT: controller 已投影的 Assistant 活动、内容、权限、布局及交互状态。
 * OUTPUT: Assistant 子视图按职责消费的窄状态与环境契约。
 * POS: MessageItem controller 到 Assistant 视图的类型边界，不选择内容模式策略。
 */
import type {
  CSSProperties,
  ReactNode,
  RefObject,
} from "react";

import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  AgentMention,
  GoalCompletionReceipt,
  PublicHandoffReply,
  RecalledMemoryReference,
} from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import { CONVERSATION_ASSISTANT_FRAME_WIDTH_CLASS_NAME } from "../../../../conversation-panel-styles";
import type {
  AssistantContentMode,
  ContentProjection,
  PendingInteractionOwner,
} from "../../message-item-projection";
import type { MessageActivityState } from "../../activity/message-activity-state";
import type { ProcessSummaryProjection } from "../../process/message-process-summary";
import type { UnresolvedToolStatus } from "../content/content-renderer-contract";

export interface AssistantActivityState {
  emptyStreamStatus: "cancelled" | "error" | null;
  showCursor: boolean;
  standalone: boolean;
  state: MessageActivityState | null;
}

export interface AssistantDirectState {
  projection: ContentProjection;
  visible: boolean;
}

export interface AssistantFinalState {
	content: string | ContentBlock[] | null;
	mentions: AgentMention[];
  isStreaming: boolean;
  streamingIndexes: ReadonlySet<number>;
  visible: boolean;
}

export interface AssistantFooterState {
  copied: boolean;
  goalCompletionReceipt: GoalCompletionReceipt | null;
  memories: RecalledMemoryReference[];
  onCopy?: () => Promise<void>;
  stats: AssistantFooterStats | null;
  visible: boolean;
}

export interface AssistantFooterStats {
  cacheReadTokens: string | null;
  cost: string | null;
  duration: string | null;
  tokens: string | null;
}

interface AssistantHeaderState {
  agentId: string | null;
  automationTaskName: string | null;
  canStop: boolean;
  handoffReply: PublicHandoffReply | null;
  stop: () => void;
  timestamp: number | undefined;
}

interface AssistantLayoutState {
  contentAreaRef: RefObject<HTMLDivElement | null>;
  contentAreaStyle: CSSProperties | undefined;
}

export interface AssistantPermissionState {
  all: PendingPermission[];
  matchedByToolUseId: ReadonlyMap<string, PendingPermission>;
  owner: PendingInteractionOwner;
  unmatched: PendingPermission[];
}

export interface AssistantProcessState {
  anchorRef: RefObject<HTMLElement | null>;
  expanded: boolean;
  projection: ContentProjection;
  summary: ProcessSummaryProjection;
  toggle: () => void;
  visible: boolean;
}

interface MessageAssistantState {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  final: AssistantFinalState;
  footer: AssistantFooterState;
  header: AssistantHeaderState;
  hidden: boolean;
  layout: AssistantLayoutState;
  model?: string;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
  showMaxTokensWarning: boolean;
}

export interface AssistantContentEnvironment {
  canRespondToPermissions: boolean;
  hiddenToolNames: string[];
  mode: AssistantContentMode;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissionReadOnlyReason?: string;
	workspaceAgentId?: string | null;
	unresolvedToolStatus?: UnresolvedToolStatus;
	agentMentionDirectory?: import("../../../agent-mention-chip").AgentMentionDirectory;
	onOpenAgentContact?: (agentId: string) => void;
}

export interface MessageAssistantSectionProps {
  assistant: MessageAssistantState;
  assistantContentMode: AssistantContentMode;
  assistantEmptyState?: ReactNode;
  assistantHeaderAction?: ReactNode;
  canRespondToPermissions: boolean;
  compact: boolean;
  currentAgentAvatar?: string | null;
  currentAgentName?: string | null;
  hiddenToolNames: string[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissionReadOnlyReason?: string;
  showHeader: boolean;
  workspaceAgentId?: string | null;
  unresolvedToolStatus?: UnresolvedToolStatus;
  agentMentionDirectory?: import("../../../agent-mention-chip").AgentMentionDirectory;
}

interface AssistantMessageScopeOptions {
  assistantAgentId: string | null;
  hasContactAction: boolean;
  workspaceAgentId?: string | null;
}

export interface AssistantMessageScope {
  canOpenContact: boolean;
  contactAgentId: string | null;
  contentWorkspaceAgentId?: string | null;
}

export function resolveAssistantMessageScope({
  assistantAgentId,
  hasContactAction,
  workspaceAgentId,
}: AssistantMessageScopeOptions): AssistantMessageScope {
  const contentWorkspaceAgentId = resolveContentWorkspaceAgentId(
    assistantAgentId,
    workspaceAgentId,
  );
  const contactAgentId = contentWorkspaceAgentId ?? null;
  return {
    canOpenContact: hasContactAction && Boolean(contactAgentId),
    contactAgentId,
    contentWorkspaceAgentId,
  };
}

function resolveContentWorkspaceAgentId(
  assistantAgentId: string | null,
  workspaceAgentId?: string | null,
) {
  return assistantAgentId ?? workspaceAgentId;
}

const ASSISTANT_LAYOUTS = {
  compact: {
    content: "pt-1 text-base leading-6",
    inner: "max-w-full",
    section: "px-0",
    showMetadata: true,
  },
  expanded: {
    content: "w-full max-w-full pt-2 text-[16px] leading-7",
    inner: CONVERSATION_ASSISTANT_FRAME_WIDTH_CLASS_NAME,
    section: "px-2 sm:px-3",
    showMetadata: false,
  },
} as const;

export function resolveAssistantMessageLayout(compact: boolean) {
  return ASSISTANT_LAYOUTS[compact ? "compact" : "expanded"];
}

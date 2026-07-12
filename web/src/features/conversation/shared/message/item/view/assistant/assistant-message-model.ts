import type {
  CSSProperties,
  ReactNode,
  RefObject,
} from "react";

import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { AssistantContentMode, ContentProjection } from "../../message-item-projection";
import type { MessageActivityState } from "../../activity/message-activity-state";

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
  isStreaming: boolean;
  streamingIndexes: ReadonlySet<number>;
  visible: boolean;
}

export interface AssistantFooterState {
  copied: boolean;
  onCopy?: () => Promise<void>;
  stats: AssistantFooterStats | null;
  visible: boolean;
}

export interface AssistantFooterStats {
  cacheHit: string | null;
  cost: string | null;
  duration: string | null;
  tokens: string | null;
}

interface AssistantHeaderState {
  agentId: string | null;
  canStop: boolean;
  model: string | undefined;
  stop: () => void;
  timestamp: number | undefined;
}

interface AssistantLayoutState {
  contentAreaRef: RefObject<HTMLDivElement | null>;
  contentAreaStyle: CSSProperties | undefined;
}

export interface AssistantPermissionState {
  matchedByToolUseId: ReadonlyMap<string, PendingPermission>;
  unmatched: PendingPermission[];
}

export interface AssistantProcessState {
  anchorRef: RefObject<HTMLElement | null>;
  expanded: boolean;
  projection: ContentProjection;
  summary: string;
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
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
  showMaxTokensWarning: boolean;
}

export interface AssistantContentEnvironment {
  canRespondToPermissions: boolean;
  hiddenToolNames: string[];
  mode: AssistantContentMode;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissionReadOnlyReason?: string;
  workspaceAgentId?: string | null;
}

export interface MessageAssistantSectionProps {
  assistant: MessageAssistantState;
  assistantContentMode: AssistantContentMode;
  assistantHeaderAction?: ReactNode;
  canRespondToPermissions: boolean;
  compact: boolean;
  currentAgentAvatar?: string | null;
  currentAgentName?: string | null;
  hiddenToolNames: string[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissionReadOnlyReason?: string;
  workspaceAgentId?: string | null;
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

export function resolveAssistantDisplayName(name?: string | null): string {
  return name || "协作成员";
}

const ASSISTANT_LAYOUTS = {
  compact: {
    content: "text-[15px] leading-6",
    grid: "grid-cols-[minmax(0,1fr)]",
    inner: "max-w-full",
    section: "px-0",
    showSideAvatar: false,
  },
  expanded: {
    content: "text-[16px] leading-7",
    grid:
      "nexus-chat-assistant-grid-expanded grid-cols-[40px_minmax(0,1fr)] gap-3",
    inner: "max-w-[980px]",
    section: "px-2 sm:px-3",
    showSideAvatar: true,
  },
} as const;

export function resolveAssistantMessageLayout(compact: boolean) {
  return ASSISTANT_LAYOUTS[compact ? "compact" : "expanded"];
}

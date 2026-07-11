import {
  useCallback,
  type CSSProperties,
  type ReactNode,
  type RefObject,
} from "react";
import { AlertTriangle } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ContentBlock } from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import { MessageStats } from "../../../ui/message-stats";
import {
  MessageActivityStatus,
  type MessageActivityState,
} from "../../../ui/message-primitives";
import type { ContentProjection } from "../../message-item-projection";
import type { MessageStatsData } from "../../message-item-types";
import { ContentRenderer } from "../content/content-renderer";
import {
  AssistantMessageAvatar,
  AssistantMessageHeader,
} from "./assistant-message-header";
import { AssistantProcessCallchain } from "./assistant-process-callchain";
import { PendingPermissionList } from "./pending-permission-list";

const EMPTY_CONTENT_BLOCKS: ContentBlock[] = [];

interface MessageAssistantState {
  activity: {
    emptyStreamStatus: "cancelled" | "error" | null;
    showCursor: boolean;
    standalone: boolean;
    state: MessageActivityState | null;
  };
  direct: {
    projection: ContentProjection;
    visible: boolean;
  };
  final: {
    content: string | ContentBlock[] | null;
    isStreaming: boolean;
    streamingIndexes: ReadonlySet<number>;
    visible: boolean;
  };
  footer: {
    copied: boolean;
    onCopy?: () => Promise<void>;
    stats: MessageStatsData | null;
    visible: boolean;
  };
  header: {
    agentId: string | null;
    canStop: boolean;
    model: string | undefined;
    stop: () => void;
    timestamp: number | undefined;
  };
  hidden: boolean;
  layout: {
    contentAreaRef: RefObject<HTMLDivElement | null>;
    contentAreaStyle: CSSProperties | undefined;
  };
  permissions: {
    matchedByToolUseId: ReadonlyMap<string, PendingPermission>;
    unmatched: PendingPermission[];
  };
  process: {
    anchorRef: RefObject<HTMLElement | null>;
    expanded: boolean;
    projection: ContentProjection;
    summary: string;
    toggle: () => void;
    visible: boolean;
  };
  showMaxTokensWarning: boolean;
}

interface MessageAssistantSectionProps {
  assistant: MessageAssistantState;
  assistantContentMode:
    | "dm_live"
    | "dm_archived"
    | "room_thread"
    | "room_result";
  assistantHeaderAction?: ReactNode;
  canRespondToPermissions: boolean;
  compact: boolean;
  currentAgentAvatar?: string | null;
  currentAgentName?: string | null;
  hiddenToolNames?: string[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissionReadOnlyReason?: string;
  workspaceAgentId?: string | null;
}

export function MessageAssistantSection({
  assistant,
  assistantContentMode,
  assistantHeaderAction,
  canRespondToPermissions,
  compact,
  currentAgentAvatar,
  currentAgentName,
  hiddenToolNames = ["TodoWrite"],
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  permissionReadOnlyReason,
  workspaceAgentId,
}: MessageAssistantSectionProps) {
  const { t } = useI18n();
  const contentWorkspaceAgentId = assistant.header.agentId ?? workspaceAgentId;
  const avatarAgentId = assistant.header.agentId ?? workspaceAgentId ?? null;
  const collapsedProcessFileArtifacts = useWorkspaceFileArtifactsFromContent(
    assistant.process.visible && !assistant.process.expanded
      ? assistant.process.projection.content
      : EMPTY_CONTENT_BLOCKS,
  );
  const handleOpenAgentContact = useCallback(() => {
    if (avatarAgentId) {
      onOpenAgentContact?.(avatarAgentId);
    }
  }, [avatarAgentId, onOpenAgentContact]);

  if (assistant.hidden) {
    return null;
  }

  const pendingPermissionBlock = (
    <PendingPermissionList
      canRespond={canRespondToPermissions}
      isRoomThreadMode={assistantContentMode === "room_thread"}
      onResponse={onPermissionResponse}
      permissions={assistant.permissions.unmatched}
      readOnlyReason={permissionReadOnlyReason}
      workspaceAgentId={contentWorkspaceAgentId}
    />
  );
  const canOpenContact = Boolean(avatarAgentId && onOpenAgentContact);

  return (
    <div className={cn("nexus-chat-message-section w-full", compact ? "px-0" : "px-2 sm:px-3")}>
      <div className={cn("w-full", compact ? "max-w-full" : "max-w-[980px]")}>
        <div
          className={cn(
            "nexus-chat-assistant-grid group grid min-w-0",
            compact
              ? "grid-cols-[minmax(0,1fr)]"
              : "nexus-chat-assistant-grid-expanded grid-cols-[40px_minmax(0,1fr)] gap-3",
          )}
        >
          {!compact ? (
            <AssistantMessageAvatar
              avatarUrl={currentAgentAvatar}
              displayName={currentAgentName || "协作成员"}
              onOpenContact={canOpenContact ? handleOpenAgentContact : undefined}
            />
          ) : null}

          <div className="relative min-w-0">
            <AssistantMessageHeader
              avatarUrl={currentAgentAvatar}
              canOpenContact={canOpenContact}
              canStop={assistant.header.canStop}
              compact={compact}
              headerAction={assistantHeaderAction}
              model={assistant.header.model}
              name={currentAgentName}
              onOpenContact={handleOpenAgentContact}
              onStop={assistant.header.stop}
              timestamp={assistant.header.timestamp}
            />

            <div
              className={cn(
                "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
                compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
              )}
              ref={assistant.layout.contentAreaRef}
              style={assistant.layout.contentAreaStyle}
            >
              {assistant.activity.standalone ? (
                <MessageActivityStatus
                  className="py-1"
                  state={assistant.activity.state!}
                />
              ) : null}
              <EmptyStreamStatus status={assistant.activity.emptyStreamStatus} />

              {assistant.direct.visible ? (
                <div>
                  <ContentRenderer
                    canRespondToPermissions={canRespondToPermissions}
                    content={assistant.direct.projection.content}
                    fallbackActivityState={assistant.activity.state}
                    hiddenToolNames={hiddenToolNames}
                    isStreaming={assistant.activity.showCursor}
                    onOpenWorkspaceFile={onOpenWorkspaceFile}
                    onPermissionResponse={onPermissionResponse}
                    pendingPermissionsByToolUseId={assistant.permissions.matchedByToolUseId}
                    permissionReadOnlyReason={permissionReadOnlyReason}
                    showTimelineDots
                    streamingBlockIndexes={assistant.direct.projection.streamingIndexes}
                    workspaceAgentId={contentWorkspaceAgentId}
                  />
                  {pendingPermissionBlock}
                </div>
              ) : null}

              {assistant.process.visible ? (
                <AssistantProcessCallchain
                  anchorRef={assistant.process.anchorRef}
                  canRespondToPermissions={canRespondToPermissions}
                  collapsedFileArtifacts={collapsedProcessFileArtifacts}
                  fallbackActivityState={assistant.activity.state}
                  hiddenToolNames={hiddenToolNames}
                  isExpanded={assistant.process.expanded}
                  isStreaming={assistant.activity.showCursor}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  onPermissionResponse={onPermissionResponse}
                  pendingPermissionBlock={pendingPermissionBlock}
                  pendingPermissionsByToolUseId={assistant.permissions.matchedByToolUseId}
                  permissionReadOnlyReason={permissionReadOnlyReason}
                  processProjection={assistant.process.projection}
                  summary={assistant.process.summary}
                  toggleExpanded={assistant.process.toggle}
                  workspaceAgentId={contentWorkspaceAgentId}
                />
              ) : null}

              {assistant.final.visible ? (
                <ContentRenderer
                  content={assistant.final.content ?? []}
                  fallbackActivityState={assistant.activity.state}
                  isStreaming={assistant.final.isStreaming}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  streamingBlockIndexes={assistant.final.streamingIndexes}
                  workspaceAgentId={contentWorkspaceAgentId}
                />
              ) : null}

              {assistant.showMaxTokensWarning ? (
                <div className="mt-2 flex items-center gap-1.5 rounded-[8px] border border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)] px-3 py-2 text-xs leading-5 text-(--warning)">
                  <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
                  <span>{t("message.max_tokens_warning")}</span>
                </div>
              ) : null}

              {!assistant.direct.visible && !assistant.process.visible ? (
                <div className="pt-2">{pendingPermissionBlock}</div>
              ) : null}
            </div>

            {assistant.footer.visible ? (
              <MessageStats
                compact={compact}
                copiedAssistant={assistant.footer.copied}
                onCopyAssistant={assistant.footer.onCopy}
                showCursor={assistant.activity.showCursor}
                stats={assistant.footer.stats || undefined}
              />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

function EmptyStreamStatus({
  status,
}: {
  status: "cancelled" | "error" | null;
}) {
  const labels = {
    cancelled: <span className="text-xs italic text-(--text-soft)">已停止</span>,
    error: <span className="text-xs italic text-rose-500">执行失败</span>,
  };
  return status ? labels[status] : null;
}

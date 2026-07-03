"use client";

import { memo, useCallback, useMemo } from "react";
import { Bot, Loader2, Square } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  AssistantMessage,
  ResultSummary,
  RoomPendingAgentSlotState,
} from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";
import {
  AgentRoundStatus,
  extractAgentPreviewText,
} from "@/features/conversation/shared/utils";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-primitives";

interface GroupAgentStatusCardProps {
  agentId: string;
  agentName: string;
  agentAvatar?: string | null;
  messages: AssistantMessage[];
  resultSummary?: ResultSummary;
  pendingSlot?: RoomPendingAgentSlotState;
  status: AgentRoundStatus;
  pendingPermissions?: PendingPermission[];
  isThreadActive: boolean;
  onClickThread: () => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  onOpenAgentContact?: (agentId: string) => void;
  onStopMessage?: () => void;
}

/** 紧凑型 Agent 状态卡片 — 每个 Agent 在 Round 中的摘要 */
function GroupAgentStatusCardInner({
  agentId: agentId,
  agentName: agentName,
  agentAvatar: agentAvatar,
  messages,
  resultSummary: resultSummary,
  pendingSlot: pendingSlot,
  status,
  pendingPermissions: pendingPermissions = [],
  isThreadActive: isThreadActive,
  onClickThread: onClickThread,
  onPermissionResponse: onPermissionResponse,
  canRespondToPermissions: canRespondToPermissions = true,
  permissionReadOnlyReason: permissionReadOnlyReason,
  onOpenAgentContact: onOpenAgentContact,
  onStopMessage: onStopMessage,
}: GroupAgentStatusCardProps) {
  const preview = useMemo(() => extractAgentPreviewText(messages), [messages]);
  const primaryPendingPermission = pendingPermissions[0];
  const isQuestionPending = Boolean(
    primaryPendingPermission
    && (
      primaryPendingPermission.interaction_mode === "question"
      || primaryPendingPermission.tool_name === "AskUserQuestion"
    ),
  );
  const isWaitingPermission = pendingPermissions.length > 0 && (status === "pending" || status === "streaming");
  const lastMsg = messages[messages.length - 1];
  const canStop = onStopMessage && (status === "pending" || status === "streaming");
  const timestamp = lastMsg?.timestamp ?? resultSummary?.timestamp ?? pendingSlot?.timestamp ?? 0;
  const model = lastMsg?.model ?? null;
  const summaryText = useMemo(() => {
    const resultText = resultSummary?.result?.trim();
    if (isWaitingPermission) {
      return canRespondToPermissions
        ? (primaryPendingPermission?.summary || "等待权限确认")
        : (permissionReadOnlyReason || "当前暂不可确认权限");
    }
    if (status === "cancelled") {
      return resultText || "已停止";
    }
    if (status === "error") {
      return resultText || "执行失败";
    }
    if (preview) {
      return preview;
    }
    if (status === "pending") {
      return "正在准备回复...";
    }
    if (status === "streaming") {
      return "正在回复...";
    }
    return "";
  }, [canRespondToPermissions, isWaitingPermission, permissionReadOnlyReason, preview, primaryPendingPermission?.summary, resultSummary?.result, status]);
  const shouldRenderMarkdownSummary = Boolean(
    preview
    && !isWaitingPermission
    && status !== "cancelled"
    && status !== "error",
  );

  const handleStop = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (onStopMessage) {
        onStopMessage();
      }
    },
    [onStopMessage],
  );
  const handleAllow = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (isQuestionPending) {
      onClickThread();
      return;
    }
    if (!primaryPendingPermission || !onPermissionResponse) {
      onClickThread();
      return;
    }
    onPermissionResponse({
      request_id: primaryPendingPermission.request_id,
      decision: "allow",
    });
  }, [isQuestionPending, onClickThread, onPermissionResponse, primaryPendingPermission]);
  const handleDeny = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (!primaryPendingPermission || !onPermissionResponse) {
      onClickThread();
      return;
    }
    onPermissionResponse({
      request_id: primaryPendingPermission.request_id,
      decision: "deny",
    });
  }, [onClickThread, onPermissionResponse, primaryPendingPermission]);
  const handleToggleThread = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onClickThread();
  }, [onClickThread]);
  const handleOpenAgentContact = useCallback((e: React.MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    onOpenAgentContact?.(agentId);
  }, [agentId, onOpenAgentContact]);

  return (
    <div
      className={cn(
        "group/card grid min-w-0 grid-cols-[40px_minmax(0,1fr)] gap-3 px-2 py-3 transition-colors duration-(--motion-duration-normal) cursor-pointer",
        isThreadActive
          ? "bg-primary/5"
          : "hover:bg-(--interaction-hover-background)",
      )}
      onClick={onClickThread}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") onClickThread(); }}
    >
      <MessageAvatar
        ariaLabel={`打开 ${agentName} 的联络`}
        avatarUrl={agentAvatar}
        className="shrink-0"
        onClick={onOpenAgentContact ? handleOpenAgentContact : undefined}
        size="full"
        title={`打开 ${agentName} 的联络`}
      >
        {!agentAvatar && <Bot className="h-4 w-4" />}
      </MessageAvatar>

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-sm font-bold text-(--text-strong)">{agentName}</span>
          {(status === "pending" || status === "streaming") && !isWaitingPermission ? (
            <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />
          ) : null}
          <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
            {timestamp ? formatTime(timestamp) : "--:--"}
          </span>
          {model ? <span className="min-w-0 truncate text-xs text-(--text-soft)">{model}</span> : null}
          <div className="min-w-0 flex-1" />

          <button
            type="button"
            onClick={handleToggleThread}
            className={cn(
              "rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
              isThreadActive
                ? "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)"
                : "border-(--divider-subtle-color) bg-transparent text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-default)",
            )}
          >
            {isThreadActive ? "关闭 Thread" : "查看 Thread"}
          </button>

          {isWaitingPermission ? (
            <>
              <button
                type="button"
                onClick={handleDeny}
                disabled={!canRespondToPermissions}
                title={!canRespondToPermissions ? permissionReadOnlyReason : undefined}
                className={cn(
                  "rounded-md border border-(--divider-subtle-color) bg-transparent px-2 py-1 text-[11px] font-medium text-(--text-default) transition-colors",
                  canRespondToPermissions
                    ? "hover:bg-(--interaction-hover-background)"
                    : "cursor-not-allowed opacity-(--disabled-opacity)",
                )}
              >
                拒绝
              </button>
              <button
                type="button"
                onClick={handleAllow}
                disabled={!canRespondToPermissions}
                title={!canRespondToPermissions ? permissionReadOnlyReason : undefined}
                className={cn(
                  "rounded-md px-2 py-1 text-[11px] font-medium text-white transition-colors",
                  canRespondToPermissions
                    ? "bg-primary hover:bg-primary/88"
                    : "cursor-not-allowed bg-(--muted)",
                )}
              >
                {isQuestionPending ? "去回答" : "允许"}
              </button>
            </>
          ) : null}

          {canStop ? (
            <button
              type="button"
              onClick={handleStop}
              className="flex h-6 items-center gap-1 rounded px-1.5 text-xs text-(--icon-muted) transition-colors hover:bg-(--interaction-hover-background) hover:text-(--icon-default)"
            >
              <Square className="h-3 w-3 fill-current" />
            </button>
          ) : null}
        </div>

        <div className="min-w-0 pt-1">
          {shouldRenderMarkdownSummary ? (
            <MarkdownRendererContent
              content={preview}
              variant="summary"
              className="line-clamp-1 text-(--text-strong)"
              workspaceAgentId={agentId}
            />
          ) : (
            <p
              className={cn(
                "truncate text-[15px] leading-7",
                status === "error"
                  ? "text-(--destructive)"
                  : status === "cancelled"
                    ? "text-(--text-soft) italic"
                    : isWaitingPermission
                      ? "text-(--text-default)"
                      : "text-(--text-strong)",
              )}
            >
              {summaryText}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

export const GroupAgentStatusCard = memo(GroupAgentStatusCardInner);

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

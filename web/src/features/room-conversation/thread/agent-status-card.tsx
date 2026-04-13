"use client";

import { memo, useCallback, useMemo } from "react";
import { Bot, Loader2, Square } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  AssistantMessage,
  ResultMessage,
  RoomPendingAgentSlotState,
} from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import {
  AgentRoundStatus,
  extractAgentPreviewText,
} from "@/features/conversation-shared/utils";
import { MarkdownRendererContent } from "@/features/conversation-shared/message/markdown-renderer-content";

interface AgentStatusCardProps {
  agent_id: string;
  agent_name: string;
  messages: AssistantMessage[];
  result_message?: ResultMessage;
  pending_slot?: RoomPendingAgentSlotState;
  status: AgentRoundStatus;
  pending_permissions?: PendingPermission[];
  is_thread_active: boolean;
  on_click_thread: () => void;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_stop_message?: () => void;
}

/** 紧凑型 Agent 状态卡片 — 每个 Agent 在 Round 中的摘要 */
function AgentStatusCardInner({
  agent_name,
  messages,
  result_message,
  pending_slot,
  status,
  pending_permissions = [],
  is_thread_active,
  on_click_thread,
  on_permission_response,
  can_respond_to_permissions = true,
  permission_read_only_reason,
  on_stop_message,
}: AgentStatusCardProps) {
  const preview = useMemo(() => extractAgentPreviewText(messages), [messages]);
  const primary_pending_permission = pending_permissions[0];
  const is_question_pending = Boolean(
    primary_pending_permission
    && (
      primary_pending_permission.interaction_mode === "question"
      || primary_pending_permission.tool_name === "AskUserQuestion"
    ),
  );
  const is_waiting_permission = pending_permissions.length > 0 && (status === "pending" || status === "streaming");
  const last_msg = messages[messages.length - 1];
  const can_stop = on_stop_message && (status === "pending" || status === "streaming");
  const timestamp = last_msg?.timestamp ?? result_message?.timestamp ?? pending_slot?.timestamp ?? 0;
  const model = last_msg?.model ?? null;
  const summary_text = useMemo(() => {
    if (is_waiting_permission) {
      return can_respond_to_permissions
        ? (primary_pending_permission?.summary || "等待权限确认")
        : (permission_read_only_reason || "另一窗口正在处理权限确认");
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
    if (status === "cancelled") {
      return "已停止";
    }
    if (status === "error") {
      return "执行失败";
    }
    return "";
  }, [can_respond_to_permissions, is_waiting_permission, permission_read_only_reason, preview, primary_pending_permission?.summary, status]);
  const should_render_markdown_summary = Boolean(
    preview
    && !is_waiting_permission
    && status !== "cancelled"
    && status !== "error",
  );

  const handle_stop = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (on_stop_message) {
        on_stop_message();
      }
    },
    [on_stop_message],
  );
  const handle_allow = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (is_question_pending) {
      on_click_thread();
      return;
    }
    if (!primary_pending_permission || !on_permission_response) {
      on_click_thread();
      return;
    }
    on_permission_response({
      request_id: primary_pending_permission.request_id,
      decision: "allow",
    });
  }, [is_question_pending, on_click_thread, on_permission_response, primary_pending_permission]);
  const handle_deny = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (!primary_pending_permission || !on_permission_response) {
      on_click_thread();
      return;
    }
    on_permission_response({
      request_id: primary_pending_permission.request_id,
      decision: "deny",
    });
  }, [on_click_thread, on_permission_response, primary_pending_permission]);
  const handle_toggle_thread = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    on_click_thread();
  }, [on_click_thread]);

  return (
    <div
      className={cn(
        "group/card grid min-w-0 grid-cols-[40px_minmax(0,1fr)] gap-3 px-2 py-3 transition-colors duration-[var(--motion-duration-normal)] cursor-pointer",
        is_thread_active
          ? "bg-primary/5"
          : "hover:bg-[var(--interaction-hover-background)]",
      )}
      onClick={on_click_thread}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") on_click_thread(); }}
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-[var(--divider-subtle-color)] bg-[var(--surface-inset-background)] text-[color:var(--icon-default)]">
        <Bot className="h-4 w-4" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-sm font-bold text-[color:var(--text-strong)]">{agent_name}</span>
          {(status === "pending" || status === "streaming") && !is_waiting_permission ? (
            <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />
          ) : null}
          <span className="hidden shrink-0 text-xs text-[color:var(--text-muted)] sm:inline">
            {timestamp ? formatTime(timestamp) : "--:--"}
          </span>
          {model ? <span className="min-w-0 truncate text-xs text-[color:var(--text-soft)]">{model}</span> : null}
          <div className="min-w-0 flex-1" />

          <button
            type="button"
            onClick={handle_toggle_thread}
            className={cn(
              "rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
              is_thread_active
                ? "border-[var(--status-info-soft-border)] bg-[var(--status-info-soft-bg)] text-[color:var(--status-info-soft-text)]"
                : "border-[var(--divider-subtle-color)] bg-[var(--material-chip-background)] text-[color:var(--text-muted)] hover:bg-[var(--interaction-hover-background)] hover:text-[color:var(--text-default)]",
            )}
          >
            {is_thread_active ? "关闭 Thread" : "查看 Thread"}
          </button>

          {is_waiting_permission ? (
            <>
              <button
                type="button"
                onClick={handle_deny}
                disabled={!can_respond_to_permissions}
                title={!can_respond_to_permissions ? permission_read_only_reason : undefined}
                className={cn(
                  "rounded-md border border-[var(--divider-subtle-color)] bg-[var(--material-chip-background)] px-2 py-1 text-[11px] font-medium text-[color:var(--text-default)] transition-colors",
                  can_respond_to_permissions
                    ? "hover:bg-[var(--interaction-hover-background)]"
                    : "cursor-not-allowed opacity-[var(--disabled-opacity)]",
                )}
              >
                拒绝
              </button>
              <button
                type="button"
                onClick={handle_allow}
                disabled={!can_respond_to_permissions}
                title={!can_respond_to_permissions ? permission_read_only_reason : undefined}
                className={cn(
                  "rounded-md px-2 py-1 text-[11px] font-medium text-white transition-colors",
                  can_respond_to_permissions
                    ? "bg-primary hover:bg-primary/88"
                    : "cursor-not-allowed bg-[var(--muted)]",
                )}
              >
                {is_question_pending ? "去回答" : "允许"}
              </button>
            </>
          ) : null}

          {can_stop ? (
            <button
              type="button"
              onClick={handle_stop}
              className="flex h-6 items-center gap-1 rounded px-1.5 text-xs text-[color:var(--icon-muted)] transition-colors hover:bg-[var(--interaction-hover-background)] hover:text-[color:var(--icon-default)]"
            >
              <Square className="h-3 w-3 fill-current" />
            </button>
          ) : null}
        </div>

        <div className="min-w-0 pt-1">
          {should_render_markdown_summary ? (
            <MarkdownRendererContent
              content={preview}
              variant="summary"
              class_name="line-clamp-1 text-[color:var(--text-strong)]"
            />
          ) : (
            <p
              className={cn(
                "truncate text-[15px] leading-7",
                status === "error"
                  ? "text-[color:var(--destructive)]"
                  : status === "cancelled"
                    ? "text-[color:var(--text-soft)] italic"
                    : is_waiting_permission
                      ? "text-[color:var(--text-default)]"
                      : "text-[color:var(--text-strong)]",
              )}
            >
              {summary_text}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

export const AgentStatusCard = memo(AgentStatusCardInner);

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

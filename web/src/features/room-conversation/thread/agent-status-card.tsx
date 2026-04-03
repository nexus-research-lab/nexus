"use client";

import { memo, useCallback, useMemo } from "react";
import { Bot, Loader2, Square } from "lucide-react";
import { cn } from "@/lib/utils";
import { AssistantMessage, ResultMessage } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import {
  AgentRoundStatus,
  extractAgentPreviewText,
  getAgentRoundStatus,
} from "@/features/conversation-shared/utils";

interface AgentStatusCardProps {
  agent_id: string;
  agent_name: string;
  messages: AssistantMessage[];
  result_message?: ResultMessage;
  pending_permissions?: PendingPermission[];
  is_thread_active: boolean;
  on_click_thread: () => void;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  on_stop_message?: () => void;
}

/** 紧凑型 Agent 状态卡片 — 每个 Agent 在 Round 中的摘要 */
function AgentStatusCardInner({
  agent_name,
  messages,
  result_message,
  pending_permissions = [],
  is_thread_active,
  on_click_thread,
  on_permission_response,
  on_stop_message,
}: AgentStatusCardProps) {
  const status: AgentRoundStatus = getAgentRoundStatus(messages, result_message);
  const preview = useMemo(() => extractAgentPreviewText(messages), [messages]);
  const primary_pending_permission = pending_permissions[0];
  const is_waiting_permission = pending_permissions.length > 0 && (status === "pending" || status === "streaming");
  const first_msg = messages[0];
  const last_msg = messages[messages.length - 1];
  const can_stop = on_stop_message && (status === "pending" || status === "streaming");
  const timestamp = last_msg?.timestamp ?? result_message?.timestamp ?? 0;
  const model = last_msg?.model ?? null;
  const summary_text = useMemo(() => {
    if (is_waiting_permission) {
      return primary_pending_permission?.summary || "等待权限确认";
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
  }, [is_waiting_permission, preview, primary_pending_permission?.summary, status]);

  const handle_stop = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (first_msg && on_stop_message) {
        on_stop_message();
      }
    },
    [first_msg, on_stop_message],
  );
  const handle_allow = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (!primary_pending_permission || !on_permission_response) {
      on_click_thread();
      return;
    }
    on_permission_response({
      request_id: primary_pending_permission.request_id,
      decision: "allow",
    });
  }, [on_click_thread, on_permission_response, primary_pending_permission]);
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
        "group/card grid min-w-0 grid-cols-[40px_minmax(0,1fr)] gap-3 px-2 py-3 transition-colors duration-200 cursor-pointer",
        is_thread_active
          ? "bg-primary/5"
          : "hover:bg-slate-50/70",
      )}
      onClick={on_click_thread}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") on_click_thread(); }}
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 text-slate-600">
        <Bot className="h-4 w-4" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-sm font-bold text-slate-900">{agent_name}</span>
          {(status === "pending" || status === "streaming") && !is_waiting_permission ? (
            <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />
          ) : null}
          <span className="hidden shrink-0 text-xs text-slate-500 sm:inline">
            {timestamp ? formatTime(timestamp) : "--:--"}
          </span>
          {model ? <span className="min-w-0 truncate text-xs text-slate-400">{model}</span> : null}
          <div className="min-w-0 flex-1" />

          <button
            type="button"
            onClick={handle_toggle_thread}
            className={cn(
              "rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
              is_thread_active
                ? "border-[#cfe0ff] bg-[#eff6ff] text-[#27539d]"
                : "border-slate-200 bg-white text-slate-500 hover:bg-slate-50 hover:text-slate-700",
            )}
          >
            {is_thread_active ? "关闭 Thread" : "查看 Thread"}
          </button>

          {is_waiting_permission ? (
            <>
              <button
                type="button"
                onClick={handle_deny}
                className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[11px] font-medium text-slate-600 transition-colors hover:bg-slate-50"
              >
                拒绝
              </button>
              <button
                type="button"
                onClick={handle_allow}
                className="rounded-md bg-[#7c6cf2] px-2 py-1 text-[11px] font-medium text-white transition-colors hover:bg-[#6f5de8]"
              >
                允许
              </button>
            </>
          ) : null}

          {can_stop ? (
            <button
              type="button"
              onClick={handle_stop}
              className="flex h-6 items-center gap-1 rounded px-1.5 text-xs text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
            >
              <Square className="h-3 w-3 fill-current" />
            </button>
          ) : null}
        </div>

        <div className="min-w-0 pt-1">
          <p
            className={cn(
              "truncate text-[15px] leading-7",
              status === "error"
                ? "text-rose-500"
                : status === "cancelled"
                  ? "text-slate-400 italic"
                  : is_waiting_permission
                    ? "text-slate-700"
                    : "text-slate-900",
            )}
          >
            {summary_text}
          </p>
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

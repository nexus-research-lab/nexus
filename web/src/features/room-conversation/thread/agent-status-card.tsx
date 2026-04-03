"use client";

import { memo, useCallback, useMemo } from "react";
import { Bot, Check, Loader2, Square, X } from "lucide-react";
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

  // result 消息中的统计信息（由父组件从 round 消息中提取传入）
  const result_msg = useMemo(() => {
    if (result_message) {
      return {
        tokens: result_message.usage
          ? `↑${result_message.usage.input_tokens} ↓${result_message.usage.output_tokens}`
          : null,
      };
    }

    // 中文注释：pending 卡片没有 result 时，退回 assistant usage 作为临时摘要。
    const last = messages[messages.length - 1];
    if (!last) return null;
    return {
      tokens: last.usage
        ? `↑${last.usage.input_tokens} ↓${last.usage.output_tokens}`
        : null,
    };
  }, [messages, result_message]);

  const first_msg = messages[0];
  const can_stop = on_stop_message && (status === "pending" || status === "streaming");

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

  return (
    <div
      className={cn(
        "group/card flex items-center gap-2.5 rounded-xl border px-3 py-2.5 transition-all duration-200 cursor-pointer",
        is_thread_active
          ? "border-primary/30 bg-primary/5 shadow-sm"
          : "border-slate-200/80 bg-white hover:border-slate-300 hover:shadow-sm",
      )}
      onClick={on_click_thread}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") on_click_thread(); }}
    >
      {/* Agent 头像 */}
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 text-slate-500">
        <Bot className="h-3.5 w-3.5" />
      </div>

      {/* 主体内容 */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-slate-800">{agent_name}</span>
          <StatusBadge status={status} />
        </div>

        {/* 状态详情 */}
        {status === "pending" && (
          <div className="mt-0.5 flex items-center gap-1.5">
            {is_waiting_permission ? (
              <p className="truncate text-xs text-amber-600">等待权限确认</p>
            ) : (
              <>
                <span className="h-1 w-1 animate-bounce rounded-full bg-slate-400 [animation-delay:0ms]" />
                <span className="h-1 w-1 animate-bounce rounded-full bg-slate-400 [animation-delay:150ms]" />
                <span className="h-1 w-1 animate-bounce rounded-full bg-slate-400 [animation-delay:300ms]" />
              </>
            )}
          </div>
        )}

        {status === "streaming" && (
          <p className={cn(
            "mt-0.5 truncate text-xs",
            is_waiting_permission ? "text-amber-600" : "text-slate-400",
          )}>
            {is_waiting_permission ? "等待权限确认" : "正在回复..."}
          </p>
        )}

        {status === "done" && preview && (
          <p className="mt-0.5 truncate text-xs text-slate-500">{preview}</p>
        )}

        {status === "cancelled" && (
          <p className="mt-0.5 text-xs text-slate-400 italic">已停止</p>
        )}

        {status === "error" && (
          <p className="mt-0.5 text-xs text-rose-500 italic">执行失败</p>
        )}

        {is_waiting_permission && primary_pending_permission?.summary ? (
          <p className="mt-0.5 truncate text-xs text-slate-500">{primary_pending_permission.summary}</p>
        ) : null}
      </div>

      {/* 右侧操作区 */}
      <div className="flex shrink-0 items-center gap-1">
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

        {/* Token 统计 (done 状态) */}
        {status === "done" && result_msg?.tokens && (
          <span className="hidden text-[10px] tabular-nums text-slate-400 sm:inline">
            {result_msg.tokens}
          </span>
        )}

        {/* 停止按钮 */}
        {can_stop && (
          <button
            type="button"
            onClick={handle_stop}
            className="flex h-6 items-center gap-1 rounded-md px-1.5 text-xs text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
          >
            <Square className="h-3 w-3 fill-current" />
          </button>
        )}

      </div>
    </div>
  );
}

/** 状态标记 */
function StatusBadge({ status }: { status: AgentRoundStatus }) {
  switch (status) {
    case "streaming":
      return <Loader2 className="h-3 w-3 animate-spin text-primary" />;
    case "done":
      return <Check className="h-3 w-3 text-emerald-500" />;
    case "error":
      return <X className="h-3 w-3 text-rose-500" />;
    case "cancelled":
      return <Square className="h-3 w-3 text-slate-400" />;
    default:
      return null;
  }
}

export const AgentStatusCard = memo(AgentStatusCardInner);

"use client";

import { CircleAlert } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import { MessageAvatar } from "./message/ui/message-avatar";

interface ConversationErrorBubbleProps {
  error: string;
  compact?: boolean;
}

interface ErrorPresentation {
  title: string;
  detail: string;
}

function withRetryDetail(error: string): string {
  const normalizedError = error.trim().replace(/[。.!！?？；;，,\s]+$/u, "");
  const message = normalizedError.length > 0 ? normalizedError : "请求失败";
  return `${message}。请稍后重试；如果当前轮次没有继续响应，可以刷新页面后重新发送上一条消息。`;
}

function resolveErrorPresentation(error: string): ErrorPresentation {
  const normalizedError = error.toLowerCase();

  if (
    normalizedError.includes("provider_error=server_overload") ||
    normalizedError.includes("provider_error=rate_limit") ||
    normalizedError.includes("overloaded_error") ||
    normalizedError.includes("rate_limit_error") ||
    normalizedError.includes("repeated 529") ||
    normalizedError.includes(" 529 ") ||
    normalizedError.includes(" 429 ") ||
    error.includes("模型请求暂时受限")
  ) {
    return {
      title: "模型请求暂时受限",
      detail: "当前 LLM Provider 返回限流或过载。请稍后重试；如果持续失败，临时切换到可用 Provider 或模型。",
    };
  }

  if (
    normalizedError.includes("websocket") ||
    error.includes("WebSocket未连接") ||
    error.includes("连接")
  ) {
    return {
      title: "连接中断",
      detail: "浏览器与 Nexus 的实时通道暂时没有响应，系统会自动尝试重连。可能是网络、后端负载或运行时模型服务阻塞；如果刚才的消息没有继续处理，可以刷新页面后重新发送。",
    };
  }

  if (error.includes("服务器") || error.includes("后端")) {
    return {
      title: "后端响应异常",
      detail: "Nexus 后端已返回异常或长时间没有响应。请稍后重试；如果健康检查正常，优先检查当前会话的运行时和模型 provider 状态。",
    };
  }

  return {
    title: "系统消息",
    detail: withRetryDetail(error),
  };
}

export function ConversationErrorBubble({
  error,
  compact = false,
}: ConversationErrorBubbleProps) {
  const presentation = resolveErrorPresentation(error);

  return (
    <div className={cn("w-full", compact ? "px-0" : "px-2 sm:px-3")}>
      <div className={cn("w-full", compact ? "max-w-full" : "max-w-[980px]")}>
        <div className={cn(
          "group grid min-w-0",
          compact ? "grid-cols-[minmax(0,1fr)]" : "grid-cols-[40px_minmax(0,1fr)] gap-3",
        )}>
          {!compact ? (
            <MessageAvatar>
              <CircleAlert className="h-4 w-4 text-destructive" />
            </MessageAvatar>
          ) : null}

          <div className="relative min-w-0">
            <div className={cn(
              "flex min-w-0 items-center gap-2",
              compact ? "min-h-6 pb-0" : "h-7 pb-0.5",
            )}>
              {compact ? (
                <MessageAvatar className="shrink-0" size="compact">
                  <CircleAlert className="h-3 w-3 text-destructive" />
                </MessageAvatar>
              ) : null}
              <span className="shrink-0 text-sm font-bold text-(--text-strong)">
                系统消息
              </span>
            </div>

            <div className={cn(
              "min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
              compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
            )}>
              <p className="font-medium text-destructive">{presentation.title}</p>
              <p className="mt-1 text-sm text-(--text-muted)">{presentation.detail}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

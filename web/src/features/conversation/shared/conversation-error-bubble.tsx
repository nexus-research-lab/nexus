"use client";

import { CircleAlert } from "lucide-react";

import { cn } from "@/lib/utils";

import { MessageAvatar } from "./message/ui/message-primitives";

interface ConversationErrorBubbleProps {
  error: string;
  compact?: boolean;
}

interface ErrorPresentation {
  title: string;
  detail: string;
}

function resolve_error_presentation(error: string): ErrorPresentation {
  const normalized_error = error.toLowerCase();

  if (
    normalized_error.includes("websocket") ||
    error.includes("WebSocket未连接") ||
    error.includes("连接")
  ) {
    return {
      title: "连接中断",
      detail: "浏览器与 Nexus 后端的实时连接异常，系统会自动尝试重连。请先确认网络和后端服务状态；连接恢复后，如果刚才的消息没有继续处理，可以刷新页面后重新发送。",
    };
  }

  if (error.includes("服务器") || error.includes("后端")) {
    return {
      title: "无法连接到后端服务",
      detail: "请确认后端服务正在运行，并检查本地开发端口或部署代理配置。服务恢复后，可以刷新页面重新进入当前对话。",
    };
  }

  return {
    title: "系统消息",
    detail: `${error}。请稍后重试；如果当前轮次没有继续响应，可以刷新页面后重新发送上一条消息。`,
  };
}

export function ConversationErrorBubble({
  error,
  compact = false,
}: ConversationErrorBubbleProps) {
  const presentation = resolve_error_presentation(error);

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
                <MessageAvatar class_name="shrink-0" size="compact">
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

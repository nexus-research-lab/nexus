"use client";

import { Bot } from "lucide-react";

import { cn } from "@/lib/utils";

import { MessageAvatar } from "./message/ui/message-primitives";

interface ConversationErrorBubbleProps {
  error: string;
  compact?: boolean;
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
}

interface ErrorPresentation {
  title: string;
  detail: string;
}

export function is_provider_error(error: string): boolean {
  return error.toLowerCase().includes("provider");
}

function resolve_error_presentation(error: string): ErrorPresentation {
  if (error.includes("服务器")) {
    return {
      title: "无法连接到后端服务",
      detail: "请确保后端服务正在运行（端口 8010）。",
    };
  }
  return {
    title: "对话出错",
    detail: error,
  };
}

export function ConversationErrorBubble({
  error,
  compact = false,
  current_agent_name,
  current_agent_avatar,
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
            <MessageAvatar avatar_url={current_agent_avatar}>
              {!current_agent_avatar && <Bot className="h-4 w-4" />}
            </MessageAvatar>
          ) : null}

          <div className="relative min-w-0">
            <div className={cn(
              "flex min-w-0 items-center gap-2",
              compact ? "min-h-6 pb-0" : "h-7 pb-0.5",
            )}>
              {compact ? (
                <MessageAvatar class_name="shrink-0" size="compact" avatar_url={current_agent_avatar}>
                  {!current_agent_avatar && <Bot className="h-3 w-3" />}
                </MessageAvatar>
              ) : null}
              <span className="shrink-0 text-sm font-bold text-(--text-strong)">
                {current_agent_name || "协作成员"}
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

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

interface ErrorPresentationRule extends ErrorPresentation {
  markers: readonly string[];
}

// 规则顺序表达诊断优先级，具体 Provider 原因不能被通用连接错误覆盖。
const ERROR_PRESENTATION_RULES: readonly ErrorPresentationRule[] = [
  {
    detail:
      "当前账号本月的订阅额度已全部用尽，暂时无法发起新的 Agent 请求。这是账号级月度额度，不是单条回复的输出长度限制。请升级套餐，或等待下个计费周期重置后再继续使用。",
    markers: [
      "当前账号本月的订阅额度已全部用尽",
      "账号本月额度已耗尽",
      "账号本月额度已用完",
      "本月订阅套餐 token 额度已用完",
      "subscription token quota exceeded",
    ],
    title: "账号本月额度已耗尽",
  },
  {
    detail:
      "当前 LLM Provider 返回限流或过载。请稍后重试；如果持续失败，临时切换到可用 Provider 或模型。",
    markers: [
      "provider_error=server_overload",
      "provider_error=rate_limit",
      "overloaded_error",
      "rate_limit_error",
      "repeated 529",
      " 529 ",
      " 429 ",
      "模型请求暂时受限",
    ],
    title: "模型请求暂时受限",
  },
  {
    detail:
      "浏览器与 Nexus 的实时通道暂时没有响应，系统会自动尝试重连。可能是网络、后端负载或运行时模型服务阻塞；如果刚才的消息没有继续处理，可以刷新页面后重新发送。",
    markers: [
      "websocket",
      "connection closed",
      "connection lost",
      "network error",
      "连接中断",
      "未连接",
    ],
    title: "连接中断",
  },
  {
    detail:
      "Nexus 后端已返回异常或长时间没有响应。请稍后重试；如果健康检查正常，优先检查当前会话的运行时和模型 provider 状态。",
    markers: ["服务器", "后端", "服务内部错误"],
    title: "后端响应异常",
  },
];

function withRetryDetail(error: string): string {
  const normalizedError = error.trim().replace(/[。.!！?？；;，,\s]+$/u, "");
  const message = normalizedError.length > 0 ? normalizedError : "请求失败";
  return `${message}。请稍后重试；如果当前轮次没有继续响应，可以刷新页面后重新发送上一条消息。`;
}

function resolveErrorPresentation(error: string): ErrorPresentation {
  const normalizedError = error.toLowerCase();
  const matchedRule = ERROR_PRESENTATION_RULES.find((rule) =>
    rule.markers.some((marker) => normalizedError.includes(marker)),
  );
  return matchedRule ?? {
    detail: withRetryDetail(error),
    title: "系统消息",
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

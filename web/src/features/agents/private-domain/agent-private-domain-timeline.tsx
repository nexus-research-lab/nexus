import {
  Inbox,
  Loader2,
  MessageCircle,
} from "lucide-react";

import { PrivateParticipantAvatar } from "@/features/agents/private-domain/agent-private-domain-avatar";
import {
  event_route_label,
  private_thread_title,
} from "@/features/agents/private-domain/agent-private-domain-model";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import {
  cn,
  format_relative_time,
} from "@/lib/utils";
import {
  AgentPrivateEvent,
  AgentPrivateThread,
} from "@/types/agent/private-domain";

export function PrivateEventTimeline({
  agent_id,
  class_name,
  compact = false,
  error,
  events,
  is_loading,
  thread,
}: {
  agent_id: string;
  class_name?: string;
  compact?: boolean;
  error: string | null;
  events: AgentPrivateEvent[];
  is_loading: boolean;
  thread: AgentPrivateThread | null;
}) {
  return (
    <section
      className={cn(
        "flex min-h-0 flex-col overflow-hidden border border-(--divider-subtle-color)",
        compact
          ? "rounded-[14px] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_30%,transparent)]"
          : "rounded-[16px] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_42%,transparent)]",
        class_name,
      )}
    >
      <div className={cn("flex items-center justify-between gap-3 border-b border-(--divider-subtle-color)", compact ? "h-10 px-3" : "h-11 px-4")}>
        <div className="min-w-0">
          <p className={cn("truncate font-bold text-(--text-strong)", compact ? "text-[12.5px]" : "text-[13px]")}>
            {thread ? private_thread_title(thread, agent_id) : "联络消息"}
          </p>
          {thread ? (
            <p className={cn("mt-0.5 truncate font-semibold text-(--text-soft)", compact ? "text-[10px]" : "text-[10.5px]")}>
              {thread.room_name || "房间"} · {thread.conversation_title || "主对话"}
            </p>
          ) : null}
        </div>
        {is_loading ? <Loader2 className="h-4 w-4 animate-spin text-(--text-soft)" /> : null}
      </div>

      <div className={cn("soft-scrollbar min-h-0 flex-1 overflow-y-auto", compact ? "px-3 py-3" : "px-4 py-4")}>
        {error ? (
          <p className="rounded-[14px] border border-[color:color-mix(in_srgb,var(--destructive)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-3 py-2 text-[12px] font-semibold text-(--destructive)">
            {error}
          </p>
        ) : null}
        {!error && !thread ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-(--text-soft)">
            <MessageCircle className="h-6 w-6" />
            <span className="text-[12px] font-semibold">选择一条联络记录</span>
          </div>
        ) : null}
        {!error && thread && events.length === 0 && !is_loading ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-(--text-soft)">
            <Inbox className="h-6 w-6" />
            <span className="text-[12px] font-semibold">暂无消息</span>
          </div>
        ) : null}
        <div className="space-y-3">
          {events.map((event) => (
            <PrivateEventBubble
              agent_id={agent_id}
              compact={compact}
              event={event}
              key={event.message_id}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

function PrivateEventBubble({
  agent_id,
  compact = false,
  event,
}: {
  agent_id: string;
  compact?: boolean;
  event: AgentPrivateEvent;
}) {
  const is_outgoing = event.direction === "outgoing";
  const is_self = event.direction === "self";
  const source = event.participants.find((participant) => participant.agent_id === event.source_agent_id);
  return (
    <div className={cn("flex", is_self ? "justify-center" : is_outgoing ? "justify-end" : "justify-start")}>
      <div
        className={cn(
          "w-fit border",
          compact
            ? "max-w-[88%] rounded-[13px] px-2.5 py-2 shadow-none"
            : "max-w-[min(720px,78%)] rounded-[16px] px-3 py-2.5 shadow-[0_8px_24px_rgba(15,23,42,0.04)]",
          is_self
            ? "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_72%,transparent)]"
            : is_outgoing
              ? "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]"
              : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_62%,transparent)]",
        )}
      >
        <div className={cn("flex min-w-0 items-center", compact ? "gap-1.5" : "gap-2")}>
          <PrivateParticipantAvatar participant={source} size="sm" />
          <span className={cn("truncate font-bold text-(--text-strong)", compact ? "text-[11.5px]" : "text-[12px]")}>
            {source?.agent_id === agent_id ? "我" : source?.name || event.source_agent_id}
          </span>
          <span className="rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_68%,transparent)] px-1.5 py-0.5 text-[10px] font-semibold text-(--text-soft)">
            私信
          </span>
          <span className="ml-auto shrink-0 text-[10.5px] font-semibold text-(--text-soft)">
            {format_relative_time(event.timestamp)}
          </span>
        </div>
        <MarkdownRendererContent
          class_name={cn(
            "text-(--text-default) [&_[data-markdown-anchor]]:my-1 [&_[data-markdown-anchor]]:leading-5 [&_blockquote]:my-2 [&_ol]:mb-2 [&_ol]:space-y-1 [&_ul]:mb-2 [&_ul]:space-y-1",
            compact ? "mt-1.5 text-[12.5px] leading-5" : "mt-2 text-[13px] leading-5",
          )}
          content={event.content || "（无正文）"}
          mermaid_show_header={false}
          workspace_agent_id={event.source_agent_id}
        />
        <p className={cn("truncate font-semibold text-(--text-soft)", compact ? "mt-1.5 text-[10px]" : "mt-2 text-[10.5px]")}>
          {event_route_label(event, agent_id)}
        </p>
      </div>
    </div>
  );
}

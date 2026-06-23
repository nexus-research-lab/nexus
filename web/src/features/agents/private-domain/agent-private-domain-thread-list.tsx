import {
  Inbox,
  Loader2,
  MessageCircle,
  StickyNote,
  UsersRound,
} from "lucide-react";

import { PrivateParticipantAvatarStack } from "@/features/agents/private-domain/agent-private-domain-avatar";
import { private_thread_title } from "@/features/agents/private-domain/agent-private-domain-model";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import {
  cn,
  format_relative_time,
} from "@/lib/utils";
import { AgentPrivateThread } from "@/types/agent/private-domain";

export function PrivateThreadList({
  agent_id,
  class_name,
  compact = false,
  is_loading,
  on_select,
  selected_thread_id,
  threads,
}: {
  agent_id: string;
  class_name?: string;
  compact?: boolean;
  is_loading: boolean;
  on_select: (thread_id: string) => void;
  selected_thread_id: string | null;
  threads: AgentPrivateThread[];
}) {
  if (is_loading && threads.length === 0) {
    return (
      <div className={cn("flex items-center justify-center", class_name)}>
        <Loader2 className="h-5 w-5 animate-spin text-(--text-soft)" />
      </div>
    );
  }

  if (threads.length === 0) {
    return (
      <div className={cn("flex flex-col items-center justify-center gap-2 px-4 text-center", class_name)}>
        <Inbox className="h-5 w-5 text-(--text-soft)" />
        <p className="text-[12px] font-semibold text-(--text-muted)">暂无联络记录</p>
      </div>
    );
  }

  return (
    <div className={cn("soft-scrollbar min-h-0 overflow-y-auto", compact ? "p-1.5" : "p-2", class_name)}>
      <div className={compact ? "space-y-0.5" : "space-y-1"}>
        {threads.map((thread) => {
          const is_active = thread.thread_id === selected_thread_id;
          return (
            <button
              className={cn(
                "group flex w-full min-w-0 items-start border text-left transition",
                compact ? "gap-2 rounded-[10px] px-2 py-2" : "gap-2.5 rounded-[12px] px-2.5 py-2.5",
                is_active
                  ? compact
                    ? "border-transparent bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] shadow-[inset_2px_0_0_var(--primary)]"
                    : "border-[color:color-mix(in_srgb,var(--primary)_38%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]"
                  : "border-transparent hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background)",
              )}
              key={thread.thread_id}
              onClick={() => on_select(thread.thread_id)}
              type="button"
            >
              <PrivateParticipantAvatarStack
                owner_agent_id={agent_id}
                participants={thread.participants}
              />
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-1.5">
                  <span className={cn("truncate font-bold text-(--text-strong)", compact ? "text-[12.5px]" : "text-[13px]")}>
                    {private_thread_title(thread, agent_id)}
                  </span>
                  <ThreadScopeIcon scope={thread.scope} />
                </div>
                <MarkdownRendererContent
                  class_name={cn(
                    "mt-1 text-(--text-muted) [&_*]:leading-4",
                    compact ? "line-clamp-1 text-[11.5px] leading-4" : "line-clamp-2 text-[12px] leading-4",
                  )}
                  content={thread.last_content_preview || "联络消息"}
                  mermaid_show_header={false}
                  variant="summary"
                  workspace_agent_id={thread.participant_agent_ids[0] ?? agent_id}
                />
                <div className={cn("flex items-center gap-1.5 font-semibold text-(--text-soft)", compact ? "mt-1 text-[10px]" : "mt-1.5 text-[10.5px]")}>
                  <span className="truncate">{thread.room_name || "房间"}</span>
                  <span>·</span>
                  <span>{thread.message_count}</span>
                  {thread.last_timestamp ? (
                    <>
                      <span>·</span>
                      <span>{format_relative_time(thread.last_timestamp)}</span>
                    </>
                  ) : null}
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function ThreadScopeIcon({ scope }: { scope: string }) {
  const Icon = scope === "audience" ? UsersRound : scope === "self" ? StickyNote : MessageCircle;
  return <Icon className="h-3.5 w-3.5 shrink-0 text-(--text-soft)" />;
}

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Handshake,
  Inbox,
  Loader2,
  MessageCircle,
  RefreshCw,
  StickyNote,
  UsersRound,
} from "lucide-react";

import {
  AgentPrivateDomainQuery,
  list_agent_private_events_api,
  list_agent_private_threads_api,
} from "@/lib/api/agent-private-domain-api";
import {
  cn,
  format_relative_time,
  get_icon_avatar_src,
  get_initials,
} from "@/lib/utils";
import { Agent } from "@/types/agent/agent";
import {
  AgentPrivateEvent,
  AgentPrivateParticipant,
  AgentPrivateThread,
} from "@/types/agent/private-domain";

interface AgentPrivateDomainViewProps {
  agent: Agent;
  room_id?: string | null;
  conversation_id?: string | null;
  variant?: "full" | "preview";
}

export function AgentPrivateDomainView({
  agent,
  room_id = null,
  conversation_id = null,
  variant = "full",
}: AgentPrivateDomainViewProps) {
  const [threads, set_threads] = useState<AgentPrivateThread[]>([]);
  const [selected_thread_id, set_selected_thread_id] = useState<string | null>(null);
  const [events, set_events] = useState<AgentPrivateEvent[]>([]);
  const [threads_loading, set_threads_loading] = useState(false);
  const [events_loading, set_events_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const is_preview = variant === "preview";

  const query = useMemo<AgentPrivateDomainQuery>(() => ({
    room_id,
    conversation_id,
    limit: is_preview ? 16 : 80,
    room_limit: is_preview ? 1 : 160,
  }), [conversation_id, is_preview, room_id]);

  const load_threads = useCallback(async () => {
    set_threads_loading(true);
    set_error(null);
    try {
      const page = await list_agent_private_threads_api(agent.agent_id, query);
      const next_threads = page.items ?? [];
      set_threads(next_threads);
      set_selected_thread_id((current) => {
        if (current && next_threads.some((thread) => thread.thread_id === current)) {
          return current;
        }
        return next_threads[0]?.thread_id ?? null;
      });
    } catch (load_error) {
      set_error(load_error instanceof Error ? load_error.message : "加载联络记录失败");
      set_threads([]);
      set_selected_thread_id(null);
    } finally {
      set_threads_loading(false);
    }
  }, [agent.agent_id, query]);

  const load_events = useCallback(async (thread_id: string | null) => {
    if (!thread_id) {
      set_events([]);
      return;
    }
    set_events_loading(true);
    set_error(null);
    try {
      const page = await list_agent_private_events_api(agent.agent_id, thread_id, {
        ...query,
        limit: is_preview ? 40 : 120,
      });
      set_events(page.items ?? []);
    } catch (load_error) {
      set_error(load_error instanceof Error ? load_error.message : "加载联络消息失败");
      set_events([]);
    } finally {
      set_events_loading(false);
    }
  }, [agent.agent_id, is_preview, query]);

  useEffect(() => {
    let cancelled = false;
    set_threads_loading(true);
    set_error(null);
    void list_agent_private_threads_api(agent.agent_id, query)
      .then((page) => {
        if (cancelled) return;
        const next_threads = page.items ?? [];
        set_threads(next_threads);
        set_selected_thread_id(next_threads[0]?.thread_id ?? null);
      })
      .catch((load_error) => {
        if (cancelled) return;
        set_error(load_error instanceof Error ? load_error.message : "加载联络记录失败");
        set_threads([]);
        set_selected_thread_id(null);
      })
      .finally(() => {
        if (!cancelled) {
          set_threads_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agent.agent_id, query]);

  useEffect(() => {
    let cancelled = false;
    if (!selected_thread_id) {
      set_events([]);
      return () => {
        cancelled = true;
      };
    }
    set_events_loading(true);
    set_error(null);
    void list_agent_private_events_api(agent.agent_id, selected_thread_id, {
      ...query,
      limit: is_preview ? 40 : 120,
    })
      .then((page) => {
        if (!cancelled) {
          set_events(page.items ?? []);
        }
      })
      .catch((load_error) => {
        if (!cancelled) {
          set_error(load_error instanceof Error ? load_error.message : "加载联络消息失败");
          set_events([]);
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_events_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agent.agent_id, is_preview, query, selected_thread_id]);

  const selected_thread = useMemo(
    () => threads.find((thread) => thread.thread_id === selected_thread_id) ?? null,
    [selected_thread_id, threads],
  );

  const handle_refresh = useCallback(() => {
    void load_threads();
    void load_events(selected_thread_id);
  }, [load_events, load_threads, selected_thread_id]);

  if (is_preview) {
    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PrivateDomainToolbar
          count={threads.length}
          is_loading={threads_loading || events_loading}
          on_refresh={handle_refresh}
          title="联络"
        />
        <div className="grid min-h-0 flex-1 grid-rows-[minmax(132px,0.38fr)_minmax(0,1fr)] gap-3 px-4 pb-4 pt-3">
          <PrivateThreadList
            agent_id={agent.agent_id}
            class_name="min-h-0"
            is_loading={threads_loading}
            on_select={set_selected_thread_id}
            selected_thread_id={selected_thread_id}
            threads={threads}
          />
          <PrivateEventTimeline
            agent_id={agent.agent_id}
            error={error}
            events={events}
            is_loading={events_loading}
            show_thread_meta={false}
            thread={selected_thread}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-hidden px-5 py-5 xl:px-6">
      <div className="grid h-full min-h-0 w-full grid-cols-[280px_minmax(320px,1fr)] gap-3 xl:grid-cols-[300px_minmax(420px,1fr)]">
        <section className="flex min-h-0 flex-col overflow-hidden rounded-[16px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_54%,transparent)]">
          <PrivateDomainToolbar
            count={threads.length}
            is_loading={threads_loading}
            on_refresh={handle_refresh}
            title="联络"
          />
          <PrivateThreadList
            agent_id={agent.agent_id}
            class_name="min-h-0 flex-1"
            is_loading={threads_loading}
            on_select={set_selected_thread_id}
            selected_thread_id={selected_thread_id}
            threads={threads}
          />
        </section>

        <PrivateEventTimeline
          agent_id={agent.agent_id}
          error={error}
          events={events}
          is_loading={events_loading}
          show_thread_meta
          thread={selected_thread}
        />
      </div>
    </div>
  );
}

function PrivateDomainToolbar({
  count,
  is_loading,
  on_refresh,
  title,
}: {
  count: number;
  is_loading: boolean;
  on_refresh: () => void;
  title: string;
}) {
  return (
    <div className="flex h-11 items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-3.5">
      <div className="flex min-w-0 items-center gap-2">
        <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
          <Handshake className="h-3.5 w-3.5" />
        </span>
        <span className="truncate text-[13px] font-bold text-(--text-strong)">{title}</span>
        <span className="text-[11px] font-semibold text-(--text-soft)">{count}</span>
      </div>
      <button
        aria-label="刷新联络"
        className="flex h-7 w-7 items-center justify-center rounded-full text-(--icon-default) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={on_refresh}
        type="button"
      >
        <RefreshCw className={cn("h-3.5 w-3.5", is_loading && "animate-spin")} />
      </button>
    </div>
  );
}

function PrivateThreadList({
  agent_id,
  class_name,
  is_loading,
  on_select,
  selected_thread_id,
  threads,
}: {
  agent_id: string;
  class_name?: string;
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
    <div className={cn("soft-scrollbar min-h-0 overflow-y-auto p-2", class_name)}>
      <div className="space-y-1">
        {threads.map((thread) => {
          const is_active = thread.thread_id === selected_thread_id;
          return (
            <button
              className={cn(
                "group flex w-full min-w-0 items-start gap-2.5 rounded-[12px] border px-2.5 py-2.5 text-left transition",
                is_active
                  ? "border-[color:color-mix(in_srgb,var(--primary)_38%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]"
                  : "border-transparent hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background)",
              )}
              key={thread.thread_id}
              onClick={() => on_select(thread.thread_id)}
              type="button"
            >
              <ParticipantAvatarStack
                owner_agent_id={agent_id}
                participants={thread.participants}
              />
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-1.5">
                  <span className="truncate text-[13px] font-bold text-(--text-strong)">
                    {private_thread_title(thread, agent_id)}
                  </span>
                  <ThreadScopeIcon scope={thread.scope} />
                </div>
                <p className="mt-1 line-clamp-2 text-[12px] leading-4 text-(--text-muted)">
                  {thread.last_content_preview || action_type_label(thread.last_action_type)}
                </p>
                <div className="mt-1.5 flex items-center gap-1.5 text-[10.5px] font-semibold text-(--text-soft)">
                  <span className="truncate">{thread.room_name || "房间"}</span>
                  <span>·</span>
                  <span>{thread.action_count}</span>
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

function PrivateEventTimeline({
  agent_id,
  error,
  events,
  is_loading,
  show_thread_meta = false,
  thread,
}: {
  agent_id: string;
  error: string | null;
  events: AgentPrivateEvent[];
  is_loading: boolean;
  show_thread_meta?: boolean;
  thread: AgentPrivateThread | null;
}) {
  return (
    <section className="flex min-h-0 flex-col overflow-hidden rounded-[16px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_42%,transparent)]">
      <div className="border-b border-(--divider-subtle-color) px-4 py-3">
        <div className="flex min-w-0 items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="truncate text-[13px] font-bold text-(--text-strong)">
              {thread ? private_thread_title(thread, agent_id) : "联络消息"}
            </p>
            {thread ? (
              <p className="mt-0.5 truncate text-[10.5px] font-semibold text-(--text-soft)">
                {thread.room_name || "房间"} · {thread.conversation_title || "主对话"}
              </p>
            ) : null}
          </div>
          {is_loading ? <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-(--text-soft)" /> : null}
        </div>
        {show_thread_meta && thread ? (
          <PrivateThreadMetaBar
            agent_id={agent_id}
            thread={thread}
          />
        ) : null}
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-4 py-4">
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
              event={event}
              key={event.action_id}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

function PrivateEventBubble({
  agent_id,
  event,
}: {
  agent_id: string;
  event: AgentPrivateEvent;
}) {
  const is_outgoing = event.direction === "outgoing";
  const is_self = event.direction === "self";
  const source = event.participants.find((participant) => participant.agent_id === event.source_agent_id);
  return (
    <div className={cn("flex", is_self ? "justify-center" : is_outgoing ? "justify-end" : "justify-start")}>
      <div
        className={cn(
          "w-fit max-w-[min(720px,78%)] rounded-[16px] border px-3 py-2.5 shadow-[0_8px_24px_rgba(15,23,42,0.04)]",
          is_self
            ? "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_72%,transparent)]"
            : is_outgoing
              ? "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]"
              : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_62%,transparent)]",
        )}
      >
        <div className="flex min-w-0 items-center gap-2">
          <AgentAvatar participant={source} size="sm" />
          <span className="truncate text-[12px] font-bold text-(--text-strong)">
            {source?.agent_id === agent_id ? "我" : source?.name || event.source_agent_id}
          </span>
          <span className="rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-semibold text-(--text-soft)">
            {action_type_label(event.action_type)}
          </span>
          <span className="ml-auto shrink-0 text-[10.5px] font-semibold text-(--text-soft)">
            {format_relative_time(event.timestamp)}
          </span>
        </div>
        <p className="mt-2 whitespace-pre-wrap break-words text-[13px] leading-5 text-(--text-default)">
          {event.content || "（无正文）"}
        </p>
        <p className="mt-2 truncate text-[10.5px] font-semibold text-(--text-soft)">
          {event_route_label(event, agent_id)}
        </p>
      </div>
    </div>
  );
}

function PrivateThreadMetaBar({
  agent_id,
  thread,
}: {
  agent_id: string;
  thread: AgentPrivateThread;
}) {
  const peers = thread.participants.filter((participant) => participant.agent_id !== agent_id);
  const visible_participants = peers.length ? peers : thread.participants;
  const member_label = visible_participants
    .map((participant) => participant.agent_id === agent_id ? "我" : participant.name || participant.agent_id)
    .join("、");
  return (
    <div className="mt-2 flex min-w-0 flex-wrap items-center gap-1.5">
      <span className="rounded-full bg-(--surface-muted-background) px-2 py-0.5 text-[10.5px] font-semibold text-(--text-soft)">
        {scope_label(thread.scope)}
      </span>
      <span className="rounded-full bg-(--surface-muted-background) px-2 py-0.5 text-[10.5px] font-semibold text-(--text-soft)">
        {thread.action_count} 条
      </span>
      <span
        className="max-w-full truncate rounded-full bg-(--surface-muted-background) px-2 py-0.5 text-[10.5px] font-semibold text-(--text-soft)"
        title={member_label}
      >
        {member_label}
      </span>
    </div>
  );
}

function ParticipantAvatarStack({
  owner_agent_id,
  participants,
}: {
  owner_agent_id: string;
  participants: AgentPrivateParticipant[];
}) {
  const peers = participants.filter((participant) => participant.agent_id !== owner_agent_id);
  const stack_participants = peers.length ? peers : participants;
  const is_group = stack_participants.length > 1;
  const visible = stack_participants.slice(0, is_group ? 2 : 1);
  const overflow_count = Math.max(stack_participants.length - visible.length, 0);
  return (
    <div className="relative flex h-9 w-10 shrink-0 items-center justify-start">
      {visible.map((participant, index) => (
        <span
          className={cn(index > 0 && "-ml-2")}
          key={participant.agent_id}
          style={{ zIndex: 10 - index }}
        >
          <AgentAvatar participant={participant} size={is_group ? "stack" : "md"} />
        </span>
      ))}
      {overflow_count > 0 ? (
        <span className="absolute bottom-0 right-0 z-20 flex h-3.5 min-w-3.5 items-center justify-center rounded-full border border-(--surface-elevated-background) bg-(--surface-muted-background) px-0.5 text-[8px] font-bold leading-none text-(--text-soft)">
          +{overflow_count}
        </span>
      ) : null}
    </div>
  );
}

function AgentAvatar({
  participant,
  size,
}: {
  participant?: AgentPrivateParticipant;
  size: "sm" | "stack" | "md";
}) {
  const src = get_icon_avatar_src(participant?.avatar ?? null);
  const class_name = size === "sm"
    ? "h-5 w-5 text-[9px]"
    : size === "stack"
      ? "h-6 w-6 text-[10px]"
      : "h-8 w-8 text-[11px]";
  return (
    <span className={cn("flex shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) font-black text-(--text-muted)", class_name)}>
      {src ? (
        <img alt={participant?.name || participant?.agent_id || "Agent"} className="h-full w-full object-cover" src={src} />
      ) : (
        get_initials(participant?.name || participant?.agent_id, "AG", 2)
      )}
    </span>
  );
}

function ThreadScopeIcon({ scope }: { scope: string }) {
  const Icon = scope === "audience" ? UsersRound : scope === "self" ? StickyNote : MessageCircle;
  return <Icon className="h-3.5 w-3.5 shrink-0 text-(--text-soft)" />;
}

function private_thread_title(thread: AgentPrivateThread, agent_id: string) {
  const peers = thread.participants.filter((participant) => participant.agent_id !== agent_id);
  if (peers.length === 0) {
    return "私有笔记";
  }
  return peers.map((participant) => participant.name || participant.agent_id).join("、");
}

function action_type_label(action_type?: string | null) {
  switch (action_type) {
    case "private_message":
      return "私信";
    case "request_reply":
      return "请求";
    case "private_note":
      return "备注";
    case "marker":
      return "标记";
    default:
      return "联络";
  }
}

function scope_label(scope: string) {
  switch (scope) {
    case "direct":
      return "一对一";
    case "audience":
      return "小范围";
    case "self":
      return "仅自己";
    default:
      return scope || "-";
  }
}

function event_route_label(event: AgentPrivateEvent, agent_id: string) {
  if (event.action_type === "private_note") {
    return "仅自己";
  }
  if (event.target_agent_id) {
    const target = event.participants.find((participant) => participant.agent_id === event.target_agent_id);
    return `给 ${target?.agent_id === agent_id ? "我" : target?.name || event.target_agent_id}`;
  }
  const audience = event.participants
    .filter((participant) => participant.agent_id !== event.source_agent_id)
    .map((participant) => participant.agent_id === agent_id ? "我" : participant.name || participant.agent_id);
  return audience.length ? `给 ${audience.join("、")}` : scope_label(event.reply_target);
}

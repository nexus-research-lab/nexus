"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { PrivateEventTimeline } from "@/features/agents/private-domain/agent-private-domain-timeline";
import { PrivateDomainToolbar } from "@/features/agents/private-domain/agent-private-domain-toolbar";
import { PrivateThreadList } from "@/features/agents/private-domain/agent-private-domain-thread-list";
import {
  AgentPrivateDomainQuery,
  list_agent_private_events_api,
  list_agent_private_threads_api,
} from "@/lib/api/agent-private-domain-api";
import { is_external_session_conversation_id } from "@/features/conversation/external-session-labels";
import {
  cn,
} from "@/lib/utils";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { Agent } from "@/types/agent/agent";
import {
  AgentPrivateEvent,
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
  const is_external_session_conversation = is_external_session_conversation_id(conversation_id);

  const query = useMemo<AgentPrivateDomainQuery>(() => ({
    room_id,
    conversation_id: is_external_session_conversation ? null : conversation_id,
    limit: is_preview ? 16 : 80,
    room_limit: is_preview ? 1 : 160,
  }), [conversation_id, is_external_session_conversation, is_preview, room_id]);

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
      <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
        <div className="grid h-full min-h-0 flex-1 grid-cols-[230px_minmax(0,1fr)] items-stretch gap-3 overflow-hidden px-4 pb-4 pt-3 2xl:grid-cols-[250px_minmax(0,1fr)]">
          <section className="flex h-full min-h-0 flex-col overflow-hidden rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_36%,transparent)]">
            <PrivateDomainToolbar
              count={threads.length}
              is_loading={threads_loading || events_loading}
              on_refresh={handle_refresh}
              title="联络"
            />
            <PrivateThreadList
              agent_id={agent.agent_id}
              class_name="min-h-0 flex-1"
              compact
              is_loading={threads_loading}
              on_select={set_selected_thread_id}
              selected_thread_id={selected_thread_id}
              threads={threads}
            />
          </section>
          <PrivateEventTimeline
            agent_id={agent.agent_id}
            class_name="h-full min-h-0"
            compact
            error={error}
            events={events}
            is_loading={events_loading}
            thread={selected_thread}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-hidden px-5 py-5 xl:px-6">
      <div className={cn(
        "mx-auto grid h-full min-h-0 w-full grid-cols-[280px_minmax(320px,1fr)] gap-3 xl:grid-cols-[300px_minmax(420px,1fr)]",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}>
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
          thread={selected_thread}
        />
      </div>
    </div>
  );
}

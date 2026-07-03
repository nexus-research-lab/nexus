"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { PrivateEventTimeline } from "@/features/agents/private-domain/agent-private-domain-timeline";
import { PrivateDomainToolbar } from "@/features/agents/private-domain/agent-private-domain-toolbar";
import { PrivateThreadList } from "@/features/agents/private-domain/agent-private-domain-thread-list";
import {
  AgentPrivateDomainQuery,
  listAgentPrivateEventsApi,
  listAgentPrivateThreadsApi,
} from "@/lib/api/agent-private-domain-api";
import { isExternalSessionConversationId } from "@/features/conversation/external-session-labels";
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
  roomId?: string | null;
  conversationId?: string | null;
  variant?: "full" | "preview";
}

export function AgentPrivateDomainView({
  agent,
  roomId: roomId = null,
  conversationId: conversationId = null,
  variant = "full",
}: AgentPrivateDomainViewProps) {
  const isPreview = variant === "preview";
  const isExternalSessionConversation = isExternalSessionConversationId(conversationId);
  const queryResetKey = [
    agent.agent_id,
    roomId ?? "",
    conversationId ?? "",
    variant,
  ].join("\x1f");
  const [threads, setThreads] = useResettableState<AgentPrivateThread[]>([], queryResetKey);
  const [selectedThreadId, setSelectedThreadId] = useResettableState<string | null>(null, queryResetKey);
  const eventsResetKey = `${queryResetKey}\x1e${selectedThreadId ?? ""}`;
  const [events, setEvents] = useResettableState<AgentPrivateEvent[]>([], eventsResetKey);
  const [threadsLoading, setThreadsLoading] = useResettableState(true, queryResetKey);
  const [eventsLoading, setEventsLoading] = useResettableState(Boolean(selectedThreadId), eventsResetKey);
  const [error, setError] = useResettableState<string | null>(null, eventsResetKey);

  const query = useMemo<AgentPrivateDomainQuery>(() => ({
    room_id: roomId,
    conversation_id: isExternalSessionConversation ? null : conversationId,
    limit: isPreview ? 16 : 80,
    room_limit: isPreview ? 1 : 160,
  }), [conversationId, isExternalSessionConversation, isPreview, roomId]);

  const loadThreads = useCallback(async () => {
    setThreadsLoading(true);
    setError(null);
    try {
      const page = await listAgentPrivateThreadsApi(agent.agent_id, query);
      const nextThreads = page.items ?? [];
      setThreads(nextThreads);
      setSelectedThreadId((current) => {
        if (current && nextThreads.some((thread) => thread.thread_id === current)) {
          return current;
        }
        return nextThreads[0]?.thread_id ?? null;
      });
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载联络记录失败");
      setThreads([]);
      setSelectedThreadId(null);
    } finally {
      setThreadsLoading(false);
    }
  }, [agent.agent_id, query]);

  const loadEvents = useCallback(async (threadId: string | null) => {
    if (!threadId) {
      setEvents([]);
      return;
    }
    setEventsLoading(true);
    setError(null);
    try {
      const page = await listAgentPrivateEventsApi(agent.agent_id, threadId, {
        ...query,
        limit: isPreview ? 40 : 120,
      });
      setEvents(page.items ?? []);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载联络消息失败");
      setEvents([]);
    } finally {
      setEventsLoading(false);
    }
  }, [agent.agent_id, isPreview, query]);

  useEffect(() => {
    let cancelled = false;
    void listAgentPrivateThreadsApi(agent.agent_id, query)
      .then((page) => {
        if (cancelled) return;
        const nextThreads = page.items ?? [];
        setThreads(nextThreads);
        setSelectedThreadId(nextThreads[0]?.thread_id ?? null);
      })
      .catch((loadError) => {
        if (cancelled) return;
        setError(loadError instanceof Error ? loadError.message : "加载联络记录失败");
        setThreads([]);
        setSelectedThreadId(null);
      })
      .finally(() => {
        if (!cancelled) {
          setThreadsLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agent.agent_id, query]);

  useEffect(() => {
    let cancelled = false;
    if (!selectedThreadId) {
      return () => {
        cancelled = true;
      };
    }
    void listAgentPrivateEventsApi(agent.agent_id, selectedThreadId, {
      ...query,
      limit: isPreview ? 40 : 120,
    })
      .then((page) => {
        if (!cancelled) {
          setEvents(page.items ?? []);
        }
      })
      .catch((loadError) => {
        if (!cancelled) {
          setError(loadError instanceof Error ? loadError.message : "加载联络消息失败");
          setEvents([]);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setEventsLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agent.agent_id, isPreview, query, selectedThreadId]);

  const selectedThread = useMemo(
    () => threads.find((thread) => thread.thread_id === selectedThreadId) ?? null,
    [selectedThreadId, threads],
  );

  const handleRefresh = useCallback(() => {
    void loadThreads();
    void loadEvents(selectedThreadId);
  }, [loadEvents, loadThreads, selectedThreadId]);

  if (isPreview) {
    return (
      <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
        <div className="grid h-full min-h-0 flex-1 grid-cols-[230px_minmax(0,1fr)] items-stretch gap-3 overflow-hidden px-4 pb-4 pt-3 2xl:grid-cols-[250px_minmax(0,1fr)]">
          <section className="flex h-full min-h-0 flex-col overflow-hidden rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_36%,transparent)]">
            <PrivateDomainToolbar
              count={threads.length}
              isLoading={threadsLoading || eventsLoading}
              onRefresh={handleRefresh}
              title="联络"
            />
            <PrivateThreadList
              agentId={agent.agent_id}
              className="min-h-0 flex-1"
              compact
              isLoading={threadsLoading}
              onSelect={setSelectedThreadId}
              selectedThreadId={selectedThreadId}
              threads={threads}
            />
          </section>
          <PrivateEventTimeline
            agentId={agent.agent_id}
            className="h-full min-h-0"
            compact
            error={error}
            events={events}
            isLoading={eventsLoading}
            thread={selectedThread}
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
            isLoading={threadsLoading}
            onRefresh={handleRefresh}
            title="联络"
          />
          <PrivateThreadList
            agentId={agent.agent_id}
            className="min-h-0 flex-1"
            isLoading={threadsLoading}
            onSelect={setSelectedThreadId}
            selectedThreadId={selectedThreadId}
            threads={threads}
          />
        </section>

        <PrivateEventTimeline
          agentId={agent.agent_id}
          error={error}
          events={events}
          isLoading={eventsLoading}
          thread={selectedThread}
        />
      </div>
    </div>
  );
}

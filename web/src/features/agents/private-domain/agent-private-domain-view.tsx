/**
 * INPUT: 当前 Agent/Room/Conversation scope 与只读联络记录 API。
 * OUTPUT: 保留最后成功快照、隔离过期响应并提供就地恢复动作的私域联络视图。
 * POS: Agent 私域记录读取编排；不把读取失败解释成数据修改，也不显示原始异常。
 */
"use client";

import { RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import {
  PrivateEventTimeline,
  type PrivateDomainReadFailure,
} from "@/features/agents/private-domain/timeline/agent-private-domain-timeline";
import { PrivateDomainToolbar } from "@/features/agents/private-domain/agent-private-domain-toolbar";
import type { PrivateDomainLocalization } from "@/features/agents/private-domain/agent-private-domain-thread-model";
import { PrivateThreadList } from "@/features/agents/private-domain/agent-private-domain-thread-list";
import {
  AgentPrivateDomainQuery,
  listAgentPrivateEventsApi,
  listAgentPrivateThreadsApi,
} from "@/lib/api/agent/private-domain-api";
import { isExternalSessionConversationId } from "@/lib/conversation/external-session";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { Agent } from "@/types/agent/agent";
import {
  AgentPrivateEvent,
  AgentPrivateThread,
} from "@/types/agent/private-domain";

import "./agent-private-domain.css";

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
  const { locale, t } = useI18n();
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
  const [threadsFailure, setThreadsFailure] = useResettableState<PrivateDomainReadFailure | null>(null, queryResetKey);
  const [eventsFailure, setEventsFailure] = useResettableState<PrivateDomainReadFailure | null>(null, eventsResetKey);
  const threadsRef = useRef(threads);
  const eventsRef = useRef(events);
  const activeQueryKeyRef = useRef(queryResetKey);
  const activeEventsKeyRef = useRef(eventsResetKey);
  threadsRef.current = threads;
  eventsRef.current = events;
  activeQueryKeyRef.current = queryResetKey;
  activeEventsKeyRef.current = eventsResetKey;
  const localization = useMemo(() => ({ locale, t }), [locale, t]);

  const query = useMemo<AgentPrivateDomainQuery>(() => ({
    room_id: roomId,
    conversation_id: isExternalSessionConversation ? null : conversationId,
    limit: isPreview ? 16 : 80,
    room_limit: isPreview ? 1 : 160,
  }), [conversationId, isExternalSessionConversation, isPreview, roomId]);

  const loadThreads = useCallback(async () => {
    const requestKey = queryResetKey;
    setThreadsLoading(true);
    try {
      const page = await listAgentPrivateThreadsApi(agent.agent_id, query);
      if (activeQueryKeyRef.current !== requestKey) {
        return;
      }
      const nextThreads = page.items ?? [];
      setThreads(nextThreads);
      setThreadsFailure(null);
      setSelectedThreadId((current) => {
        if (current && nextThreads.some((thread) => thread.thread_id === current)) {
          return current;
        }
        return nextThreads[0]?.thread_id ?? null;
      });
    } catch (loadError) {
      if (activeQueryKeyRef.current === requestKey) {
        setThreadsFailure({
          stale: threadsRef.current.length > 0,
        });
      }
    } finally {
      if (activeQueryKeyRef.current === requestKey) {
        setThreadsLoading(false);
      }
    }
  }, [
    agent.agent_id,
    query,
    queryResetKey,
    setSelectedThreadId,
    setThreads,
    setThreadsFailure,
    setThreadsLoading,
  ]);

  const loadEvents = useCallback(async (threadId: string | null) => {
    const requestKey = eventsResetKey;
    if (!threadId) {
      setEvents([]);
      setEventsFailure(null);
      return;
    }
    setEventsLoading(true);
    try {
      const page = await listAgentPrivateEventsApi(agent.agent_id, threadId, {
        ...query,
        limit: isPreview ? 40 : 120,
      });
      if (activeEventsKeyRef.current !== requestKey) {
        return;
      }
      setEvents(page.items ?? []);
      setEventsFailure(null);
    } catch (loadError) {
      if (activeEventsKeyRef.current === requestKey) {
        setEventsFailure({
          stale: eventsRef.current.length > 0,
        });
      }
    } finally {
      if (activeEventsKeyRef.current === requestKey) {
        setEventsLoading(false);
      }
    }
  }, [
    agent.agent_id,
    eventsResetKey,
    isPreview,
    query,
    setEvents,
    setEventsFailure,
    setEventsLoading,
  ]);

  useEffect(() => {
    void loadThreads();
  }, [loadThreads]);

  useEffect(() => {
    void loadEvents(selectedThreadId);
  }, [loadEvents, selectedThreadId]);

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
          <section className="surface-radius-md flex h-full min-h-0 flex-col overflow-hidden border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_36%,transparent)]">
            <PrivateDomainToolbar
              count={threads.length}
              isLoading={threadsLoading || eventsLoading}
              onRefresh={handleRefresh}
              refreshLabel={t("agent_options.contact.refresh")}
              title={t("agent_options.contact.preview_title")}
            />
            {threadsFailure ? (
              <PrivateRecordsFailure
                compact
                failure={threadsFailure}
                isLoading={threadsLoading}
                localization={localization}
                onRetry={() => void loadThreads()}
              />
            ) : null}
            {threadsFailure && !threadsFailure.stale && threads.length === 0 ? null : (
              <PrivateThreadList
                agentId={agent.agent_id}
                className="min-h-0 flex-1"
                compact
                isLoading={threadsLoading}
                localization={localization}
                onSelect={setSelectedThreadId}
                selectedThreadId={selectedThreadId}
                threads={threads}
              />
            )}
          </section>
          <PrivateEventTimeline
            agentId={agent.agent_id}
            className="h-full min-h-0"
            compact
            failure={eventsFailure}
            events={events}
            isLoading={eventsLoading}
            localization={localization}
            onRetry={() => void loadEvents(selectedThreadId)}
            thread={selectedThread}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="nexus-private-domain-layout grid min-h-0 min-w-0 flex-1 overflow-hidden">
      <aside className="flex min-h-0 min-w-0 flex-col overflow-hidden bg-(--surface-raised-background)">
        <PrivateDomainToolbar
          count={threads.length}
          isLoading={threadsLoading || eventsLoading}
          onRefresh={handleRefresh}
          refreshLabel={t("agent_options.contact.refresh")}
          title={t("agent_options.contact.records_title")}
        />
        {threadsFailure ? (
          <PrivateRecordsFailure
            failure={threadsFailure}
            isLoading={threadsLoading}
            localization={localization}
            onRetry={() => void loadThreads()}
          />
        ) : null}
        {threadsFailure && !threadsFailure.stale && threads.length === 0 ? null : (
          <PrivateThreadList
            agentId={agent.agent_id}
            className="min-h-0 flex-1"
            isLoading={threadsLoading}
            localization={localization}
            onSelect={setSelectedThreadId}
            selectedThreadId={selectedThreadId}
            threads={threads}
          />
        )}
      </aside>

      <PrivateEventTimeline
        agentId={agent.agent_id}
        failure={eventsFailure}
        events={events}
        isLoading={eventsLoading}
        localization={localization}
        onRetry={() => void loadEvents(selectedThreadId)}
        thread={selectedThread}
      />
    </div>
  );
}

function PrivateRecordsFailure({
  compact = false,
  failure,
  isLoading,
  localization,
  onRetry,
}: {
  compact?: boolean;
  failure: PrivateDomainReadFailure;
  isLoading: boolean;
  localization: PrivateDomainLocalization;
  onRetry: () => void;
}) {
  return (
    <UiResourceState
      className={compact ? "mx-2 min-h-0 py-3" : "mx-3 min-h-0 py-3"}
      impact={failure.stale
        ? localization.t("agent_options.contact.private_records_stale_impact")
        : localization.t("agent_options.contact.private_records_unavailable_impact")}
      primaryAction={{
        busy: isLoading,
        busyLabel: localization.t("common.loading"),
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: localization.t("agent_options.contact.retry_private_records"),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={localization.t("agent_options.contact.private_records_load_failed")}
    />
  );
}

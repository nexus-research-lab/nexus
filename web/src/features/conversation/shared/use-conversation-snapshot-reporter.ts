import { useEffect, useRef } from "react";

import {
  areEquivalentSessionKeys,
  parseSessionKey,
} from "@/lib/conversation/session-key";
import type { Message } from "@/types/conversation/message/entity";

interface ConversationActivitySnapshot {
  scopeKey: string;
  latestReplyTimestamp: number | null;
}

function getLatestReplyTimestamp(messages: Message[]): number | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const timestamp = getAssistantReplyTimestamp(messages[index]);
    if (timestamp !== null) {
      return timestamp;
    }
  }
  return normalizeConversationTimestamp(messages.at(-1)?.timestamp);
}

function getAssistantReplyTimestamp(message: Message): number | null {
  if (message.role !== "assistant") {
    return null;
  }
  return normalizeConversationTimestamp(
    message.result_summary?.timestamp ?? message.timestamp,
  );
}

function normalizeConversationTimestamp(
  timestamp: number | undefined,
): number | null {
  return timestamp !== undefined && Number.isFinite(timestamp) && timestamp > 0
    ? timestamp
    : null;
}

/** 历史加载只建立基线，同一会话出现更新回复时才刷新活跃时间。 */
function shouldEmitConversationActivity(
  previous: ConversationActivitySnapshot | null,
  scopeKey: string,
  latestReplyTimestamp: number | null,
): boolean {
  return Boolean(
    latestReplyTimestamp &&
      previous?.scopeKey === scopeKey &&
      latestReplyTimestamp > (previous.latestReplyTimestamp ?? 0),
  );
}

export interface ConversationSnapshotBuildInput {
  scope_key: string;
  has_user_input: boolean;
  last_message: Message;
  latest_reply_timestamp: number | null;
  message_count: number;
  should_report_last_activity: boolean;
}

export function hasConversationUserInput(messages: readonly Message[]): boolean {
  return messages.some((message) => (
    message.role === "user"
    && message.hidden_from_user !== true
    && message.is_synthetic !== true
  ));
}

export function doConversationMessagesBelongToScope(
  messages: readonly Message[],
  scopeKey: string | null,
): boolean {
  if (!scopeKey) {
    return false;
  }
  return messages.every((message) => {
    if (message.conversation_id?.trim() === scopeKey) {
      return true;
    }
    if (areEquivalentSessionKeys(message.session_key, scopeKey)) {
      return true;
    }
    const parsedSessionKey = parseSessionKey(message.session_key);
    return parsedSessionKey.conversation_id === scopeKey
      || parsedSessionKey.ref === scopeKey;
  });
}

export function shouldReportConversationSnapshot({
  messages,
  observed_scope_key: observedScopeKey,
  scope_key: scopeKey,
}: {
  messages: readonly Message[];
  observed_scope_key: string | null;
  scope_key: string | null;
}): boolean {
  return Boolean(
    scopeKey
    && observedScopeKey === scopeKey
    && messages.length > 0
    && doConversationMessagesBelongToScope(messages, scopeKey)
  );
}

interface ConversationActivityPatch {
  last_activity_at?: number;
}

export function buildConversationActivityPatch({
  latest_reply_timestamp: latestReplyTimestamp,
  should_report_last_activity: shouldReportLastActivity,
}: Pick<
  ConversationSnapshotBuildInput,
  "latest_reply_timestamp" | "should_report_last_activity"
>): ConversationActivityPatch {
  if (!shouldReportLastActivity || latestReplyTimestamp === null) {
    return {};
  }
  return { last_activity_at: latestReplyTimestamp };
}

interface UseConversationSnapshotReporterOptions<TSnapshot> {
  scope_key: string | null;
  messages: Message[];
  build_snapshot: (input: ConversationSnapshotBuildInput) => TSnapshot;
  on_snapshot_change?: (snapshot: TSnapshot) => void;
}

export function useConversationSnapshotReporter<TSnapshot>({
  scope_key: scopeKey,
  messages,
  build_snapshot: buildSnapshot,
  on_snapshot_change: onSnapshotChange,
}: UseConversationSnapshotReporterOptions<TSnapshot>) {
  const lastSnapshotKeyRef = useRef<string | null>(null);
  const lastActivitySnapshotRef =
    useRef<ConversationActivitySnapshot | null>(null);
  const observedScopeKeyRef = useRef<string | null>(scopeKey);

  useEffect(() => {
    const observedScopeKey = observedScopeKeyRef.current;
    observedScopeKeyRef.current = scopeKey;
    if (observedScopeKey !== scopeKey) {
      // Conversation 切换后的首个 effect 仍可能看到上一会话的 messages。
      // 必须先等待消息集合按新 scope 清空/装载，不能把旧用户输入投影给新 draft。
      lastSnapshotKeyRef.current = null;
      lastActivitySnapshotRef.current = null;
      return;
    }
    if (!scopeKey || !shouldReportConversationSnapshot({
      messages,
      observed_scope_key: observedScopeKey,
      scope_key: scopeKey,
    })) return;

    const lastMessage = messages[messages.length - 1];
    const latestReplyTimestamp = getLatestReplyTimestamp(messages);
    const shouldReportLastActivity = shouldEmitConversationActivity(
      lastActivitySnapshotRef.current,
      scopeKey,
      latestReplyTimestamp,
    );
    const snapshot = buildSnapshot({
      scope_key: scopeKey,
      has_user_input: hasConversationUserInput(messages),
      last_message: lastMessage,
      latest_reply_timestamp: latestReplyTimestamp,
      message_count: messages.length,
      should_report_last_activity: shouldReportLastActivity,
    });
    const snapshotKey = JSON.stringify(snapshot);
    const nextActivitySnapshot = {
      scopeKey,
      latestReplyTimestamp,
    };

    // 历史加载只同步快照，不应该因为切换视图刷新活跃时间。
    if (lastSnapshotKeyRef.current === snapshotKey) {
      lastActivitySnapshotRef.current = nextActivitySnapshot;
      return;
    }

    lastSnapshotKeyRef.current = snapshotKey;
    lastActivitySnapshotRef.current = nextActivitySnapshot;
    onSnapshotChange?.(snapshot);
  }, [buildSnapshot, messages, onSnapshotChange, scopeKey]);
}

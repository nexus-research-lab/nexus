import { useEffect, useRef } from "react";

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
  last_message: Message;
  latest_reply_timestamp: number | null;
  should_report_last_activity: boolean;
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

  useEffect(() => {
    if (!scopeKey || messages.length === 0) return;

    const lastMessage = messages[messages.length - 1];
    const latestReplyTimestamp = getLatestReplyTimestamp(messages);
    const shouldReportLastActivity = shouldEmitConversationActivity(
      lastActivitySnapshotRef.current,
      scopeKey,
      latestReplyTimestamp,
    );
    const snapshot = buildSnapshot({
      scope_key: scopeKey,
      last_message: lastMessage,
      latest_reply_timestamp: latestReplyTimestamp,
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

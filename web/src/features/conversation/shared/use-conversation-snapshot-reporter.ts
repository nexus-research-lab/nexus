import { useEffect, useRef } from "react";

import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";

interface ConversationActivitySnapshot {
  scopeKey: string;
  latestReplyTimestamp: number | null;
}

function getLatestReplyTimestamp(messages: Message[]): number | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role !== "assistant") {
      continue;
    }
    const assistant = message as AssistantMessage;
    const timestamp =
      assistant.result_summary?.timestamp ?? assistant.timestamp;
    if (Number.isFinite(timestamp) && timestamp > 0) {
      return timestamp;
    }
  }
  const lastTimestamp = messages[messages.length - 1]?.timestamp;
  return lastTimestamp && Number.isFinite(lastTimestamp) && lastTimestamp > 0
    ? lastTimestamp
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

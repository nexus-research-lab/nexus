import { useEffect, useRef } from "react";

import type { Message } from "@/types/conversation/message";

import {
  build_conversation_activity_snapshot,
  get_latest_reply_timestamp,
  should_emit_conversation_activity,
  type ConversationActivitySnapshot,
} from "./utils";

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
    const latestReplyTimestamp = get_latest_reply_timestamp(messages);
    const shouldReportLastActivity = should_emit_conversation_activity(
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
    const nextActivitySnapshot = build_conversation_activity_snapshot(
      scopeKey,
      latestReplyTimestamp,
    );

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

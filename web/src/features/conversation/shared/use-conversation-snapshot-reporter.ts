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
  scope_key,
  messages,
  build_snapshot,
  on_snapshot_change,
}: UseConversationSnapshotReporterOptions<TSnapshot>) {
  const last_snapshot_key_ref = useRef<string | null>(null);
  const last_activity_snapshot_ref =
    useRef<ConversationActivitySnapshot | null>(null);

  useEffect(() => {
    if (!scope_key || messages.length === 0) return;

    const last_message = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    const should_report_last_activity = should_emit_conversation_activity(
      last_activity_snapshot_ref.current,
      scope_key,
      latest_reply_timestamp,
    );
    const snapshot = build_snapshot({
      scope_key,
      last_message,
      latest_reply_timestamp,
      should_report_last_activity,
    });
    const snapshot_key = JSON.stringify(snapshot);
    const next_activity_snapshot = build_conversation_activity_snapshot(
      scope_key,
      latest_reply_timestamp,
    );

    // 历史加载只同步快照，不应该因为切换视图刷新活跃时间。
    if (last_snapshot_key_ref.current === snapshot_key) {
      last_activity_snapshot_ref.current = next_activity_snapshot;
      return;
    }

    last_snapshot_key_ref.current = snapshot_key;
    last_activity_snapshot_ref.current = next_activity_snapshot;
    on_snapshot_change?.(snapshot);
  }, [build_snapshot, messages, on_snapshot_change, scope_key]);
}

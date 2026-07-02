/**
 * =====================================================
 * @File   ：agent-conversation-runtime-machine.ts
 * @Date   ：2026-04-09 20:53:00
 * @Author ：leemysw
 * 2026-04-09 20:53:00   Create
 * =====================================================
 */

import {
  AssistantMessage,
  AssistantMessageStatus,
  ChatAckData,
  Message,
  RoundLifecycleStatus,
} from '@/types';
import {
  AgentConversationChatType,
  AgentConversationRuntimePhase,
} from '@/types/agent/agent-conversation';
import { are_runtime_snapshots_equal } from './conversation-runtime-state';

export interface ActiveMessageTracker {
  round_id: string;
  status: AssistantMessageStatus;
}

export interface AgentConversationRuntimeSnapshot {
  phase: AgentConversationRuntimePhase;
  sending_round_ids: string[];
  running_round_ids: string[];
  terminal_round_ids: string[];
  live_round_ids: string[];
  active_messages: Record<string, ActiveMessageTracker>;
  pending_permission_count: number;
  is_loading: boolean;
}

function isTerminalAssistantStatus(status?: AssistantMessageStatus): boolean {
  return status === 'done' || status === 'cancelled' || status === 'error';
}

function hasTerminalAssistantProjection(message: AssistantMessage): boolean {
  return Boolean(message.result_summary)
    || Boolean(message.stop_reason)
    || isTerminalAssistantStatus(message.stream_status);
}

function buildActiveMessageRecord(
  trackers: Map<string, ActiveMessageTracker>,
): Record<string, ActiveMessageTracker> {
  return Object.fromEntries(trackers.entries());
}

export class AgentConversationRuntimeMachine {
  private chat_type: AgentConversationChatType;

  private sending_round_ids = new Set<string>();

  private running_round_ids = new Set<string>();

  private terminal_round_ids = new Set<string>();

  private active_message_trackers = new Map<string, ActiveMessageTracker>();

  private pending_permission_count = 0;

  private listeners = new Set<() => void>();

  private snapshot_cache: AgentConversationRuntimeSnapshot | null = null;

  public constructor(chatType: AgentConversationChatType) {
    this.chat_type = chatType;
  }

  // useSyncExternalStore subscription. Returns an unsubscribe fn.
  public subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  // Call after any mutation. Recomputes the snapshot, and only notifies
  // subscribers when it actually changed — so the cached snapshot stays
  // referentially stable across no-op transitions (required by
  // useSyncExternalStore to avoid render loops).
  public emit(): void {
    const next = this.compute_snapshot();
    if (this.snapshot_cache && are_runtime_snapshots_equal(this.snapshot_cache, next)) {
      return;
    }
    this.snapshot_cache = next;
    for (const listener of this.listeners) {
      listener();
    }
  }

  public set_chat_type(chatType: AgentConversationChatType): void {
    this.chat_type = chatType;
  }

  public reset(): void {
    this.sending_round_ids.clear();
    this.running_round_ids.clear();
    this.terminal_round_ids.clear();
    this.active_message_trackers.clear();
    this.pending_permission_count = 0;
  }

  public track_outbound_round(roundId: string): void {
    this.terminal_round_ids.delete(roundId);
    this.sending_round_ids.add(roundId);
  }

  public clear_round(
    roundId?: string | null,
    includeRelatedRounds: boolean = false,
  ): void {
    if (!roundId) {
      return;
    }

    const shouldClearRound = (trackedRoundId: string) => (
      trackedRoundId === roundId ||
      (includeRelatedRounds && trackedRoundId.startsWith(`${roundId}:`))
    );

    for (const trackedRoundId of [...this.sending_round_ids]) {
      if (shouldClearRound(trackedRoundId)) {
        this.sending_round_ids.delete(trackedRoundId);
      }
    }

    for (const trackedRoundId of [...this.running_round_ids]) {
      if (shouldClearRound(trackedRoundId)) {
        this.running_round_ids.delete(trackedRoundId);
      }
    }

    for (const trackedRoundId of [...this.terminal_round_ids]) {
      if (shouldClearRound(trackedRoundId)) {
        this.terminal_round_ids.delete(trackedRoundId);
      }
    }

    for (const [messageId, tracker] of this.active_message_trackers.entries()) {
      if (shouldClearRound(tracker.round_id)) {
        this.active_message_trackers.delete(messageId);
      }
    }
  }

  public update_message_status(
    messageId: string,
    status: AssistantMessageStatus,
    roundId?: string | null,
  ): void {
    const currentTracker = this.active_message_trackers.get(messageId);
    const resolvedRoundId = roundId ?? currentTracker?.round_id ?? '';
    if (resolvedRoundId && this.is_round_terminal(resolvedRoundId)) {
      this.active_message_trackers.delete(messageId);
      return;
    }

    if (isTerminalAssistantStatus(status)) {
      this.active_message_trackers.delete(messageId);
      return;
    }

    this.active_message_trackers.set(messageId, {
      round_id: resolvedRoundId,
      status,
    });
  }

  public track_chat_ack(ack: ChatAckData): void {
    this.sending_round_ids.delete(ack.round_id);
    const pendingCount = ack.pending?.length ?? 0;

    for (const slot of ack.pending ?? []) {
      const agentRoundId = (
        slot.round_id ||
        (pendingCount > 1 ? `${ack.round_id}:${slot.agent_id}` : ack.round_id)
      );
      if (this.is_round_terminal(agentRoundId)) {
        continue;
      }
      this.active_message_trackers.set(slot.msg_id, {
        round_id: agentRoundId,
        status: slot.status ?? 'pending',
      });
    }
  }

  public track_assistant_message(message: AssistantMessage): void {
    if (this.is_round_terminal(message.round_id)) {
      this.active_message_trackers.delete(message.message_id);
      return;
    }

    if (hasTerminalAssistantProjection(message)) {
      this.active_message_trackers.delete(message.message_id);
      return;
    }

    this.active_message_trackers.set(message.message_id, {
      round_id: message.round_id,
      status: message.stream_status ?? 'streaming',
    });
  }

  public track_round_status(
    roundId: string,
    status: RoundLifecycleStatus,
  ): void {
    if (status === 'running') {
      this.sending_round_ids.delete(roundId);
      this.terminal_round_ids.delete(roundId);
      this.running_round_ids.add(roundId);
      return;
    }

    this.terminal_round_ids.add(roundId);
    this.clear_round(roundId, this.chat_type === 'group');
  }

  public sync_running_rounds(roundIds: string[]): void {
    const nextRunningRoundIds = new Set(
      roundIds
        .map((roundId) => roundId.trim())
        .filter(Boolean),
    );

    this.running_round_ids = nextRunningRoundIds;
    for (const roundId of nextRunningRoundIds) {
      this.sending_round_ids.delete(roundId);
      this.terminal_round_ids.delete(roundId);
    }
  }

  public set_pending_permission_count(count: number): void {
    this.pending_permission_count = Math.max(0, count);
  }

  public reconcile_from_snapshot(messages: Message[]): void {
    const terminalMessageIds = new Set<string>();

    for (const message of messages) {
      if (message.role !== 'assistant') {
        continue;
      }

      if (hasTerminalAssistantProjection(message)) {
        terminalMessageIds.add(message.message_id);
      }
    }

    const nextTrackers = new Map<string, ActiveMessageTracker>();
    for (const [messageId, tracker] of this.active_message_trackers.entries()) {
      if (terminalMessageIds.has(messageId) || this.is_round_terminal(tracker.round_id)) {
        continue;
      }
      nextTrackers.set(messageId, tracker);
    }

    if (this.chat_type !== 'group') {
      for (const message of messages) {
        if (message.role !== 'assistant') {
          continue;
        }
        if (
          hasTerminalAssistantProjection(message) ||
          this.is_round_terminal(message.round_id)
        ) {
          continue;
        }
        nextTrackers.set(message.message_id, {
          round_id: message.round_id,
          status: message.stream_status ?? 'streaming',
        });
      }
    }

    this.active_message_trackers = nextTrackers;
  }

  // getSnapshot for useSyncExternalStore: stable ref between emits.
  public snapshot(): AgentConversationRuntimeSnapshot {
    return (this.snapshot_cache ??= this.compute_snapshot());
  }

  private compute_snapshot(): AgentConversationRuntimeSnapshot {
    const phase = this.resolve_phase();
    const liveRoundIds = new Set<string>([
      ...this.sending_round_ids,
      ...this.running_round_ids,
    ]);
    for (const tracker of this.active_message_trackers.values()) {
      if (tracker.round_id) {
        liveRoundIds.add(tracker.round_id);
      }
    }
    return {
      phase,
      sending_round_ids: [...this.sending_round_ids],
      running_round_ids: [...this.running_round_ids],
      terminal_round_ids: [...this.terminal_round_ids],
      live_round_ids: [...liveRoundIds],
      active_messages: buildActiveMessageRecord(this.active_message_trackers),
      pending_permission_count: this.pending_permission_count,
      is_loading: phase !== 'idle',
    };
  }

  public is_round_terminal(roundId: string): boolean {
    if (!roundId) {
      return false;
    }
    if (this.terminal_round_ids.has(roundId)) {
      return true;
    }
    if (this.chat_type !== 'group') {
      return false;
    }
    for (const terminalRoundId of this.terminal_round_ids) {
      if (roundId.startsWith(`${terminalRoundId}:`)) {
        return true;
      }
    }
    return false;
  }

  private resolve_phase(): AgentConversationRuntimePhase {
    if (this.pending_permission_count > 0) {
      return 'awaiting_permission';
    }

    for (const tracker of this.active_message_trackers.values()) {
      if (tracker.status === 'streaming') {
        return 'streaming';
      }
    }

    if (this.sending_round_ids.size > 0) {
      return 'sending';
    }

    if (this.running_round_ids.size > 0 || this.active_message_trackers.size > 0) {
      return 'running';
    }

    return 'idle';
  }
}

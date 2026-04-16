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

export interface ActiveMessageTracker {
  round_id: string;
  status: AssistantMessageStatus;
}

export interface AgentConversationRuntimeSnapshot {
  phase: AgentConversationRuntimePhase;
  pending_round_ids: string[];
  running_round_ids: string[];
  terminal_round_ids: string[];
  active_messages: Record<string, ActiveMessageTracker>;
  pending_permission_count: number;
  is_loading: boolean;
}

function is_terminal_assistant_status(status?: AssistantMessageStatus): boolean {
  return status === 'done' || status === 'cancelled' || status === 'error';
}

function build_active_message_record(
  trackers: Map<string, ActiveMessageTracker>,
): Record<string, ActiveMessageTracker> {
  return Object.fromEntries(trackers.entries());
}

export class AgentConversationRuntimeMachine {
  private chat_type: AgentConversationChatType;

  private pending_round_ids = new Set<string>();

  private running_round_ids = new Set<string>();

  private terminal_round_ids = new Set<string>();

  private active_message_trackers = new Map<string, ActiveMessageTracker>();

  private pending_permission_count = 0;

  public constructor(chat_type: AgentConversationChatType) {
    this.chat_type = chat_type;
  }

  public set_chat_type(chat_type: AgentConversationChatType): void {
    this.chat_type = chat_type;
  }

  public reset(): void {
    this.pending_round_ids.clear();
    this.running_round_ids.clear();
    this.terminal_round_ids.clear();
    this.active_message_trackers.clear();
    this.pending_permission_count = 0;
  }

  public queue_round(round_id: string): void {
    this.terminal_round_ids.delete(round_id);
    this.pending_round_ids.add(round_id);
  }

  public clear_round(
    round_id?: string | null,
    include_related_rounds: boolean = false,
  ): void {
    if (!round_id) {
      return;
    }

    const should_clear_round = (tracked_round_id: string) => (
      tracked_round_id === round_id ||
      (include_related_rounds && tracked_round_id.startsWith(`${round_id}:`))
    );

    for (const tracked_round_id of [...this.pending_round_ids]) {
      if (should_clear_round(tracked_round_id)) {
        this.pending_round_ids.delete(tracked_round_id);
      }
    }

    for (const tracked_round_id of [...this.running_round_ids]) {
      if (should_clear_round(tracked_round_id)) {
        this.running_round_ids.delete(tracked_round_id);
      }
    }

    for (const tracked_round_id of [...this.terminal_round_ids]) {
      if (should_clear_round(tracked_round_id)) {
        this.terminal_round_ids.delete(tracked_round_id);
      }
    }

    for (const [message_id, tracker] of this.active_message_trackers.entries()) {
      if (should_clear_round(tracker.round_id)) {
        this.active_message_trackers.delete(message_id);
      }
    }
  }

  public update_message_status(
    message_id: string,
    status: AssistantMessageStatus,
    round_id?: string | null,
  ): void {
    const current_tracker = this.active_message_trackers.get(message_id);
    const resolved_round_id = round_id ?? current_tracker?.round_id ?? '';
    if (resolved_round_id && this.is_round_terminal(resolved_round_id)) {
      this.active_message_trackers.delete(message_id);
      return;
    }

    if (is_terminal_assistant_status(status)) {
      this.active_message_trackers.delete(message_id);
      return;
    }

    this.active_message_trackers.set(message_id, {
      round_id: resolved_round_id,
      status,
    });
  }

  public track_chat_ack(ack: ChatAckData): void {
    this.pending_round_ids.delete(ack.round_id);
    const pending_count = ack.pending?.length ?? 0;

    for (const slot of ack.pending ?? []) {
      const agent_round_id = (
        slot.round_id ||
        (pending_count > 1 ? `${ack.round_id}:${slot.agent_id}` : ack.round_id)
      );
      if (this.is_round_terminal(agent_round_id)) {
        continue;
      }
      this.active_message_trackers.set(slot.msg_id, {
        round_id: agent_round_id,
        status: slot.status ?? 'pending',
      });
    }
  }

  public track_assistant_message(message: AssistantMessage): void {
    if (this.is_round_terminal(message.round_id)) {
      this.active_message_trackers.delete(message.message_id);
      return;
    }

    if (message.stop_reason || is_terminal_assistant_status(message.stream_status)) {
      this.active_message_trackers.delete(message.message_id);
      return;
    }

    this.active_message_trackers.set(message.message_id, {
      round_id: message.round_id,
      status: message.stream_status ?? 'streaming',
    });
  }

  public track_round_status(
    round_id: string,
    status: RoundLifecycleStatus,
  ): void {
    if (status === 'running') {
      this.pending_round_ids.delete(round_id);
      this.terminal_round_ids.delete(round_id);
      this.running_round_ids.add(round_id);
      return;
    }

    this.terminal_round_ids.add(round_id);
    this.clear_round(round_id, this.chat_type === 'group');
  }

  public sync_running_rounds(round_ids: string[]): void {
    const next_running_round_ids = new Set(
      round_ids
        .map((round_id) => round_id.trim())
        .filter(Boolean),
    );

    this.running_round_ids = next_running_round_ids;
    for (const round_id of next_running_round_ids) {
      this.pending_round_ids.delete(round_id);
      this.terminal_round_ids.delete(round_id);
    }
  }

  public set_pending_permission_count(count: number): void {
    this.pending_permission_count = Math.max(0, count);
  }

  public reconcile_from_snapshot(messages: Message[]): void {
    const terminal_message_ids = new Set<string>();

    for (const message of messages) {
      if (message.role !== 'assistant') {
        continue;
      }

      if (message.stop_reason || is_terminal_assistant_status(message.stream_status)) {
        terminal_message_ids.add(message.message_id);
      }
    }

    const next_trackers = new Map<string, ActiveMessageTracker>();
    for (const [message_id, tracker] of this.active_message_trackers.entries()) {
      if (terminal_message_ids.has(message_id) || this.is_round_terminal(tracker.round_id)) {
        continue;
      }
      next_trackers.set(message_id, tracker);
    }

    if (this.chat_type !== 'group') {
      for (const message of messages) {
        if (message.role !== 'assistant') {
          continue;
        }
        if (
          message.stop_reason ||
          is_terminal_assistant_status(message.stream_status) ||
          this.is_round_terminal(message.round_id)
        ) {
          continue;
        }
        next_trackers.set(message.message_id, {
          round_id: message.round_id,
          status: message.stream_status ?? 'streaming',
        });
      }
    }

    this.active_message_trackers = next_trackers;
  }

  public snapshot(): AgentConversationRuntimeSnapshot {
    const phase = this.resolve_phase();
    return {
      phase,
      pending_round_ids: [...this.pending_round_ids],
      running_round_ids: [...this.running_round_ids],
      terminal_round_ids: [...this.terminal_round_ids],
      active_messages: build_active_message_record(this.active_message_trackers),
      pending_permission_count: this.pending_permission_count,
      is_loading: phase !== 'idle',
    };
  }

  public is_round_terminal(round_id: string): boolean {
    if (!round_id) {
      return false;
    }
    if (this.terminal_round_ids.has(round_id)) {
      return true;
    }
    if (this.chat_type !== 'group') {
      return false;
    }
    for (const terminal_round_id of this.terminal_round_ids) {
      if (round_id.startsWith(`${terminal_round_id}:`)) {
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

    if (this.pending_round_ids.size > 0) {
      return 'queued';
    }

    if (this.running_round_ids.size > 0 || this.active_message_trackers.size > 0) {
      return 'running';
    }

    return 'idle';
  }
}

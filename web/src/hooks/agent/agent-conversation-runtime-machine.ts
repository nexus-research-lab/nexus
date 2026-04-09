/**
 * =====================================================
 * @File   ：agent-conversation-runtime-machine.ts
 * @Date   ：2026-04-08 12:05:47
 * @Author ：leemysw
 * 2026-04-08 12:05:47   Create
 * =====================================================
 */

import {
  AssistantMessage,
  AssistantMessageStatus,
  ChatAckData,
  Message,
  ResultMessage,
} from '@/types';
import {
  AgentConversationChatType,
  AgentConversationRuntimePhase,
} from '@/types/agent-conversation';

export interface ActiveMessageTracker {
  round_id: string;
  status: AssistantMessageStatus;
}

export interface AgentConversationRuntimeSnapshot {
  phase: AgentConversationRuntimePhase;
  pending_round_ids: string[];
  open_round_ids: string[];
  active_round_ids: string[];
  active_messages: Record<string, ActiveMessageTracker>;
  pending_permission_count: number;
  is_server_generating: boolean;
  is_loading: boolean;
}

function is_terminal_assistant_status(status?: AssistantMessageStatus): boolean {
  return status === 'done' || status === 'cancelled' || status === 'error';
}

function collect_completed_round_ids(messages: Message[]): Set<string> {
  const completed_round_ids = new Set<string>();
  for (const message of messages) {
    if (message.role === 'result') {
      completed_round_ids.add(message.round_id);
    }
  }
  return completed_round_ids;
}

function collect_open_round_ids(messages: Message[]): Set<string> {
  const completed_round_ids = collect_completed_round_ids(messages);
  const open_round_ids = new Set<string>();
  for (const message of messages) {
    if (
      completed_round_ids.has(message.round_id) ||
      (message.role !== 'user' && message.role !== 'assistant')
    ) {
      continue;
    }
    // 中文注释：Round 是否仍在执行只看“是否已有 ResultMessage 收口”。
    // assistant 的 is_complete / done 仅表示单条快照结束，不能代表整轮结束，
    // 否则工具执行、权限确认后的空档会被提前判成 idle。
    open_round_ids.add(message.round_id);
  }
  return open_round_ids;
}

function build_active_message_record(
  trackers: Map<string, ActiveMessageTracker>,
): Record<string, ActiveMessageTracker> {
  return Object.fromEntries(trackers.entries());
}

export class AgentConversationRuntimeMachine {
  private chat_type: AgentConversationChatType;

  private pending_round_ids = new Set<string>();

  private open_round_ids = new Set<string>();

  private active_round_ids = new Set<string>();

  private completed_round_ids = new Set<string>();

  private active_message_trackers = new Map<string, ActiveMessageTracker>();

  private pending_permission_count = 0;

  private is_server_generating = false;

  public constructor(chat_type: AgentConversationChatType) {
    this.chat_type = chat_type;
  }

  public set_chat_type(chat_type: AgentConversationChatType): void {
    this.chat_type = chat_type;
  }

  public reset(): void {
    this.pending_round_ids.clear();
    this.open_round_ids.clear();
    this.active_round_ids.clear();
    this.completed_round_ids.clear();
    this.active_message_trackers.clear();
    this.pending_permission_count = 0;
    this.is_server_generating = false;
  }

  public queue_round(round_id: string): void {
    this.completed_round_ids.delete(round_id);
    this.pending_round_ids.add(round_id);
    this.open_round_ids.add(round_id);
    this.is_server_generating = false;
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

    for (const tracked_round_id of [...this.active_round_ids]) {
      if (should_clear_round(tracked_round_id)) {
        this.active_round_ids.delete(tracked_round_id);
      }
    }

    for (const tracked_round_id of [...this.open_round_ids]) {
      if (should_clear_round(tracked_round_id)) {
        this.open_round_ids.delete(tracked_round_id);
      }
    }

    for (const [message_id, tracker] of this.active_message_trackers.entries()) {
      if (should_clear_round(tracker.round_id)) {
        this.active_message_trackers.delete(message_id);
      }
    }

    this.reconcile_server_generating_flag();
  }

  public update_message_status(
    message_id: string,
    status: AssistantMessageStatus,
    round_id?: string | null,
  ): void {
    const current_tracker = this.active_message_trackers.get(message_id);
    const resolved_round_id = round_id ?? current_tracker?.round_id ?? '';

    if (resolved_round_id && this.completed_round_ids.has(resolved_round_id)) {
      this.active_message_trackers.delete(message_id);
      this.reconcile_server_generating_flag();
      return;
    }

    if (is_terminal_assistant_status(status)) {
      this.active_message_trackers.delete(message_id);
      this.reconcile_server_generating_flag();
      return;
    }

    if (resolved_round_id) {
      this.active_round_ids.add(resolved_round_id);
    }
    this.active_message_trackers.set(message_id, {
      round_id: resolved_round_id,
      status,
    });
  }

  public track_chat_ack(ack: ChatAckData): void {
    this.pending_round_ids.delete(ack.round_id);
    if (this.completed_round_ids.has(ack.round_id)) {
      return;
    }
    const pending_count = ack.pending?.length ?? 0;

    for (const slot of ack.pending ?? []) {
      const agent_round_id = (
        slot.round_id ||
        (pending_count > 1 ? `${ack.round_id}:${slot.agent_id}` : ack.round_id)
      );
      if (this.completed_round_ids.has(agent_round_id)) {
        continue;
      }
      this.active_round_ids.add(agent_round_id);
      this.active_message_trackers.set(slot.msg_id, {
        round_id: agent_round_id,
        status: slot.status ?? 'pending',
      });
    }
  }

  public track_assistant_message(message: AssistantMessage): void {
    this.pending_round_ids.delete(message.round_id);
    if (this.completed_round_ids.has(message.round_id)) {
      this.active_message_trackers.delete(message.message_id);
      this.reconcile_server_generating_flag();
      return;
    }
    this.open_round_ids.add(message.round_id);
    this.active_round_ids.add(message.round_id);

    if (message.stop_reason || is_terminal_assistant_status(message.stream_status)) {
      this.active_message_trackers.delete(message.message_id);
      this.reconcile_server_generating_flag();
      return;
    }

    this.active_message_trackers.set(message.message_id, {
      round_id: message.round_id,
      status: message.stream_status ?? 'streaming',
    });
  }

  public track_result_message(message: ResultMessage): void {
    this.completed_round_ids.add(message.round_id);
    this.open_round_ids.delete(message.round_id);
    this.clear_round(message.round_id, this.chat_type === 'group');
  }

  public set_pending_permission_count(count: number): void {
    this.pending_permission_count = Math.max(0, count);
    this.reconcile_server_generating_flag();
  }

  public mark_session_generating(): void {
    this.is_server_generating = true;
  }

  public mark_session_stopped(): void {
    this.reset();
  }

  public reconcile_from_snapshot(messages: Message[]): void {
    const completed_round_ids = collect_completed_round_ids(messages);
    this.completed_round_ids = completed_round_ids;
    const snapshot_open_round_ids = collect_open_round_ids(messages);
    const terminal_message_ids = new Set<string>();

    for (const message of messages) {
      if (message.role !== 'assistant') {
        continue;
      }

      if (message.stop_reason || is_terminal_assistant_status(message.stream_status)) {
        terminal_message_ids.add(message.message_id);
      }
    }

    this.pending_round_ids = new Set(
      [...this.pending_round_ids].filter((round_id) => !completed_round_ids.has(round_id)),
    );
    this.open_round_ids = new Set([
      ...[...this.open_round_ids].filter((round_id) => !completed_round_ids.has(round_id)),
      ...snapshot_open_round_ids,
    ]);
    this.active_round_ids = new Set([
      ...[...this.active_round_ids].filter((round_id) => !completed_round_ids.has(round_id)),
      ...(this.chat_type === 'group' ? [] : [...snapshot_open_round_ids]),
    ]);

    const next_trackers = new Map<string, ActiveMessageTracker>();
    for (const [message_id, tracker] of this.active_message_trackers.entries()) {
      if (completed_round_ids.has(tracker.round_id) || terminal_message_ids.has(message_id)) {
        continue;
      }
      next_trackers.set(message_id, tracker);
    }

    if (this.chat_type !== 'group') {
      for (const message of messages) {
        if (
          message.role !== 'assistant' ||
          message.stop_reason ||
          is_terminal_assistant_status(message.stream_status)
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
    this.active_round_ids = new Set([
      ...this.active_round_ids,
      ...[...next_trackers.values()].map((tracker) => tracker.round_id).filter(Boolean),
    ]);
    this.reconcile_server_generating_flag();
  }

  public snapshot(): AgentConversationRuntimeSnapshot {
    const phase = this.resolve_phase();
    return {
      phase,
      pending_round_ids: [...this.pending_round_ids],
      open_round_ids: [...this.open_round_ids],
      active_round_ids: [...this.active_round_ids],
      active_messages: build_active_message_record(this.active_message_trackers),
      pending_permission_count: this.pending_permission_count,
      is_server_generating: this.is_server_generating,
      is_loading: phase !== 'idle',
    };
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

    if (
      this.open_round_ids.size > 0 ||
      this.active_round_ids.size > 0 ||
      this.active_message_trackers.size > 0 ||
      this.is_server_generating
    ) {
      return 'running';
    }

    return 'idle';
  }

  private reconcile_server_generating_flag(): void {
    if (
      this.pending_round_ids.size > 0 ||
      this.open_round_ids.size > 0 ||
      this.active_round_ids.size > 0 ||
      this.active_message_trackers.size > 0 ||
      this.pending_permission_count > 0
    ) {
      return;
    }

    this.is_server_generating = false;
  }
}

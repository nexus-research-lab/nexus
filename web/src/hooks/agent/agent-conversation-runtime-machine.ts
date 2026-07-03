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
import { areRuntimeSnapshotsEqual } from './conversation-runtime-state';

export interface ActiveMessageTracker {
  roundId: string;
  status: AssistantMessageStatus;
}

export interface AgentConversationRuntimeSnapshot {
  phase: AgentConversationRuntimePhase;
  sendingRoundIds: string[];
  runningRoundIds: string[];
  terminalRoundIds: string[];
  liveRoundIds: string[];
  activeMessages: Record<string, ActiveMessageTracker>;
  pendingPermissionCount: number;
  isLoading: boolean;
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
  private chatType: AgentConversationChatType;

  private sendingRoundIds = new Set<string>();

  private runningRoundIds = new Set<string>();

  private terminalRoundIds = new Set<string>();

  private activeMessageTrackers = new Map<string, ActiveMessageTracker>();

  private pendingPermissionCount = 0;

  private listeners = new Set<() => void>();

  private snapshotCache: AgentConversationRuntimeSnapshot | null = null;

  public constructor(chatType: AgentConversationChatType) {
    this.chatType = chatType;
  }

  // useSyncExternalStore 订阅入口，返回取消订阅函数。
  public subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  // 每次状态变更后重算快照，只在真实变化时通知订阅者。
  public emit(): void {
    const next = this.computeSnapshot();
    if (this.snapshotCache && areRuntimeSnapshotsEqual(this.snapshotCache, next)) {
      return;
    }
    this.snapshotCache = next;
    for (const listener of this.listeners) {
      listener();
    }
  }

  public setChatType(chatType: AgentConversationChatType): void {
    this.chatType = chatType;
  }

  public reset(): void {
    this.sendingRoundIds.clear();
    this.runningRoundIds.clear();
    this.terminalRoundIds.clear();
    this.activeMessageTrackers.clear();
    this.pendingPermissionCount = 0;
  }

  public trackOutboundRound(roundId: string): void {
    this.terminalRoundIds.delete(roundId);
    this.sendingRoundIds.add(roundId);
  }

  public clearRound(
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

    for (const trackedRoundId of [...this.sendingRoundIds]) {
      if (shouldClearRound(trackedRoundId)) {
        this.sendingRoundIds.delete(trackedRoundId);
      }
    }

    for (const trackedRoundId of [...this.runningRoundIds]) {
      if (shouldClearRound(trackedRoundId)) {
        this.runningRoundIds.delete(trackedRoundId);
      }
    }

    for (const trackedRoundId of [...this.terminalRoundIds]) {
      if (shouldClearRound(trackedRoundId)) {
        this.terminalRoundIds.delete(trackedRoundId);
      }
    }

    for (const [messageId, tracker] of this.activeMessageTrackers.entries()) {
      if (shouldClearRound(tracker.roundId)) {
        this.activeMessageTrackers.delete(messageId);
      }
    }
  }

  public updateMessageStatus(
    messageId: string,
    status: AssistantMessageStatus,
    roundId?: string | null,
  ): void {
    const currentTracker = this.activeMessageTrackers.get(messageId);
    const resolvedRoundId = roundId ?? currentTracker?.roundId ?? '';
    if (resolvedRoundId && this.isRoundTerminal(resolvedRoundId)) {
      this.activeMessageTrackers.delete(messageId);
      return;
    }

    if (isTerminalAssistantStatus(status)) {
      this.activeMessageTrackers.delete(messageId);
      return;
    }

    this.activeMessageTrackers.set(messageId, {
      roundId: resolvedRoundId,
      status,
    });
  }

  public trackChatAck(ack: ChatAckData): void {
    this.sendingRoundIds.delete(ack.round_id);
    const pendingCount = ack.pending?.length ?? 0;

    for (const slot of ack.pending ?? []) {
      const agentRoundId = (
        slot.round_id ||
        (pendingCount > 1 ? `${ack.round_id}:${slot.agent_id}` : ack.round_id)
      );
      if (this.isRoundTerminal(agentRoundId)) {
        continue;
      }
      this.activeMessageTrackers.set(slot.msg_id, {
        roundId: agentRoundId,
        status: slot.status ?? 'pending',
      });
    }
  }

  public trackAssistantMessage(message: AssistantMessage): void {
    if (this.isRoundTerminal(message.round_id)) {
      this.activeMessageTrackers.delete(message.message_id);
      return;
    }

    if (hasTerminalAssistantProjection(message)) {
      this.activeMessageTrackers.delete(message.message_id);
      return;
    }

    this.activeMessageTrackers.set(message.message_id, {
      roundId: message.round_id,
      status: message.stream_status ?? 'streaming',
    });
  }

  public trackRoundStatus(
    roundId: string,
    status: RoundLifecycleStatus,
  ): void {
    if (status === 'running') {
      this.sendingRoundIds.delete(roundId);
      this.terminalRoundIds.delete(roundId);
      this.runningRoundIds.add(roundId);
      return;
    }

    this.terminalRoundIds.add(roundId);
    this.clearRound(roundId, this.chatType === 'group');
  }

  public syncRunningRounds(roundIds: string[]): void {
    const nextRunningRoundIds = new Set(
      roundIds
        .map((roundId) => roundId.trim())
        .filter(Boolean),
    );

    this.runningRoundIds = nextRunningRoundIds;
    for (const roundId of nextRunningRoundIds) {
      this.sendingRoundIds.delete(roundId);
      this.terminalRoundIds.delete(roundId);
    }
  }

  public setPendingPermissionCount(count: number): void {
    this.pendingPermissionCount = Math.max(0, count);
  }

  public reconcileFromSnapshot(messages: Message[]): void {
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
    for (const [messageId, tracker] of this.activeMessageTrackers.entries()) {
      if (terminalMessageIds.has(messageId) || this.isRoundTerminal(tracker.roundId)) {
        continue;
      }
      nextTrackers.set(messageId, tracker);
    }

    if (this.chatType !== 'group') {
      for (const message of messages) {
        if (message.role !== 'assistant') {
          continue;
        }
        if (
          hasTerminalAssistantProjection(message) ||
          this.isRoundTerminal(message.round_id)
        ) {
          continue;
        }
        nextTrackers.set(message.message_id, {
          roundId: message.round_id,
          status: message.stream_status ?? 'streaming',
        });
      }
    }

    this.activeMessageTrackers = nextTrackers;
  }

  // useSyncExternalStore 的 getSnapshot，emit 之间保持引用稳定。
  public snapshot(): AgentConversationRuntimeSnapshot {
    return (this.snapshotCache ??= this.computeSnapshot());
  }

  private computeSnapshot(): AgentConversationRuntimeSnapshot {
    const phase = this.resolvePhase();
    const liveRoundIds = new Set<string>([
      ...this.sendingRoundIds,
      ...this.runningRoundIds,
    ]);
    for (const tracker of this.activeMessageTrackers.values()) {
      if (tracker.roundId) {
        liveRoundIds.add(tracker.roundId);
      }
    }
    return {
      phase,
      sendingRoundIds: [...this.sendingRoundIds],
      runningRoundIds: [...this.runningRoundIds],
      terminalRoundIds: [...this.terminalRoundIds],
      liveRoundIds: [...liveRoundIds],
      activeMessages: buildActiveMessageRecord(this.activeMessageTrackers),
      pendingPermissionCount: this.pendingPermissionCount,
      isLoading: phase !== 'idle',
    };
  }

  public isRoundTerminal(roundId: string): boolean {
    if (!roundId) {
      return false;
    }
    if (this.terminalRoundIds.has(roundId)) {
      return true;
    }
    if (this.chatType !== 'group') {
      return false;
    }
    for (const terminalRoundId of this.terminalRoundIds) {
      if (roundId.startsWith(`${terminalRoundId}:`)) {
        return true;
      }
    }
    return false;
  }

  private resolvePhase(): AgentConversationRuntimePhase {
    if (this.pendingPermissionCount > 0) {
      return 'awaiting_permission';
    }

    for (const tracker of this.activeMessageTrackers.values()) {
      if (tracker.status === 'streaming') {
        return 'streaming';
      }
    }

    if (this.sendingRoundIds.size > 0) {
      return 'sending';
    }

    if (this.runningRoundIds.size > 0 || this.activeMessageTrackers.size > 0) {
      return 'running';
    }

    return 'idle';
  }
}

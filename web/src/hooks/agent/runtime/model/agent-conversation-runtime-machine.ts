import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type { ChatAckData } from "@/types/conversation/message/event";
import type { RoundLifecycleStatus } from "@/types/conversation/message/event";
import type {
  AgentConversationChatType,
  AgentConversationRuntimePhase,
} from "@/types/agent/agent-conversation";

import {
  areRuntimeSnapshotsEqual,
  type AgentConversationRuntimeSnapshot,
} from "./conversation-runtime-state";

interface ActiveMessageTracker {
  roundId: string;
  status: AssistantMessageStatus;
}

function isTerminalAssistantStatus(status?: AssistantMessageStatus): boolean {
  return status === "done" || status === "cancelled" || status === "error";
}

function hasTerminalAssistantProjection(message: AssistantMessage): boolean {
  return Boolean(message.result_summary)
    || Boolean(message.stop_reason)
    || isTerminalAssistantStatus(message.stream_status);
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

  /** 发送中状态按 client_request_id 追踪，ack 后转为 canonical round。 */
  public trackOutboundRequest(clientRequestId: string): void {
    this.sendingRoundIds.add(clientRequestId);
  }

  public clearOutboundRequest(clientRequestId?: string | null): void {
    if (clientRequestId) {
      this.sendingRoundIds.delete(clientRequestId);
    }
  }

  public updateMessageStatus(
    messageId: string,
    status: AssistantMessageStatus,
    roundId?: string | null,
  ): void {
    const currentTracker = this.activeMessageTrackers.get(messageId);
    const resolvedRoundId = roundId ?? currentTracker?.roundId ?? "";
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
    this.sendingRoundIds.delete(ack.client_request_id);
    this.terminalRoundIds.delete(ack.round_id);

    for (const slot of ack.pending) {
      if (this.isRoundTerminal(ack.round_id)) {
        continue;
      }
      this.activeMessageTrackers.set(slot.msg_id, {
        roundId: ack.round_id,
        status: slot.status,
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
      status: message.stream_status ?? "streaming",
    });
  }

  public trackRoundStatus(
    roundId: string,
    status: RoundLifecycleStatus,
  ): void {
    if (status === "running") {
      this.sendingRoundIds.delete(roundId);
      this.terminalRoundIds.delete(roundId);
      this.runningRoundIds.add(roundId);
      return;
    }

    this.terminalRoundIds.add(roundId);
    this.sendingRoundIds.delete(roundId);
    this.runningRoundIds.delete(roundId);
    for (const [messageId, tracker] of this.activeMessageTrackers.entries()) {
      if (tracker.roundId === roundId) {
        this.activeMessageTrackers.delete(messageId);
      }
    }
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
      if (message.role !== "assistant") {
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

    if (this.chatType !== "group") {
      for (const message of messages) {
        if (message.role !== "assistant") {
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
          status: message.stream_status ?? "streaming",
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
      terminalRoundIds: [...this.terminalRoundIds],
      liveRoundIds: [...liveRoundIds],
      isLoading: phase !== "idle",
    };
  }

  public isRoundTerminal(roundId: string): boolean {
    return Boolean(roundId) && this.terminalRoundIds.has(roundId);
  }

  private resolvePhase(): AgentConversationRuntimePhase {
    if (this.pendingPermissionCount > 0) {
      return "awaiting_permission";
    }

    for (const tracker of this.activeMessageTrackers.values()) {
      if (tracker.status === "streaming") {
        return "streaming";
      }
    }

    if (this.sendingRoundIds.size > 0) {
      return "sending";
    }

    if (this.runningRoundIds.size > 0 || this.activeMessageTrackers.size > 0) {
      return "running";
    }

    return "idle";
  }
}

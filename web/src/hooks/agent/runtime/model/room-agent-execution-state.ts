/**
 * INPUT: 当前 Session 的 Room execution 顺序锚点，以及 permission / slot / message / lifecycle 证据。
 * OUTPUT: keyed by root round + agent_round 的 canonical 初始顺序、首次可见锚点和 acknowledged 非交互 tombstone。
 * POS: Room execution shell 连续性的纯状态转换；React 状态与协议发送只负责调用。
 */
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type {
  AgentRoundStatusEventPayload,
  RoundLifecycleStatus,
  StreamMessage,
} from "@/types/conversation/message/event";
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

interface RoomExecutionIdentity {
  agentId: string;
  agentRoundId: string;
  roundId: string;
}

interface RoomExecutionEvidence extends RoomExecutionIdentity {
  firstSeenAt: number;
  handoffId?: string;
  phase: RoomAgentExecutionState["phase"];
  preferredDisplayOrder?: number;
  status: AssistantMessageStatus;
}

const TERMINAL_STATUSES = new Set<AssistantMessageStatus>([
  "cancelled",
  "done",
  "error",
]);
const TERMINAL_STATUS_PRIORITY: Partial<Record<AssistantMessageStatus, number>> = {
  cancelled: 2,
  done: 1,
  error: 3,
};
const ROUND_STATUS: Record<RoundLifecycleStatus, AssistantMessageStatus> = {
  error: "error",
  finished: "done",
  interrupted: "cancelled",
  running: "streaming",
};
const ROOM_DISPLAY_ORDER_SCALE = 1_000;

function buildExecutionKey(
  roundId: string,
  agentRoundId: string,
): string {
  return `${roundId}\u001f${agentRoundId}`;
}

function executionKey(state: RoomAgentExecutionState): string {
  return buildExecutionKey(state.round_id, state.agent_round_id);
}

function normalizeIdentity(
  roundId?: string | null,
  agentId?: string | null,
  agentRoundId?: string | null,
): RoomExecutionIdentity | null {
  const normalizedRoundId = roundId?.trim();
  const normalizedAgentId = agentId?.trim();
  const normalizedAgentRoundId = agentRoundId?.trim();
  return normalizedRoundId && normalizedAgentId && normalizedAgentRoundId
    ? {
        agentId: normalizedAgentId,
        agentRoundId: normalizedAgentRoundId,
        roundId: normalizedRoundId,
      }
    : null;
}

function nextDisplayOrder(
  states: readonly RoomAgentExecutionState[],
  roundId: string,
): number {
  return states.reduce(
    (next, state) => state.round_id === roundId
      ? Math.max(next, state.display_order + 1)
      : next,
    0,
  );
}

function mergeMonotonicStatus(
  current: AssistantMessageStatus,
  incoming: AssistantMessageStatus,
): AssistantMessageStatus {
  if (!TERMINAL_STATUSES.has(current)) {
    return incoming;
  }
  if (!TERMINAL_STATUSES.has(incoming)) {
    return current;
  }
  return (TERMINAL_STATUS_PRIORITY[incoming] ?? 0)
      > (TERMINAL_STATUS_PRIORITY[current] ?? 0)
    ? incoming
    : current;
}

function mergeEvidence(
  states: RoomAgentExecutionState[],
  evidence: RoomExecutionEvidence,
): RoomAgentExecutionState[] {
  const key = buildExecutionKey(evidence.roundId, evidence.agentRoundId);
  const index = states.findIndex((state) => executionKey(state) === key);
  if (index < 0) {
    const appendOrder = nextDisplayOrder(states, evidence.roundId);
    const preferredOrder = evidence.preferredDisplayOrder;
    return [
      ...states,
      {
        agent_id: evidence.agentId,
        agent_round_id: evidence.agentRoundId,
        display_order:
          typeof preferredOrder === "number"
          && Number.isFinite(preferredOrder)
          && preferredOrder >= appendOrder
            ? preferredOrder
            : appendOrder,
        first_seen_at: evidence.firstSeenAt,
        ...(evidence.handoffId ? { handoff_id: evidence.handoffId } : {}),
        phase: evidence.phase,
        round_id: evidence.roundId,
        status: evidence.status,
      },
    ];
  }

  const current = states[index];
  const nextPhase = resolveNextPhase(current.phase, evidence.phase);
  const nextStatus = mergeMonotonicStatus(current.status, evidence.status);
  const nextHandoffId = evidence.handoffId ?? current.handoff_id;
  if (
    current.agent_id === evidence.agentId
    && current.handoff_id === nextHandoffId
    && current.phase === nextPhase
    && current.status === nextStatus
  ) {
    return states;
  }
  const next = [...states];
  next[index] = {
    ...current,
    agent_id: evidence.agentId,
    ...(nextHandoffId ? { handoff_id: nextHandoffId } : {}),
    phase: nextPhase,
    status: nextStatus,
  };
  return next;
}

function resolveNextPhase(
  current: RoomAgentExecutionState["phase"],
  incoming: RoomAgentExecutionState["phase"],
): RoomAgentExecutionState["phase"] {
  if (current === "terminal" || incoming === "terminal") {
    return "terminal";
  }
  if (current === "active" || incoming === "active") {
    return "active";
  }
  return incoming;
}

function syncEvidence(
  current: RoomAgentExecutionState[],
  evidence: RoomExecutionEvidence[],
): RoomAgentExecutionState[] {
  const evidenceByRound = new Map<string, RoomExecutionEvidence[]>();
  for (const item of evidence) {
    const roundEvidence = evidenceByRound.get(item.roundId) ?? [];
    roundEvidence.push(item);
    evidenceByRound.set(item.roundId, roundEvidence);
  }

  let next = current;
  for (const [roundId, roundEvidence] of evidenceByRound) {
    const hasObservedRound = current.some((state) => state.round_id === roundId);
    const orderedEvidence = hasObservedRound
      ? roundEvidence
      : [...roundEvidence].sort(compareInitialEvidenceOrder);
    next = orderedEvidence.reduce(mergeEvidence, next);
  }
  return next;
}

function compareInitialEvidenceOrder(
  left: RoomExecutionEvidence,
  right: RoomExecutionEvidence,
): number {
  const leftOrder = left.preferredDisplayOrder;
  const rightOrder = right.preferredDisplayOrder;
  if (leftOrder !== undefined && rightOrder !== undefined) {
    return leftOrder - rightOrder || left.firstSeenAt - right.firstSeenAt;
  }
  if (leftOrder !== undefined) {
    return -1;
  }
  if (rightOrder !== undefined) {
    return 1;
  }
  return left.firstSeenAt - right.firstSeenAt;
}

function permissionEvidence(
  permission: PendingPermission,
  observedAt: number,
): RoomExecutionEvidence | null {
  const identity = normalizeIdentity(
    permission.round_id,
    permission.agent_id,
    permission.agent_round_id,
  );
  return identity
    ? {
        ...identity,
        firstSeenAt: observedAt,
        phase: "pending_permission",
        status: "pending",
      }
    : null;
}

/**
 * 当前 pending 集合既注册 permission-first 顺序，也清理未被确认便消失的请求。
 */
export function syncRoomAgentExecutionsFromPermissions(
  current: RoomAgentExecutionState[],
  permissions: PendingPermission[],
  observedAt: number = Date.now(),
): RoomAgentExecutionState[] {
  const evidence = permissions.flatMap((permission) => {
    const item = permissionEvidence(permission, observedAt);
    return item ? [item] : [];
  });
  const observed = syncEvidence(current, evidence);
  const pendingKeys = new Set(evidence.map((item) => (
    buildExecutionKey(item.roundId, item.agentRoundId)
  )));
  const next = observed.filter((state) => (
    state.phase !== "pending_permission"
    || pendingKeys.has(executionKey(state))
  ));
  return next.length === observed.length ? observed : next;
}

export function syncRoomAgentExecutionsFromSlots(
  current: RoomAgentExecutionState[],
  slots: RoomPendingAgentSlotState[],
): RoomAgentExecutionState[] {
  return syncEvidence(current, slots.flatMap((slot, fallbackOrder) => {
    const identity = normalizeIdentity(
      slot.round_id,
      slot.agent_id,
      slot.agent_round_id,
    );
    return identity
      ? [{
          ...identity,
          firstSeenAt: slot.timestamp,
          ...(slot.handoff_id?.trim()
            ? { handoffId: slot.handoff_id.trim() }
            : {}),
          phase: TERMINAL_STATUSES.has(slot.status) ? "terminal" : "active",
          preferredDisplayOrder: resolveSlotDisplayOrder(slot, fallbackOrder),
          status: slot.status,
        }]
      : [];
  }));
}

function resolveSlotDisplayOrder(
  slot: RoomPendingAgentSlotState,
  fallbackOrder: number,
): number {
  if (Number.isFinite(slot.timestamp) && slot.timestamp > 0) {
    return (
      Math.trunc(slot.timestamp) * ROOM_DISPLAY_ORDER_SCALE
      + Math.max(slot.index ?? 0, 0)
    );
  }
  return slot.index ?? fallbackOrder;
}

function resolveMessageStatus(
  message: AssistantMessage,
): AssistantMessageStatus {
  if (!message.result_summary && message.stop_reason === "tool_use") {
    return "streaming";
  }
  if (
    message.result_summary
    || message.is_complete
    || message.stop_reason
  ) {
    if (
      message.result_summary?.is_error
      || message.result_summary?.subtype === "error"
    ) {
      return "error";
    }
    return message.result_summary?.subtype === "interrupted"
      ? "cancelled"
      : "done";
  }
  return message.stream_status ?? "streaming";
}

export function syncRoomAgentExecutionsFromMessages(
  current: RoomAgentExecutionState[],
  messages: Message[],
): RoomAgentExecutionState[] {
  return syncEvidence(current, messages.flatMap((message) => {
    if (message.role !== "assistant") {
      return [];
    }
    const identity = normalizeIdentity(
      message.round_id,
      message.agent_id,
      message.agent_round_id,
    );
    if (!identity) {
      return [];
    }
    const status = resolveMessageStatus(message);
    return [{
      ...identity,
      firstSeenAt: message.timestamp,
      phase: TERMINAL_STATUSES.has(status) ? "terminal" : "active",
      ...(typeof message.display_order === "number"
        && Number.isFinite(message.display_order)
        ? { preferredDisplayOrder: message.display_order }
        : {}),
      status,
    }];
  }));
}

export function syncRoomAgentExecutionFromStream(
  current: RoomAgentExecutionState[],
  stream: StreamMessage,
): RoomAgentExecutionState[] {
  const identity = normalizeIdentity(
    stream.round_id,
    stream.agent_id,
    stream.agent_round_id,
  );
  if (!identity) {
    return current;
  }
  // message_stop 只收口一个 Assistant turn；tool_use 后同一 agent_round
  // 仍会继续。execution 终态只由 agent/root lifecycle 或 durable terminal
  // message 证据建立。
  const status: AssistantMessageStatus = "streaming";
  return mergeEvidence(current, {
    ...identity,
    firstSeenAt: stream.timestamp,
    phase: "active",
    status,
  });
}

export function acknowledgeRoomAgentExecutionPermission(
  current: RoomAgentExecutionState[],
  permission: PendingPermission,
  observedAt: number = Date.now(),
): RoomAgentExecutionState[] {
  const evidence = permissionEvidence(permission, observedAt);
  if (!evidence) {
    return current;
  }
  const observed = mergeEvidence(current, evidence);
  const key = buildExecutionKey(evidence.roundId, evidence.agentRoundId);
  const index = observed.findIndex((state) => executionKey(state) === key);
  if (index < 0 || observed[index].phase !== "pending_permission") {
    return observed;
  }
  const next = [...observed];
  next[index] = { ...observed[index], phase: "acknowledged" };
  return next;
}

export function applyRoomAgentExecutionStatus(
  current: RoomAgentExecutionState[],
  payload: AgentRoundStatusEventPayload,
  observedAt: number = Date.now(),
): RoomAgentExecutionState[] {
  const identity = normalizeIdentity(
    payload.round_id,
    payload.agent_id,
    payload.agent_round_id,
  );
  if (!identity) {
    return current;
  }
  const status = ROUND_STATUS[payload.status];
  return mergeEvidence(current, {
    ...identity,
    firstSeenAt: observedAt,
    phase: payload.is_terminal ? "terminal" : "active",
    status,
  });
}

export function applyRoomExecutionRootStatus(
  current: RoomAgentExecutionState[],
  roundId: string,
  status: RoundLifecycleStatus,
): RoomAgentExecutionState[] {
  if (status === "running") {
    return current;
  }
  const normalizedRoundId = roundId.trim();
  const nextStatus = ROUND_STATUS[status];
  let changed = false;
  const next = current.map((state) => {
    if (state.round_id !== normalizedRoundId || state.phase === "terminal") {
      return state;
    }
    changed = true;
    return { ...state, phase: "terminal" as const, status: nextStatus };
  });
  return changed ? next : current;
}

export function removeRoomAgentExecutionRound(
  current: RoomAgentExecutionState[],
  roundId: string,
): RoomAgentExecutionState[] {
  const normalizedRoundId = roundId.trim();
  const next = current.filter((state) => state.round_id !== normalizedRoundId);
  return next.length === current.length ? current : next;
}

export function stopRoomAgentExecutions(
  current: RoomAgentExecutionState[],
): RoomAgentExecutionState[] {
  let changed = false;
  const next = current.map((state) => {
    if (state.phase === "terminal") {
      return state;
    }
    changed = true;
    return {
      ...state,
      phase: "terminal" as const,
      status: "cancelled" as const,
    };
  });
  return changed ? next : current;
}

export function isVisibleRoomAgentExecutionState(
  state: RoomAgentExecutionState,
): boolean {
  return state.phase !== "pending_permission";
}

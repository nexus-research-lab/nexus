/**
 * INPUT: Room realtime final message、Agent public mention queue、pending slot 与 execution 锚点。
 * OUTPUT: 以 handoff_id 为唯一键且 responded > active > starting > queued > preparing 的单调 mention 状态。
 * POS: Group Chat 面板的 public handoff 纯投影，不持有 React 状态或 feed 节点。
 */
import type {
  AgentHandoffPhase,
  AgentHandoffStatusMap,
} from "@/features/conversation/shared/message/agent-handoff-status-context";
import { normalizePublicHandoffReply } from "@/features/conversation/shared/message/public-handoff-reply-model";
import type {
  InputQueueItem,
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";

interface ProjectRoomAgentHandoffStatusesOptions {
  executionStates: readonly RoomAgentExecutionState[];
  inputQueueItems: readonly InputQueueItem[];
  messages: readonly Message[];
  pendingSlots: readonly RoomPendingAgentSlotState[];
}

interface SourceHandoffIdentity {
  sourceAgentId: string;
  targetAgentId: string;
}

const HANDOFF_PHASE_PRIORITY: Record<AgentHandoffPhase, number> = {
  preparing: 1,
  queued: 2,
  starting: 3,
  active: 4,
  responded: 5,
};

export function projectRoomAgentHandoffStatuses({
  executionStates,
  inputQueueItems,
  messages,
  pendingSlots,
}: ProjectRoomAgentHandoffStatusesOptions): AgentHandoffStatusMap {
  const statuses: Record<string, AgentHandoffPhase> = {};
  const setPhase = (handoffId: string | null | undefined, phase: AgentHandoffPhase) => {
    const normalizedHandoffId = handoffId?.trim();
    if (!normalizedHandoffId) {
      return;
    }
    const current = statuses[normalizedHandoffId];
    if (
      !current
      || HANDOFF_PHASE_PRIORITY[phase] > HANDOFF_PHASE_PRIORITY[current]
    ) {
      statuses[normalizedHandoffId] = phase;
    }
  };

  const sourceHandoffs = indexSourceHandoffs(messages);
  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    const reply = normalizePublicHandoffReply(message.handoff_reply);
    const source = reply
      ? sourceHandoffs.get(reply.source_message_id)?.get(reply.handoff_id)
      : null;
    if (
      reply
      && source?.sourceAgentId === reply.source_agent_id
      && source.targetAgentId === message.agent_id
    ) {
      setPhase(reply.handoff_id, "responded");
    }
  }

  for (const message of messages) {
    if (!isRealtimeFinalAssistantMessage(message)) {
      continue;
    }
    for (const mention of message.agent_mentions ?? []) {
      setPhase(mention.handoff_id, "preparing");
    }
  }
  for (const item of inputQueueItems) {
    if (item.source === "agent_public_mention") {
      setPhase(item.handoff_id, "queued");
    }
  }
  for (const slot of pendingSlots) {
    setPhase(
      slot.handoff_id,
      slot.status === "pending" ? "starting" : "active",
    );
  }
  for (const execution of executionStates) {
    setPhase(
      execution.handoff_id,
      execution.phase === "pending_permission"
        || execution.phase === "acknowledged"
        ? "starting"
        : "active",
    );
  }

  return statuses;
}

function indexSourceHandoffs(
  messages: readonly Message[],
): ReadonlyMap<string, ReadonlyMap<string, SourceHandoffIdentity>> {
  const indexed = new Map<
    string,
    Map<string, SourceHandoffIdentity>
  >();
  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    for (const mention of message.agent_mentions ?? []) {
      const handoffId = mention.handoff_id?.trim();
      if (!handoffId) {
        continue;
      }
      const handoffs = indexed.get(message.message_id)
        ?? new Map<string, SourceHandoffIdentity>();
      handoffs.set(handoffId, {
        sourceAgentId: message.agent_id.trim(),
        targetAgentId: mention.agent_id.trim(),
      });
      indexed.set(message.message_id, handoffs);
    }
  }
  return indexed;
}

function isRealtimeFinalAssistantMessage(
  message: Message,
): message is AssistantMessage {
  if (
    message.delivery_mode !== "durable"
    || !isSuccessfulFinalAssistantMessage(message)
  ) {
    return false;
  }
  return true;
}

function isSuccessfulFinalAssistantMessage(
  message: Message,
): message is AssistantMessage {
  if (
    message.role !== "assistant"
    || message.stream_status === "cancelled"
    || message.stream_status === "error"
    || message.result_summary?.is_error
    || message.result_summary?.subtype === "error"
    || message.result_summary?.subtype === "interrupted"
  ) {
    return false;
  }
  return (
    message.is_complete === true
    || message.stream_status === "done"
    || message.result_summary?.subtype === "success"
  );
}

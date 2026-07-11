/**
 * 会话运行事件及流式消息载荷。
 *
 * WebSocket 信封和通用事件数据直接使用生成协议，避免手写镜像漂移。
 */

import type { SessionId } from "../../system/sdk";
import type {
  ChatAckData as ProtocolChatAckData,
  ChatAckPendingSlot as ProtocolChatAckPendingSlot,
} from "../../generated/protocol";
import type { ContentBlock } from "./content";
import type {
  AssistantMessage,
  AssistantMessageStatus,
  ResultSummary,
  Usage,
} from "./entity";

type ChatAckPendingSlot = Omit<ProtocolChatAckPendingSlot, "status"> & {
  status: AssistantMessageStatus;
};

export interface ChatAckData extends Omit<ProtocolChatAckData, "pending"> {
  pending: ChatAckPendingSlot[];
}

export type RoundLifecycleStatus =
  | "running"
  | "finished"
  | "interrupted"
  | "error";

export interface RoundStatusEventPayload {
  round_id: string;
  status: RoundLifecycleStatus;
  is_terminal: boolean;
  result_subtype?: ResultSummary["subtype"] | null;
}

export interface AgentRoundStatusEventPayload {
  round_id: string;
  agent_round_id: string;
  agent_id: string;
  status: RoundLifecycleStatus;
  is_terminal: boolean;
}

type StreamMessageType =
  | "message_start"
  | "content_block_start"
  | "content_block_delta"
  | "message_delta"
  | "message_stop";

export interface StreamMessage {
  message_id: string;
  session_key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id: string;
  round_id: string;
  session_id?: SessionId;
  type: StreamMessageType;
  index?: number;
  content_block?: ContentBlock;
  message?: {
    model?: string;
    stop_reason?: AssistantMessage["stop_reason"];
  };
  usage?: Usage;
  timestamp: number;
}

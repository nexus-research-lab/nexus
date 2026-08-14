/**
 * INPUT: 后端持久化消息、实时 message 事件与结果摘要字段。
 * OUTPUT: DM / Room 共用的 user、assistant、system 消息实体契约。
 * POS: 前端会话消息协议的类型真相源。
 */

import type {
  DeliveryMode,
  PublicHandoffReply as ProtocolPublicHandoffReply,
} from "../../generated/protocol";
import type { SessionId } from "../../system/sdk";
import type { MessageAttachment } from "./attachment";
import type { ContentBlock } from "./content";

type MessageRole = "user" | "assistant" | "system";

export interface AgentMention {
  agent_id: string;
  label: string;
  content_block_index: number;
  start_rune: number;
  end_rune: number;
  handoff_id?: string;
}

/**
 * 宿主确认一次 public handoff 的目标 Agent 已给出最终回复。
 * 这是展示/恢复事实，不是可点击 mention，也不授予任何唤醒能力。
 */
export type PublicHandoffReply = ProtocolPublicHandoffReply;

interface BaseMessage {
  message_id: string;
  session_key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id: string;
  round_id: string;
  /** 消息被运行中轮次消费前所属的原始 round；`round_id` 是消费后的根轮次。 */
  source_round_id?: string | null;
  /** Room slot / Agent 私有执行轮次 id；`round_id` 始终表示根轮次。 */
  agent_round_id?: string | null;
  session_id?: SessionId;
  parent_id?: string;
  /** Claude Code / nxs 的原生子执行父工具调用标识。 */
  parent_tool_use_id?: string | null;
  role: MessageRole;
  timestamp: number;
  display_order?: number;
  /** Nexus durable control records use metadata.subtype without becoming model input. */
  metadata?: Record<string, unknown>;
  /**
   * 实时投影的恢复边界：
   * durable 可恢复，ephemeral 随 round 收口，transient 仅保留在当前打开的时间线。
   */
  delivery_mode?: DeliveryMode;
  /** 内部续跑输入不代表用户开始了会话。 */
  hidden_from_user?: boolean;
  is_synthetic?: boolean;
}

export interface UserMessage extends BaseMessage {
  role: "user";
  content: string;
  /** 持久化受理身份，用于原子替换本地 optimistic 消息。 */
  client_message_id?: string;
  agent_mentions?: AgentMention[];
  delivery_policy?: "queue" | "guide" | "interrupt" | "auto";
  /** Room resolved targets；guided user 据此贴近实际消费它的 Agent。 */
  target_agent_ids?: string[];
  attachments?: MessageAttachment[];
}

export interface Usage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  [key: string]: unknown;
}

export interface ResultSummary {
  message_id?: string;
  timestamp?: number;
  subtype: "success" | "error" | "interrupted";
  duration_ms: number;
  duration_api_ms: number;
  num_turns: number;
  total_cost_usd?: number;
  usage?: Usage;
  model_usage?: Record<string, unknown>;
  result?: string;
  structured_output?: unknown;
  fast_mode_state?: "off" | "cooldown" | "on" | string;
  runtime_subtype?: string;
  errors?: string[];
  /** 已知 Provider 内容安全拦截统一为 content_filtered，其余值保留 runtime 原因。 */
  terminal_reason?: string;
  stop_reason?: string;
  is_error: boolean;
}

export interface RecalledMemoryReference {
  description: string;
  name: string;
}

export type AssistantMessageStatus =
  | "pending"
  | "streaming"
  | "done"
  | "cancelled"
  | "error";

export interface GoalCompletionReceipt {
  /** 仅用于宿主精确绑定和历史合并，界面不得展示。 */
  goal_id: string;
  /** 仅用于宿主精确绑定和历史合并，界面不得展示。 */
  round_id: string;
  /** 缺失表示未知；不能投影成 0 或“不可用”。 */
  time_used_seconds?: number;
  /** 缺失表示 provider 用量尚未成为权威终值。 */
  actual_tokens?: number;
}

export interface AssistantMessage extends BaseMessage {
  role: "assistant";
  content: ContentBlock[];
  is_complete?: boolean;
  stop_reason?: "end_turn" | "max_tokens" | "stop_sequence" | "tool_use";
  model?: string;
  usage?: Usage;
  result_summary?: ResultSummary;
  goal_completion_receipt?: GoalCompletionReceipt;
  /** 本轮实际注入模型的长期记忆摘要；正文和绝对路径不进入消息。 */
  recalled_memories?: RecalledMemoryReference[];
  /** 服务端解析出的可点击 Agent mention span。 */
  agent_mentions?: AgentMention[];
  /** 宿主签发的 public handoff 回执；独立于正文中的显式 Agent mention。 */
  handoff_reply?: PublicHandoffReply;
  /** 宿主生成消息的结构化来源；展示层不得依赖正文前缀识别定时任务。 */
  metadata?: Record<string, unknown>;
  /** 前端流式状态，不属于后端持久化消息字段。 */
  stream_status?: AssistantMessageStatus;
}

interface SystemMessageMetadata extends Record<string, unknown> {
  subtype?: string;
  /** guided_input 通过 metadata 保存被消费的原始 round。 */
  source_round_id?: string | null;
  attempt?: number;
  max_retries?: number;
  retry_delay_ms?: number;
  error_status?: string | number | null;
  error?: string | null;
}

export interface SystemMessage extends BaseMessage {
  role: "system";
  content: string;
  metadata?: SystemMessageMetadata;
}

export type Message = UserMessage | AssistantMessage | SystemMessage;

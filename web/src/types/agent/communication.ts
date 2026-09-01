// INPUT: 联系人联络读取链路已经确认的失败阶段和快照新鲜度。
// OUTPUT: 控制器与纯视图共享的读取失败合同。
// POS: 只描述读取事实；不包含请求身份、自动重试或写操作结果。
export type AgentCommunicationReadFailureKind =
  | "channel"
  | "directory"
  | "history"
  | "messages";

export interface AgentCommunicationReadFailure {
  kind: AgentCommunicationReadFailureKind;
  stale: boolean;
}

export type AgentCommunicationMutationKind =
  | "add_contact"
  | "create_conversation"
  | "remove_contact"
  | "send_message";

export interface AgentCommunicationMutationFailure {
  blocksRepeat: boolean;
  effect: "accepted" | "committed" | "not_applied" | "unknown";
  intentKey: string;
  kind: AgentCommunicationMutationKind;
  message: string;
  targetAgentId?: string;
}

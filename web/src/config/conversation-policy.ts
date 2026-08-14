const ENABLE_STRICT_MODE = false;
const MESSAGE_HISTORY_ROUND_PAGE_SIZE = 3;
// 与后端请求 ACK 超时保持一致，避免客户端先于服务端判定发送失败。
const MESSAGE_SEND_ACK_TIMEOUT_MS = 10_000;
// 后端 detached Goal command 最长 15 秒；额外 5 秒覆盖 ACK 投递与调度。
const GOAL_REQUEST_ACCEPTANCE_TIMEOUT_MS = 20_000;

export function isStrictModeEnabled(): boolean {
  return ENABLE_STRICT_MODE;
}

export function getMessageHistoryRoundPageSize(): number {
  return MESSAGE_HISTORY_ROUND_PAGE_SIZE;
}

export function getMessageSendAckTimeoutMs(): number {
  return MESSAGE_SEND_ACK_TIMEOUT_MS;
}

export function getGoalRequestAcceptanceTimeoutMs(): number {
  return GOAL_REQUEST_ACCEPTANCE_TIMEOUT_MS;
}

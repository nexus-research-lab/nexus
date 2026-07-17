const ENABLE_STRICT_MODE = false;
const MESSAGE_HISTORY_ROUND_PAGE_SIZE = 3;
// 与后端请求 ACK 超时保持一致，避免客户端先于服务端判定发送失败。
const MESSAGE_SEND_ACK_TIMEOUT_MS = 10_000;

export function isStrictModeEnabled(): boolean {
  return ENABLE_STRICT_MODE;
}

export function getMessageHistoryRoundPageSize(): number {
  return MESSAGE_HISTORY_ROUND_PAGE_SIZE;
}

export function getMessageSendAckTimeoutMs(): number {
  return MESSAGE_SEND_ACK_TIMEOUT_MS;
}

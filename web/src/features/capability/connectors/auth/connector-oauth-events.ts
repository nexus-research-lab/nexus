// INPUT: 当前浏览上下文中正在打开的 OAuth Connector，以及回调页的受控结果。
// OUTPUT: 不携带 code/state/Provider 正文的同源 OAuth 完成通知。
// POS: OAuth 弹窗与 Connector 目录之间的短生命周期相关性边界；不充当服务端回执。
export type ConnectorOAuthEventType = "connector-oauth:success" | "connector-oauth:error";
export type ConnectorOAuthFailureKind = "not_connected" | "outcome_unknown";

export type ConnectorOAuthEvent = {
  connector_id?: string;
  event_id: string;
  failure_kind?: ConnectorOAuthFailureKind;
  type: ConnectorOAuthEventType;
  message: string;
};

const CONNECTOR_OAUTH_CHANNEL = "nexus.connector-oauth";
const PENDING_CONNECTOR_KEY = "nexus.connector-oauth.pending-connector";
const MAX_CONNECTOR_ID_LENGTH = 128;

function normalizeConnectorId(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const normalized = value.trim();
  return normalized
    && normalized.length <= MAX_CONNECTOR_ID_LENGTH
    && /^[a-zA-Z0-9._-]+$/.test(normalized)
    ? normalized
    : null;
}

export function rememberPendingConnectorOauth(connectorId: string): void {
  const normalized = normalizeConnectorId(connectorId);
  if (!normalized) {
    return;
  }
  try {
    window.sessionStorage.setItem(PENDING_CONNECTOR_KEY, normalized);
  } catch {
    // sessionStorage 不可用时仍可完成 OAuth；事件将退化为无 connector_id。
  }
}

export function readPendingConnectorOauth(): string | null {
  try {
    const local = normalizeConnectorId(
      window.sessionStorage.getItem(PENDING_CONNECTOR_KEY),
    );
    if (local) {
      return local;
    }
    if (window.opener && !window.opener.closed) {
      return normalizeConnectorId(
        window.opener.sessionStorage.getItem(PENDING_CONNECTOR_KEY),
      );
    }
  } catch {
    // 跨来源 opener 或浏览器隐私设置不可用时，不猜测 Connector identity。
  }
  return null;
}

export function clearPendingConnectorOauth(connectorId?: string | null): void {
  try {
    const current = normalizeConnectorId(
      window.sessionStorage.getItem(PENDING_CONNECTOR_KEY),
    );
    const expected = normalizeConnectorId(connectorId);
    if (!expected || current === expected) {
      window.sessionStorage.removeItem(PENDING_CONNECTOR_KEY);
    }
  } catch {
    // 清理失败不影响服务端连接状态。
  }
}

function createEventId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function isConnectorOAuthEvent(value: unknown): value is ConnectorOAuthEvent {
  if (!value || typeof value !== "object") {
    return false;
  }
  const payload = value as Partial<ConnectorOAuthEvent>;
  return (
    typeof payload.event_id === "string" &&
    typeof payload.message === "string" &&
    (payload.connector_id === undefined
      || normalizeConnectorId(payload.connector_id) !== null) &&
    (payload.failure_kind === undefined
      || payload.failure_kind === "not_connected"
      || payload.failure_kind === "outcome_unknown") &&
    (payload.type === "connector-oauth:success" || payload.type === "connector-oauth:error")
  );
}

export function publishConnectorOauthEvent(
  type: ConnectorOAuthEventType,
  message: string,
  options: {
    connectorId?: string | null;
    failureKind?: ConnectorOAuthFailureKind;
  } = {},
): void {
  const connectorId = normalizeConnectorId(options.connectorId)
    ?? readPendingConnectorOauth();
  const payload: ConnectorOAuthEvent = {
    ...(connectorId ? { connector_id: connectorId } : {}),
    event_id: createEventId(),
    ...(options.failureKind ? { failure_kind: options.failureKind } : {}),
    type,
    message,
  };

  if (window.opener && !window.opener.closed) {
    window.opener.postMessage(payload, window.location.origin);
  }

  if (typeof BroadcastChannel !== "undefined") {
    const channel = new BroadcastChannel(CONNECTOR_OAUTH_CHANNEL);
    channel.postMessage(payload);
    channel.close();
  }
}

export function subscribeConnectorOauthEvent(
  handler: (event: ConnectorOAuthEvent) => void,
): () => void {
  // 去重只服务当前订阅生命周期，避免模块级集合随 OAuth 次数永久增长。
  const handledEventIds = new Set<string>();
  const handleEvent = (event: ConnectorOAuthEvent) => {
    if (handledEventIds.has(event.event_id)) {
      return;
    }
    handledEventIds.add(event.event_id);
    handler(event);
  };

  const handleWindowMessage = (event: MessageEvent) => {
    if (event.origin !== window.location.origin || !isConnectorOAuthEvent(event.data)) {
      return;
    }
    handleEvent(event.data);
  };

  window.addEventListener("message", handleWindowMessage);

  const channel = typeof BroadcastChannel !== "undefined"
    ? new BroadcastChannel(CONNECTOR_OAUTH_CHANNEL)
    : null;
  const handleChannelMessage = (event: MessageEvent) => {
    if (isConnectorOAuthEvent(event.data)) {
      handleEvent(event.data);
    }
  };
  channel?.addEventListener("message", handleChannelMessage);

  return () => {
    window.removeEventListener("message", handleWindowMessage);
    channel?.removeEventListener("message", handleChannelMessage);
    channel?.close();
    handledEventIds.clear();
  };
}

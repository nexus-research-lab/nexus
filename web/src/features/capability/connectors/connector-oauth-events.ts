export type ConnectorOAuthEventType = "connector-oauth:success" | "connector-oauth:error";

export type ConnectorOAuthEvent = {
  event_id: string;
  type: ConnectorOAuthEventType;
  message: string;
};

const CONNECTOR_OAUTH_CHANNEL = "nexus.connector-oauth";
const handledOauthEventIds = new Set<string>();

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
    (payload.type === "connector-oauth:success" || payload.type === "connector-oauth:error")
  );
}

export function publishConnectorOauthEvent(
  type: ConnectorOAuthEventType,
  message: string,
): void {
  const payload: ConnectorOAuthEvent = {
    event_id: createEventId(),
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
  const handleEvent = (event: ConnectorOAuthEvent) => {
    if (handledOauthEventIds.has(event.event_id)) {
      return;
    }
    handledOauthEventIds.add(event.event_id);
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
  };
}

export type ConnectorOAuthEventType = "connector-oauth:success" | "connector-oauth:error";

export type ConnectorOAuthEvent = {
  event_id: string;
  type: ConnectorOAuthEventType;
  message: string;
};

const CONNECTOR_OAUTH_CHANNEL = "nexus.connector-oauth";
const handled_oauth_event_ids = new Set<string>();

function create_event_id(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function is_connector_oauth_event(value: unknown): value is ConnectorOAuthEvent {
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

export function publish_connector_oauth_event(
  type: ConnectorOAuthEventType,
  message: string,
): void {
  const payload: ConnectorOAuthEvent = {
    event_id: create_event_id(),
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

export function subscribe_connector_oauth_event(
  handler: (event: ConnectorOAuthEvent) => void,
): () => void {
  const handle_event = (event: ConnectorOAuthEvent) => {
    if (handled_oauth_event_ids.has(event.event_id)) {
      return;
    }
    handled_oauth_event_ids.add(event.event_id);
    handler(event);
  };

  const handle_window_message = (event: MessageEvent) => {
    if (event.origin !== window.location.origin || !is_connector_oauth_event(event.data)) {
      return;
    }
    handle_event(event.data);
  };

  window.addEventListener("message", handle_window_message);

  const channel = typeof BroadcastChannel !== "undefined"
    ? new BroadcastChannel(CONNECTOR_OAUTH_CHANNEL)
    : null;
  const handle_channel_message = (event: MessageEvent) => {
    if (is_connector_oauth_event(event.data)) {
      handle_event(event.data);
    }
  };
  channel?.addEventListener("message", handle_channel_message);

  return () => {
    window.removeEventListener("message", handle_window_message);
    channel?.removeEventListener("message", handle_channel_message);
    channel?.close();
  };
}

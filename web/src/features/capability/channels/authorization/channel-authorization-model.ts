import {
  asUnknownRecord,
  readBoolean,
  readString,
  readStringFromSet,
} from "@/lib/unknown-value";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import type {
  ChannelAuthorizationData,
  ChannelAuthorizationKind,
  ChannelAuthorizationResultData,
} from "@/types/generated/protocol";

const PRESENTATION_KINDS = new Set<ChannelAuthorizationKind>([
  "qr_code",
  "verification_code",
]);

export function parseChannelAuthorizationPresentation(
  value: unknown,
): ChannelAuthorizationData | null {
  const event = parseEventMessage(value);
  if (!event || event.event_type !== "channel_authorization") {
    return null;
  }
  const data = asUnknownRecord(event.data);
  if (!data) {
    return null;
  }
  const flowId = normalize(readString(data, "flow_id"));
  const presentationToken = normalize(readString(data, "presentation_token"));
  const kind = readStringFromSet(data, "kind", PRESENTATION_KINDS);
  const channelType = normalize(readString(data, "channel_type"));
  const accountBinding = normalize(readString(data, "account_binding"));
  const prompt = normalize(readString(data, "prompt"));
  const expiresAt = normalize(readString(data, "expires_at"));
  if (
    !flowId
    || !presentationToken
    || !kind
    || !channelType
    || !accountBinding
    || !prompt
    || !expiresAt
    || !Number.isFinite(Date.parse(expiresAt))
  ) {
    return null;
  }
  const qrPayload = normalize(readString(data, "qr_payload"));
  if (kind === "qr_code" && !qrPayload) {
    return null;
  }
  return {
    flow_id: flowId,
    presentation_token: presentationToken,
    kind,
    channel_type: channelType,
    account_binding: accountBinding,
    prompt,
    expires_at: expiresAt,
    ...(qrPayload ? { qr_payload: qrPayload } : {}),
    ...(normalize(readString(data, "qr_payload_type"))
      ? { qr_payload_type: normalize(readString(data, "qr_payload_type")) }
      : {}),
  };
}

export function parseChannelAuthorizationResult(
  value: unknown,
): ChannelAuthorizationResultData | null {
  const event = parseEventMessage(value);
  if (!event || event.event_type !== "channel_authorization_result") {
    return null;
  }
  const data = asUnknownRecord(event.data);
  if (!data) {
    return null;
  }
  const flowId = normalize(readString(data, "flow_id"));
  const accepted = readBoolean(data, "accepted");
  const message = normalize(readString(data, "message"));
  if (!flowId || accepted === null || !message) {
    return null;
  }
  const status = normalize(readString(data, "status"));
  return {
    flow_id: flowId,
    accepted,
    message,
    ...(status ? { status } : {}),
  };
}

function normalize(value: string | null): string {
  return value?.trim() ?? "";
}

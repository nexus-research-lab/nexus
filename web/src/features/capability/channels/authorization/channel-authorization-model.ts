// INPUT: Unknown Channel authorization events and controlled local failure kinds.
// OUTPUT: Validated presentations/results plus localized Problem/Impact/Recovery facts.
// POS: Authorization trust boundary; server result text never becomes visible copy.
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
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

const PRESENTATION_KINDS = new Set<ChannelAuthorizationKind>([
  "qr_code",
  "verification_code",
]);

export type ChannelAuthorizationAction = "cancel" | "submit";

export interface ChannelAuthorizationFailure {
  impact: string;
  nextStep: string;
  title: string;
  writeLocked: boolean;
}

const AUTHORIZATION_FAILURE_COPY = {
  cancel: {
    not_sent: {
      impact: "capability.channel_authorization_cancel_not_sent_impact",
      nextStep: "capability.channel_authorization_cancel_not_sent_next_step",
      title: "capability.channel_authorization_cancel_not_sent_title",
    },
    rejected: {
      impact: "capability.channel_authorization_cancel_rejected_impact",
      nextStep: "capability.channel_authorization_cancel_rejected_next_step",
      title: "capability.channel_authorization_cancel_rejected_title",
    },
  },
  submit: {
    not_sent: {
      impact: "capability.channel_authorization_submit_not_sent_impact",
      nextStep: "capability.channel_authorization_submit_not_sent_next_step",
      title: "capability.channel_authorization_submit_not_sent_title",
    },
    rejected: {
      impact: "capability.channel_authorization_submit_rejected_impact",
      nextStep: "capability.channel_authorization_submit_rejected_next_step",
      title: "capability.channel_authorization_submit_rejected_title",
    },
  },
} as const;

export function buildChannelAuthorizationFailure(
  action: ChannelAuthorizationAction,
  delivery: "not_sent" | "rejected",
  t: I18nContextValue["t"],
): ChannelAuthorizationFailure {
  const copy = AUTHORIZATION_FAILURE_COPY[action][delivery];
  return {
    impact: t(copy.impact),
    nextStep: t(copy.nextStep),
    title: t(copy.title),
    // A local not_sent disposition proves that no frame was sent. A negative
    // server ACK does not prove whether the asynchronous authorization changed
    // the Channel aggregate before its controller returned an error.
    writeLocked: delivery === "rejected",
  };
}

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

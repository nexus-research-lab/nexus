import type {
  ConnectorAuthType,
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

import { isDirectCredentialAuth } from "../auth/connector-auth";

export type ConnectorPrimaryAction =
  | "connect"
  | "configure-credential"
  | "disconnect"
  | "coming-soon"
  | "unavailable"
  | "none";

export type ConnectorStatusTone =
  | "connected"
  | "coming-soon"
  | "unconfigured"
  | "disconnected";

export interface ConnectorDetailState {
  oauthClientConfigured: boolean;
  primaryAction: ConnectorPrimaryAction;
  requiresOauthClientConfig: boolean;
  status: ConnectorStatusTone;
}

interface StateRule<Value> {
  matches: boolean;
  value: Value;
}

const AUTH_LABELS: Record<ConnectorAuthType, string> = {
  oauth2: "OAuth 2.0",
  api_key: "API Key",
  token: "Token",
  none: "无需授权",
};

export function getConnectorAuthLabel(authType: ConnectorAuthType): string {
  return AUTH_LABELS[authType];
}

export function getConnectorFeatureDetails(
  detail: ConnectorDetail,
): ConnectorFeatureDetail[] {
  const featureDetails = detail.feature_details ?? [];
  if (featureDetails.length === 0 || detail.features.length === 0) {
    return featureDetails;
  }
  const detailsByName = new Map(
    featureDetails.map((feature) => [feature.name, feature]),
  );
  return detail.features.flatMap((name) => {
    const feature = detailsByName.get(name);
    return feature ? [feature] : [];
  });
}

export function getConnectorDetailState(
  detail: ConnectorDetail,
): ConnectorDetailState {
  const connected = detail.connection_state === "connected";
  const comingSoon = detail.status === "coming_soon";
  const configured = detail.is_configured;
  const requiresOauthClientConfig =
    detail.oauth_client_config_required ?? false;
  const primaryActionRules: StateRule<ConnectorPrimaryAction>[] = [
    { matches: connected, value: "disconnect" },
    {
      matches: !comingSoon && configured,
      value: isDirectCredentialAuth(detail.auth_type)
        ? "configure-credential"
        : "connect",
    },
    { matches: comingSoon, value: "coming-soon" },
    { matches: requiresOauthClientConfig, value: "none" },
    { matches: true, value: "unavailable" },
  ];
  const statusRules: StateRule<ConnectorStatusTone>[] = [
    { matches: connected, value: "connected" },
    { matches: comingSoon, value: "coming-soon" },
    { matches: !configured, value: "unconfigured" },
    { matches: true, value: "disconnected" },
  ];

  return {
    oauthClientConfigured: detail.oauth_client_configured ?? false,
    primaryAction: primaryActionRules.find((rule) => rule.matches)?.value
      ?? "unavailable",
    requiresOauthClientConfig,
    status: statusRules.find((rule) => rule.matches)?.value ?? "disconnected",
  };
}

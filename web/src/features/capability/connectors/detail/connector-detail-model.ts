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

export type ConnectorOauthClientAction = "configure" | "reconfigure" | null;

export interface ConnectorDetailState {
  configurationError: string | null | undefined;
  oauthClientAction: ConnectorOauthClientAction;
  primaryAction: ConnectorPrimaryAction;
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
  const featureDetails = detail.feature_details;
  if (!featureDetails?.length) {
    return [];
  }
  if (detail.features.length === 0) {
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

function firstMatchingValue<Value>(rules: StateRule<Value>[]): Value {
  const rule = rules.find((candidate) => candidate.matches);
  if (!rule) {
    throw new Error("连接器状态规则缺少兜底项");
  }
  return rule.value;
}

function configuredPrimaryAction(
  authType: ConnectorAuthType,
): ConnectorPrimaryAction {
  return isDirectCredentialAuth(authType)
    ? "configure-credential"
    : "connect";
}

function configurationError(
  detail: ConnectorDetail,
  status: ConnectorStatusTone,
  requiresOauthClientConfig: boolean,
): string | null | undefined {
  if (status !== "unconfigured") {
    return null;
  }
  if (requiresOauthClientConfig) {
    return null;
  }
  return detail.config_error;
}

function oauthClientAction({
  connected,
  oauthClientConfigured,
  requiresOauthClientConfig,
}: {
  connected: boolean;
  oauthClientConfigured: boolean;
  requiresOauthClientConfig: boolean;
}): ConnectorOauthClientAction {
  return firstMatchingValue([
    { matches: connected, value: null },
    { matches: !requiresOauthClientConfig, value: null },
    { matches: oauthClientConfigured, value: "reconfigure" },
    { matches: true, value: "configure" },
  ]);
}

export function getConnectorDetailState(
  detail: ConnectorDetail,
): ConnectorDetailState {
  const connected = detail.connection_state === "connected";
  const comingSoon = detail.status === "coming_soon";
  const configured = detail.is_configured;
  const requiresOauthClientConfig = Boolean(
    detail.oauth_client_config_required,
  );
  const oauthClientConfigured = Boolean(detail.oauth_client_configured);
  const primaryActionRules: StateRule<ConnectorPrimaryAction>[] = [
    { matches: connected, value: "disconnect" },
    {
      matches: !comingSoon && configured,
      value: configuredPrimaryAction(detail.auth_type),
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
  const status = firstMatchingValue(statusRules);

  return {
    configurationError: configurationError(
      detail,
      status,
      requiresOauthClientConfig,
    ),
    oauthClientAction: oauthClientAction({
      connected,
      oauthClientConfigured,
      requiresOauthClientConfig,
    }),
    primaryAction: firstMatchingValue(primaryActionRules),
    status,
  };
}

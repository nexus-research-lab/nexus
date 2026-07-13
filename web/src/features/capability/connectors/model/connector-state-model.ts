import type {
  ConnectorAuthType,
  ConnectorInfo,
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

export interface ConnectorState {
  configurationError: string | null | undefined;
  oauthClientAction: ConnectorOauthClientAction;
  primaryAction: ConnectorPrimaryAction;
  status: ConnectorStatusTone;
}

interface StateRule<Value> {
  matches: boolean;
  value: Value;
}

export function getConnectorState(connector: ConnectorInfo): ConnectorState {
  const connected = connector.connection_state === "connected";
  const comingSoon = connector.status === "coming_soon";
  const configured = connector.is_configured;
  const requiresOauthClientConfig = Boolean(
    connector.oauth_client_config_required,
  );
  const oauthClientConfigured = Boolean(connector.oauth_client_configured);
  const status = firstMatchingValue<ConnectorStatusTone>([
    { matches: connected, value: "connected" },
    { matches: comingSoon, value: "coming-soon" },
    { matches: !configured, value: "unconfigured" },
    { matches: true, value: "disconnected" },
  ]);

  return {
    configurationError: resolveConfigurationError(
      connector,
      status,
      requiresOauthClientConfig,
    ),
    oauthClientAction: resolveOauthClientAction({
      connected,
      oauthClientConfigured,
      requiresOauthClientConfig,
    }),
    primaryAction: firstMatchingValue<ConnectorPrimaryAction>([
      { matches: connected, value: "disconnect" },
      {
        matches: !comingSoon && configured,
        value: configuredPrimaryAction(connector.auth_type),
      },
      { matches: comingSoon, value: "coming-soon" },
      { matches: requiresOauthClientConfig, value: "none" },
      { matches: true, value: "unavailable" },
    ]),
    status,
  };
}

function firstMatchingValue<Value>(rules: StateRule<Value>[]): Value {
  const rule = rules.find((candidate) => candidate.matches);
  if (!rule) throw new Error("连接器状态规则缺少兜底项");
  return rule.value;
}

function configuredPrimaryAction(
  authType: ConnectorAuthType,
): ConnectorPrimaryAction {
  return isDirectCredentialAuth(authType)
    ? "configure-credential"
    : "connect";
}

function resolveConfigurationError(
  connector: ConnectorInfo,
  status: ConnectorStatusTone,
  requiresOauthClientConfig: boolean,
): string | null | undefined {
  if (status !== "unconfigured" || requiresOauthClientConfig) return null;
  return connector.config_error;
}

function resolveOauthClientAction({
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

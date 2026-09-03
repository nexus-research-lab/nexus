import type {
  ConnectorAuthType,
  ConnectorInfo,
} from "@/types/capability/connector";

type DirectCredentialAuthType = Extract<ConnectorAuthType, "api_key" | "token">;

export type ConnectorConnectMode =
  | "direct"
  | "direct-credential"
  | "local-pairing"
  | "oauth-browser"
  | "oauth-device";

export function isDirectCredentialAuth(
  authType?: ConnectorAuthType | null,
): authType is DirectCredentialAuthType {
  return authType === "api_key" || authType === "token";
}

export function getDirectCredentialLabel(authType?: ConnectorAuthType | null): string {
  return authType === "token" ? "Token" : "API Key";
}

export function buildDirectCredentialPayload(
  authType: DirectCredentialAuthType,
  credential: string,
): Record<string, string> {
  return authType === "token" ? { token: credential } : { api_key: credential };
}

export function resolveConnectorConnectMode(
  connector: ConnectorInfo,
  desktopRuntime: boolean,
): ConnectorConnectMode {
  if (connector.auth_type === "local_pairing") {
    return "local-pairing";
  }
  if (connector.auth_type !== "oauth2") {
    return isDirectCredentialAuth(connector.auth_type)
      ? "direct-credential"
      : "direct";
  }
  return connector.supports_device_auth
    && (connector.connector_id !== "github" || desktopRuntime)
    ? "oauth-device"
    : "oauth-browser";
}

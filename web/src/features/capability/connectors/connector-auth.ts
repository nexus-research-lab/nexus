import type { ConnectorAuthType } from "@/types/capability/connector";

type DirectCredentialAuthType = Extract<ConnectorAuthType, "api_key" | "token">;

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

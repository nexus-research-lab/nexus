import type { ConnectorAuthType } from "@/types/capability/connector";

type DirectCredentialAuthType = Extract<ConnectorAuthType, "api_key" | "token">;

export function is_direct_credential_auth(
  authType?: ConnectorAuthType | null,
): authType is DirectCredentialAuthType {
  return authType === "api_key" || authType === "token";
}

export function get_direct_credential_label(authType?: ConnectorAuthType | null): string {
  return authType === "token" ? "Token" : "API Key";
}

export function build_direct_credential_payload(
  authType: DirectCredentialAuthType,
  credential: string,
): Record<string, string> {
  return authType === "token" ? { token: credential } : { api_key: credential };
}

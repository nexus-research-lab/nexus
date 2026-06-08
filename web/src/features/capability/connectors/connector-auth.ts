import type { ConnectorAuthType } from "@/types/capability/connector";

type DirectCredentialAuthType = Extract<ConnectorAuthType, "api_key" | "token">;

export function is_direct_credential_auth(
  auth_type?: ConnectorAuthType | null,
): auth_type is DirectCredentialAuthType {
  return auth_type === "api_key" || auth_type === "token";
}

export function get_direct_credential_label(auth_type?: ConnectorAuthType | null): string {
  return auth_type === "token" ? "Token" : "API Key";
}

export function build_direct_credential_payload(
  auth_type: DirectCredentialAuthType,
  credential: string,
): Record<string, string> {
  return auth_type === "token" ? { token: credential } : { api_key: credential };
}

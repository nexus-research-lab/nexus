import { getConnectorOauthRedirectUri } from "@/config/desktop-runtime";
import type { ConnectorDetail } from "@/types/capability/connector";

export interface ConnectorOauthClientDialogModel {
  callbackUrl: string;
  configured: boolean;
  connectorId: string;
  docsUrl: string | undefined;
  initialClientId: string;
  providerName: string;
  resetKey: string;
  secretPlaceholder: string;
  title: string;
}

const PROVIDER_NAMES: Partial<Record<string, string>> = {
  "feishu-docx": "飞书开放平台应用",
};

export function buildConnectorOauthClientDialogModel(
  detail: ConnectorDetail | null,
): ConnectorOauthClientDialogModel | null {
  if (!detail) return null;
  const configured = detail.oauth_client_configured ?? false;
  const initialClientId = detail.oauth_client_id ?? "";
  return {
    callbackUrl: getConnectorOauthRedirectUri(),
    configured,
    connectorId: detail.connector_id,
    docsUrl: detail.docs_url,
    initialClientId,
    providerName: PROVIDER_NAMES[detail.connector_id] ?? "OAuth 应用",
    resetKey: `${detail.connector_id}\x1f${initialClientId}`,
    secretPlaceholder: configured
      ? "重新填写后保存"
      : "飞书应用 App Secret",
    title: detail.title,
  };
}

export function connectorOauthCredentialsComplete(
  clientId: string,
  clientSecret: string,
): boolean {
  return Boolean(clientId.trim() && clientSecret.trim());
}

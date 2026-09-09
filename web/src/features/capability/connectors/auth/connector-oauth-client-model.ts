// INPUT: OAuth 配置身份、公开字段与当前语言。
// OUTPUT: 具名配置视图与独立于语言的草稿重置身份。
// POS: OAuth 客户端配置纯投影；不提交凭据。
import { getConnectorOauthRedirectUri } from "@/config/desktop-runtime";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ConnectorDetail } from "@/types/capability/connector";

export interface ConnectorOauthClientDialogModel {
  callbackUrl: string;
  configured: boolean;
  connectorId: string;
  docsUrl: string | undefined;
  initialClientId: string;
  clientIdPlaceholder: string;
  providerName: string;
  resetKey: string;
  secretPlaceholder: string;
  title: string;
}

const PROVIDER_NAMES: Partial<Record<string, TranslationKey>> = {
  "feishu-docx": "capability.oauth_provider_feishu",
};

export function buildConnectorOauthClientDialogModel(
  detail: ConnectorDetail | null,
  t: I18nContextValue["t"],
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
    providerName: t(PROVIDER_NAMES[detail.connector_id] ?? "capability.oauth_provider_generic"),
    clientIdPlaceholder: detail.connector_id === "feishu-docx" ? t("capability.oauth_feishu_id") : "Client ID",
    resetKey: `${detail.connector_id}\x1f${initialClientId}`,
    secretPlaceholder: configured
      ? t("capability.oauth_secret_replace")
      : detail.connector_id === "feishu-docx" ? t("capability.oauth_feishu_secret") : "Client Secret",
    title: detail.title,
  };
}

export function connectorOauthCredentialsComplete(
  clientId: string,
  clientSecret: string,
): boolean {
  return Boolean(clientId.trim() && clientSecret.trim());
}

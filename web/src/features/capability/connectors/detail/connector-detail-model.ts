import type {
  ConnectorAuthType,
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

const AUTH_LABEL_KEYS: Record<ConnectorAuthType, TranslationKey> = {
  custom_mcp: "capability.connector_auth_custom_mcp",
  oauth2: "capability.connector_auth_oauth2",
  api_key: "capability.connector_auth_api_key",
  token: "capability.connector_auth_token",
  none: "capability.connector_auth_none",
  local_pairing: "capability.connector_auth_local_pairing",
};

export function getConnectorAuthLabelKey(authType: ConnectorAuthType): TranslationKey {
  return AUTH_LABEL_KEYS[authType];
}

export function canReplaceConnectorOauthClient(
  detail: ConnectorDetail,
): boolean {
  return detail.connector_id === "feishu-docx"
    && Boolean(detail.oauth_client_id?.trim());
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
// INPUT: Connector 认证方式、OAuth 应用资格及能力目录。
// OUTPUT: 认证文案 key、应用替换资格与按服务端顺序排列的能力详情。
// POS: 详情纯投影，不读取界面语言或改变认证状态。
import type { TranslationKey } from "@/shared/i18n/messages";

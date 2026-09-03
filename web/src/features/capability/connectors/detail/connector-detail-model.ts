import type {
  ConnectorAuthType,
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

const AUTH_LABELS: Record<ConnectorAuthType, string> = {
  custom_mcp: "自定义 MCP",
  oauth2: "OAuth 2.0",
  api_key: "API Key",
  token: "Token",
  none: "无需授权",
  local_pairing: "本机应用配对",
};

export function getConnectorAuthLabel(authType: ConnectorAuthType): string {
  return AUTH_LABELS[authType];
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

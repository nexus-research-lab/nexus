import type {
  ConnectorAuthType,
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

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

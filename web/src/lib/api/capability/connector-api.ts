/**
 * Connector API 服务模块
 *
 * [INPUT]: 依赖 @/types/capability/connector, @/types/system/api
 * [OUTPUT]: 对外提供连接器 CRUD + OAuth 操作
 */

import {
  ConnectorDetail,
  ConnectorDeviceAuthPollResult,
  ConnectorDeviceAuthStart,
  ConnectorInfo,
} from "@/types/capability/connector";
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

const BASE = getAgentApiBaseUrl();

/** 获取连接器列表 */
export const getConnectorsApi = async (params?: {
  q?: string;
  category?: string;
  status?: string;
}): Promise<ConnectorInfo[]> => {
  const sp = new URLSearchParams();
  if (params?.q) sp.set("q", params.q);
  if (params?.category) sp.set("category", params.category);
  if (params?.status) sp.set("status", params.status);
  const qs = sp.toString();
  const url = `${BASE}/connectors${qs ? `?${qs}` : ""}`;
  return requestApi<ConnectorInfo[]>(url, {
    method: "GET",
  });
};

/** 获取连接器详情 */
export const getConnectorDetailApi = async (
  connectorId: string,
): Promise<ConnectorDetail> => {
  return requestApi<ConnectorDetail>(`${BASE}/connectors/${connectorId}`, {
    method: "GET",
  });
};

/** 授权连接 */
export const connectConnectorApi = async (
  connectorId: string,
  body?: {
    auth_code?: string;
    api_key?: string;
    token?: string;
    redirect_uri?: string;
  },
): Promise<ConnectorInfo> => {
  return requestApi<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/connect`,
    {
      method: "POST",
      body: JSON.stringify(body),
    },
  );
};

/** 断开连接 */
export const disconnectConnectorApi = async (
  connectorId: string,
): Promise<ConnectorInfo> => {
  return requestApi<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/disconnect`,
    {
      method: "POST",
    },
  );
};

/** 保存用户自有 OAuth Client 配置 */
export const saveConnectorOauthClientApi = async (
  connectorId: string,
  body: {
    client_id: string;
    client_secret: string;
  },
): Promise<ConnectorInfo> => {
  return requestApi<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/oauth-client`,
    {
      method: "PUT",
      body: JSON.stringify(body),
    },
  );
};

/** 删除用户自有 OAuth Client 配置 */
export const deleteConnectorOauthClientApi = async (
  connectorId: string,
): Promise<ConnectorInfo> => {
  return requestApi<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/oauth-client`,
    {
      method: "DELETE",
    },
  );
};

/** 获取 OAuth 授权 URL */
export const getConnectorAuthUrlApi = async (
  connectorId: string,
  redirectUri?: string,
  shop?: string,
): Promise<{ auth_url: string }> => {
  const sp = new URLSearchParams();
  if (redirectUri) sp.set("redirect_uri", redirectUri);
  if (shop) sp.set("shop", shop);
  const qs = sp.toString();
  const url = `${BASE}/connectors/${connectorId}/auth-url${qs ? `?${qs}` : ""}`;
  return requestApi<{ auth_url: string }>(url, {
    method: "GET",
  });
};

/** 完成 OAuth 回调 */
export const completeConnectorOAuthApi = async (
  code: string,
  state: string,
  redirectUri?: string,
): Promise<ConnectorInfo> => {
  const body = { code, state, redirect_uri: redirectUri };
  return requestApi<ConnectorInfo>(`${BASE}/connectors/oauth/callback`, {
    method: "POST",
    body: JSON.stringify(body),
  });
};

/** 启动 OAuth Device Flow */
export const startConnectorDeviceAuthApi = async (
  connectorId: string,
): Promise<ConnectorDeviceAuthStart> => {
  return requestApi<ConnectorDeviceAuthStart>(
    `${BASE}/connectors/${connectorId}/device/start`,
    {
      method: "POST",
    },
  );
};

/** 轮询 OAuth Device Flow */
export const pollConnectorDeviceAuthApi = async (
  connectorId: string,
  deviceCode: string,
): Promise<ConnectorDeviceAuthPollResult> => {
  return requestApi<ConnectorDeviceAuthPollResult>(
    `${BASE}/connectors/${connectorId}/device/poll`,
    {
      method: "POST",
      body: JSON.stringify({ device_code: deviceCode }),
    },
  );
};

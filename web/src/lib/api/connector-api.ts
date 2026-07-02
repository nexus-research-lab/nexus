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
import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";

const BASE = get_agent_api_base_url();

/** 获取连接器列表 */
export const get_connectors_api = async (params?: {
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
  return request_api<ConnectorInfo[]>(url, {
    method: "GET",
  });
};

/** 获取连接器详情 */
export const get_connector_detail_api = async (
  connectorId: string,
): Promise<ConnectorDetail> => {
  return request_api<ConnectorDetail>(`${BASE}/connectors/${connectorId}`, {
    method: "GET",
  });
};

/** 授权连接 */
export const connect_connector_api = async (
  connectorId: string,
  body?: {
    auth_code?: string;
    api_key?: string;
    token?: string;
    redirect_uri?: string;
  },
): Promise<ConnectorInfo> => {
  return request_api<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/connect`,
    {
      method: "POST",
      body: JSON.stringify(body),
    },
  );
};

/** 断开连接 */
export const disconnect_connector_api = async (
  connectorId: string,
): Promise<ConnectorInfo> => {
  return request_api<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/disconnect`,
    {
      method: "POST",
    },
  );
};

/** 保存用户自有 OAuth Client 配置 */
export const save_connector_oauth_client_api = async (
  connectorId: string,
  body: {
    client_id: string;
    client_secret: string;
  },
): Promise<ConnectorInfo> => {
  return request_api<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/oauth-client`,
    {
      method: "PUT",
      body: JSON.stringify(body),
    },
  );
};

/** 删除用户自有 OAuth Client 配置 */
export const delete_connector_oauth_client_api = async (
  connectorId: string,
): Promise<ConnectorInfo> => {
  return request_api<ConnectorInfo>(
    `${BASE}/connectors/${connectorId}/oauth-client`,
    {
      method: "DELETE",
    },
  );
};

/** 获取 OAuth 授权 URL */
export const get_connector_auth_url_api = async (
  connectorId: string,
  redirectUri?: string,
  shop?: string,
): Promise<{ auth_url: string }> => {
  const sp = new URLSearchParams();
  if (redirectUri) sp.set("redirect_uri", redirectUri);
  if (shop) sp.set("shop", shop);
  const qs = sp.toString();
  const url = `${BASE}/connectors/${connectorId}/auth-url${qs ? `?${qs}` : ""}`;
  return request_api<{ auth_url: string }>(url, {
    method: "GET",
  });
};

/** 完成 OAuth 回调 */
export const complete_connector_o_auth_api = async (
  code: string,
  state: string,
  redirectUri?: string,
): Promise<ConnectorInfo> => {
  const body = { code, state, redirect_uri: redirectUri };
  return request_api<ConnectorInfo>(`${BASE}/connectors/oauth/callback`, {
    method: "POST",
    body: JSON.stringify(body),
  });
};

/** 启动 OAuth Device Flow */
export const start_connector_device_auth_api = async (
  connectorId: string,
): Promise<ConnectorDeviceAuthStart> => {
  return request_api<ConnectorDeviceAuthStart>(
    `${BASE}/connectors/${connectorId}/device/start`,
    {
      method: "POST",
    },
  );
};

/** 轮询 OAuth Device Flow */
export const poll_connector_device_auth_api = async (
  connectorId: string,
  deviceCode: string,
): Promise<ConnectorDeviceAuthPollResult> => {
  return request_api<ConnectorDeviceAuthPollResult>(
    `${BASE}/connectors/${connectorId}/device/poll`,
    {
      method: "POST",
      body: JSON.stringify({ device_code: deviceCode }),
    },
  );
};

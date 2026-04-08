/**
 * Connector API 服务模块
 *
 * [INPUT]: 依赖 @/types/connector, @/types/api
 * [OUTPUT]: 对外提供连接器 CRUD + OAuth 操作
 */

import { ConnectorDetail, ConnectorInfo } from '@/types/connector';
import { getAgentApiBaseUrl } from '@/config/options';
import { request_api } from '@/lib/http';

const BASE = getAgentApiBaseUrl();

/** 获取连接器列表 */
export const getConnectorsApi = async (params?: {
  q?: string;
  category?: string;
  status?: string;
}): Promise<ConnectorInfo[]> => {
  const sp = new URLSearchParams();
  if (params?.q) sp.set('q', params.q);
  if (params?.category) sp.set('category', params.category);
  if (params?.status) sp.set('status', params.status);
  const qs = sp.toString();
  const url = `${BASE}/connectors${qs ? `?${qs}` : ''}`;
  return request_api<ConnectorInfo[]>(url, {
    method: 'GET',
  });
};

/** 获取已连接连接器数量 */
export const getConnectedCountApi = async (): Promise<number> => {
  const connectors = await getConnectorsApi();
  return connectors.filter((connector) => connector.connection_state === "connected").length;
};

/** 获取连接器详情 */
export const getConnectorDetailApi = async (connector_id: string): Promise<ConnectorDetail> => {
  return request_api<ConnectorDetail>(`${BASE}/connectors/${connector_id}`, {
    method: 'GET',
  });
};

/** 授权连接 */
export const connectConnectorApi = async (
  connector_id: string,
  body?: { auth_code?: string; api_key?: string; token?: string; redirect_uri?: string },
): Promise<ConnectorInfo> => {
  return request_api<ConnectorInfo>(`${BASE}/connectors/${connector_id}/connect`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
};

/** 断开连接 */
export const disconnectConnectorApi = async (connector_id: string): Promise<ConnectorInfo> => {
  return request_api<ConnectorInfo>(`${BASE}/connectors/${connector_id}/disconnect`, {
    method: 'POST',
  });
};

/** 获取 OAuth 授权 URL */
export const getConnectorAuthUrlApi = async (
  connector_id: string,
  redirect_uri?: string,
): Promise<{ auth_url: string }> => {
  const sp = new URLSearchParams();
  if (redirect_uri) sp.set('redirect_uri', redirect_uri);
  const qs = sp.toString();
  const url = `${BASE}/connectors/${connector_id}/auth-url${qs ? `?${qs}` : ''}`;
  return request_api<{ auth_url: string }>(url, {
    method: 'GET',
  });
};

/** 完成 OAuth 回调 */
export const completeConnectorOAuthApi = async (
  code: string,
  state: string,
  redirect_uri?: string,
): Promise<ConnectorInfo> => {
  const body = { code, state, redirect_uri };
  return request_api<ConnectorInfo>(`${BASE}/connectors/oauth/callback`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
};

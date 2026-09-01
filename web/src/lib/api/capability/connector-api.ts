// INPUT: Connector/自定义 MCP 请求参数、动态身份与认证载荷。
// OUTPUT: 统一 Connector、OAuth、自定义 MCP CRUD/启停/Tools API。
// POS: 前端连接器 HTTP 协议与动态资源路径的唯一装配边界。

import {
  ConnectorDetail,
  ConnectorDeviceAuthMode,
  ConnectorDeviceAuthPollResult,
  ConnectorDeviceAuthStart,
  ConnectorInfo,
  CustomMCPServer,
  CustomMCPServerInput,
  CustomMCPToolCatalog,
} from "@/types/capability/connector";
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

const BASE = getAgentApiBaseUrl();

function customMCPServerApiPath(
  connectorId: string,
  suffix = "",
): string {
  return `${BASE}/custom-mcp-servers/${encodeURIComponent(connectorId)}${suffix}`;
}

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
  mode?: ConnectorDeviceAuthMode,
): Promise<ConnectorDeviceAuthStart> => {
  const query = mode ? `?mode=${encodeURIComponent(mode)}` : "";
  return requestApi<ConnectorDeviceAuthStart>(
    `${BASE}/connectors/${connectorId}/device/start${query}`,
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

/** 获取当前用户的自定义 MCP server。 */
export const getCustomMCPServersApi = async (): Promise<CustomMCPServer[]> => {
  return requestApi<CustomMCPServer[]>(`${BASE}/custom-mcp-servers`, {
    method: "GET",
  });
};

/** 获取单条脱敏自定义 MCP server。 */
export const getCustomMCPServerApi = async (
  connectorId: string,
): Promise<CustomMCPServer> => {
  return requestApi<CustomMCPServer>(
    customMCPServerApiPath(connectorId),
    { method: "GET" },
  );
};

/** 获取远程自定义 MCP 当前暴露的工具目录。 */
export const getCustomMCPToolsApi = async (
  connectorId: string,
): Promise<CustomMCPToolCatalog> => {
  return requestApi<CustomMCPToolCatalog>(
    customMCPServerApiPath(connectorId, "/capabilities"),
    { method: "GET" },
  );
};

/** 创建自定义 MCP server。 */
export const createCustomMCPServerApi = async (
  body: CustomMCPServerInput,
): Promise<CustomMCPServer> => {
  return requestApi<CustomMCPServer>(`${BASE}/custom-mcp-servers`, {
    method: "POST",
    body: JSON.stringify(body),
  });
};

/** 更新自定义 MCP server，null 秘密值表示保留已有值。 */
export const updateCustomMCPServerApi = async (
  connectorId: string,
  body: CustomMCPServerInput,
): Promise<CustomMCPServer> => {
  return requestApi<CustomMCPServer>(
    customMCPServerApiPath(connectorId),
    {
      method: "PUT",
      body: JSON.stringify(body),
    },
  );
};

/** 控制自定义 MCP 是否进入对话 Connector 选择面。 */
export const setCustomMCPServerEnabledApi = async (
  connectorId: string,
  enabled: boolean,
): Promise<CustomMCPServer> => {
  return requestApi<CustomMCPServer>(
    customMCPServerApiPath(connectorId, "/enabled"),
    {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    },
  );
};

/** 删除自定义 MCP server。 */
export const deleteCustomMCPServerApi = async (
  connectorId: string,
): Promise<void> => {
  await requestApi<{ connector_id: string }>(
    customMCPServerApiPath(connectorId),
    { method: "DELETE" },
  );
};

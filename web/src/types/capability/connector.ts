/**
 * Connector（应用授权）类型定义
 */

/** 授权方式 */
export type ConnectorAuthType = "oauth2" | "api_key" | "token" | "none";

/** 连接器可用状态 */
export type ConnectorStatus = "available" | "coming_soon";

/** 用户连接状态 */
export type ConnectionState = "connected" | "disconnected" | "expired";

/** 连接器列表项 */
export interface ConnectorInfo {
  connector_id: string;
  name: string;
  title: string;
  description: string;
  icon: string;
  category: string;
  auth_type: ConnectorAuthType;
  status: ConnectorStatus;
  connection_state: ConnectionState;
  connected_at?: string;
  is_configured: boolean;
  requires_extra?: string[];
  config_error?: string | null;
}

/** 连接器详情 */
export interface ConnectorDetail extends ConnectorInfo {
  auth_url?: string;
  token_url?: string;
  scopes: string[];
  mcp_server_url?: string;
  docs_url?: string;
  features: string[];
}

/** OAuth Device Flow 启动信息 */
export interface ConnectorDeviceAuthStart {
  connector_id: string;
  device_code: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete?: string;
  expires_in: number;
  interval: number;
}

/** OAuth Device Flow 轮询状态 */
export type ConnectorDeviceAuthStatus = "pending" | "slow_down" | "connected" | "expired" | "denied";

/** OAuth Device Flow 轮询结果 */
export interface ConnectorDeviceAuthPollResult {
  status: ConnectorDeviceAuthStatus;
  message?: string;
  connector?: ConnectorInfo;
}

/** 连接器类别 */
export interface ConnectorCategory {
  key: string;
  name: string;
}

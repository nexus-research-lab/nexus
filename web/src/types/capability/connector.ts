/**
 * Connector（应用授权）类型定义
 */

/** 授权方式 */
export type ConnectorAuthType =
  | "oauth2"
  | "api_key"
  | "token"
  | "none"
  | "custom_mcp"
  | "local_pairing";

/** 目录来源 */
export type ConnectorKind = "connector" | "custom_mcp";

/** 连接器可用状态 */
export type ConnectorStatus = "available" | "coming_soon";

/** 用户连接状态 */
export type ConnectionState = "connected" | "disconnected" | "expired";

/** 连接器列表项 */
export interface ConnectorInfo {
  connector_id: string;
  kind: ConnectorKind;
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
  oauth_client_config_required?: boolean;
  oauth_client_configured?: boolean;
  supports_device_auth?: boolean;
}

export type CustomMCPServerType = "stdio" | "http" | "sse";
export type CustomMCPAuthType = "none" | "bearer" | "headers";
export type CustomMCPConfigurationState = "ready" | "recovery_required";

/** null 表示该秘密已配置但不会回传明文。 */
export type CustomMCPSecretMap = Record<string, string | null>;

export interface CustomMCPServerInput {
  name: string;
  type: CustomMCPServerType;
  command?: string;
  args?: string[];
  env?: CustomMCPSecretMap;
  url?: string;
  auth_type?: CustomMCPAuthType;
  bearer_token?: string | null;
  headers?: CustomMCPSecretMap;
}

export interface CustomMCPServer extends CustomMCPServerInput {
  connector_id: string;
  configuration_state: CustomMCPConfigurationState;
  enabled: boolean;
}

export type CustomMCPInspectionState =
  | "connected"
  | "disabled"
  | "runtime_only";

export interface CustomMCPToolArgument {
  name: string;
  description?: string;
  required?: boolean;
}

export interface CustomMCPTool {
  name: string;
  title: string;
  description?: string;
  arguments: CustomMCPToolArgument[];
  read_only?: boolean;
}

export interface CustomMCPToolCatalog {
  inspection_state: CustomMCPInspectionState;
  protocol_version?: string;
  server_name?: string;
  server_title?: string;
  server_version?: string;
  instructions?: string;
  supports_tools: boolean;
  tools: CustomMCPTool[];
}

/** 连接器详情 */
export interface ConnectorFeatureDetail {
  name: string;
  description: string;
  items?: string[];
  scopes?: string[];
}

export interface ConnectorDetail extends ConnectorInfo {
  auth_url?: string;
  token_url?: string;
  scopes: string[];
  mcp_server_url?: string;
  docs_url?: string;
  features: string[];
  feature_details?: ConnectorFeatureDetail[];
  oauth_client_id?: string | null;
}

/** OAuth Device Flow 启动信息 */
export type ConnectorDeviceAuthMode =
  | "official_qr"
  | "manual_credentials";
export type ConnectorDeviceAuthStage =
  | "app_selection"
  | "user_authorization";

export interface ConnectorDeviceAuthStart {
  connector_id: string;
  device_code: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete?: string;
  expires_in: number;
  interval: number;
  stage?: ConnectorDeviceAuthStage;
}

/** OAuth Device Flow 轮询状态 */
export type ConnectorDeviceAuthStatus = "pending" | "slow_down" | "connected" | "expired" | "denied";

/** OAuth Device Flow 轮询结果 */
export interface ConnectorDeviceAuthPollResult {
  status: ConnectorDeviceAuthStatus;
  message?: string;
  connector?: ConnectorInfo;
  next?: ConnectorDeviceAuthStart;
}

/** 本机应用批准式配对。attempt_token 是 owner-bound opaque 能力，不包含可读 Token。 */
export interface ConnectorLocalPairingStart {
  connector_id: string;
  attempt_token: string;
  endpoint: string;
  expires_in: number;
  interval: number;
}

export type ConnectorLocalPairingStatus =
  | "pending"
  | "connected"
  | "expired"
  | "denied";

export interface ConnectorLocalPairingPollResult {
  status: ConnectorLocalPairingStatus;
  message?: string;
  connector?: ConnectorInfo;
}

/** 连接器类别 */
export interface ConnectorCategory {
  key: string;
  name: string;
}

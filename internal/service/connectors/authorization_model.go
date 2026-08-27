// INPUT: MCP server 固化身份、人工批准的启动意图与 opaque flow_id。
// OUTPUT: 不含 state、PKCE、device_code、auth_code、token 或内部 session 的公开流程投影。
// POS: Connector 对话授权跨 service/MCP 的稳定数据契约。
package connectors

import "time"

const (
	// AuthorizationMethodOAuthBrowser 通过受保护的本地跳转端点打开 OAuth。
	AuthorizationMethodOAuthBrowser = "oauth_browser"
	// AuthorizationMethodDevice 通过公开 verification URI/user code 完成授权。
	AuthorizationMethodDevice = "device"

	AuthorizationStatusApproved  = "approved"
	AuthorizationStatusPending   = "pending"
	AuthorizationStatusPolling   = "polling"
	AuthorizationStatusConnected = "connected"
	AuthorizationStatusExpired   = "expired"
	AuthorizationStatusDenied    = "denied"
	AuthorizationStatusCanceled  = "canceled"
	AuthorizationStatusConflict  = "conflict"
	AuthorizationStatusFailed    = "failed"

	// ConnectorAuthorizationToolName 是 Connector 授权的统一 MCP 工具叶名。
	ConnectorAuthorizationToolName = "connector_authorization"
	// ConnectorAuthorizationActionStart 是必须由 runtime 真实人工允许的启动 action。
	ConnectorAuthorizationActionStart = "start"
)

// AuthorizationActor 只能由 server/runtime 构造，模型参数不得覆盖这些字段。
type AuthorizationActor struct {
	OwnerUserID            string
	AgentID                string
	BusinessSessionKey     string
	RootRoundID            string
	RuntimeLeaseSessionKey string
	RuntimeLeaseRoundID    string
	PrincipalUserID        string
	PrincipalRole          string
	AuthMethod             string
	AuthSessionID          string
}

// AuthorizationStartRequest 是真实 permission 卡批准并由工具执行的完整意图。
type AuthorizationStartRequest struct {
	RequestID   string              `json:"request_id"`
	ConnectorID string              `json:"connector_id"`
	Method      string              `json:"method"`
	DeviceMode  DeviceAuthStartMode `json:"device_mode,omitempty"`
	Extras      map[string]string   `json:"extras,omitempty"`
}

// AuthorizationFlowRef 约束 status/cancel 必须同时提供 flow 与 Connector。
type AuthorizationFlowRef struct {
	FlowID      string `json:"flow_id"`
	ConnectorID string `json:"connector_id"`
}

// AuthorizationFlowView 是唯一允许返回给 Agent 的流程投影。
type AuthorizationFlowView struct {
	FlowID                      string     `json:"flow_id"`
	ConnectorID                 string     `json:"connector_id"`
	Method                      string     `json:"method"`
	Status                      string     `json:"status"`
	Stage                       string     `json:"stage,omitempty"`
	AuthorizationURL            string     `json:"authorization_url,omitempty"`
	UserCode                    string     `json:"user_code,omitempty"`
	VerificationURI             string     `json:"verification_uri,omitempty"`
	VerificationURIComplete     string     `json:"verification_uri_complete,omitempty"`
	PollAfterSeconds            int        `json:"poll_after_seconds,omitempty"`
	Message                     string     `json:"message,omitempty"`
	StartConfigurationVersion   int64      `json:"start_configuration_version"`
	CurrentConfigurationVersion int64      `json:"current_configuration_version"`
	ExpiresAt                   time.Time  `json:"expires_at"`
	CompletedAt                 *time.Time `json:"completed_at,omitempty"`
}

// AuthorizationCompletionRecord 是统一审计层可读取的 durable 完成证据。
type AuthorizationCompletionRecord struct {
	FlowID                        string     `json:"flow_id"`
	OwnerUserID                   string     `json:"owner_user_id"`
	HumanPrincipalUserID          string     `json:"human_principal_user_id"`
	HumanPrincipalRole            string     `json:"human_principal_role"`
	HumanAuthMethod               string     `json:"human_auth_method"`
	PermissionRequestID           string     `json:"permission_request_id"`
	RequestID                     string     `json:"request_id"`
	AgentID                       string     `json:"agent_id"`
	BusinessSessionKey            string     `json:"business_session_key"`
	RootRoundID                   string     `json:"root_round_id"`
	RuntimeLeaseSessionKey        string     `json:"runtime_lease_session_key"`
	RuntimeLeaseRoundID           string     `json:"runtime_lease_round_id"`
	ConnectorID                   string     `json:"connector_id"`
	Method                        string     `json:"method"`
	StartConfigurationVersion     int64      `json:"start_configuration_version"`
	CompletedConfigurationVersion *int64     `json:"completed_configuration_version,omitempty"`
	Status                        string     `json:"status"`
	HumanApprovedAt               time.Time  `json:"human_approved_at"`
	CompletedAt                   *time.Time `json:"completed_at,omitempty"`
}

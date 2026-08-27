// INPUT: 人工批准、runtime 身份、Connector 版本与加密 provider 临时凭据。
// OUTPUT: 可跨进程恢复但不会直接序列化给模型的授权流程持久模型。
// POS: Connector 对话授权流程的数据库真相模型。
package connectors

import (
	"database/sql"
	"time"
)

// AuthorizationFlow 保存一次由真实人类批准的 OAuth 或 Device 授权流程。
// SecretEncrypted 只能由持有 Connector credentials key 的服务解密。
type AuthorizationFlow struct {
	FlowID                        string
	OwnerUserID                   string
	HumanPrincipalUserID          string
	HumanPrincipalRole            string
	HumanAuthMethod               string
	HumanAuthSessionID            sql.NullString
	PermissionRequestID           string
	RequestID                     string
	AgentID                       string
	BusinessSessionKey            string
	RootRoundID                   string
	RuntimeLeaseSessionKey        string
	RuntimeLeaseRoundID           string
	ConnectorID                   string
	AuthorizationMethod           string
	DeviceMode                    string
	IntentDigest                  string
	StartConfigurationVersion     int64
	ExpectedConfigurationVersion  int64
	CompletedConfigurationVersion sql.NullInt64
	Status                        string
	Stage                         string
	SecretEncrypted               string
	PublicUserCode                string
	PublicVerificationURI         string
	PublicVerificationURIComplete string
	PublicOpenPath                string
	PollIntervalSeconds           int
	ResultMessage                 string
	HumanApprovedAt               time.Time
	OpenedAt                      sql.NullTime
	NextPollAt                    sql.NullTime
	PollClaimUntil                sql.NullTime
	ExpiresAt                     time.Time
	CompletedAt                   sql.NullTime
	CanceledAt                    sql.NullTime
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

// AuthorizationFlowActivation 是 approved -> pending 的公开/加密结果。
type AuthorizationFlowActivation struct {
	ExpectedConfigurationVersion  int64
	Stage                         string
	SecretEncrypted               string
	PublicUserCode                string
	PublicVerificationURI         string
	PublicVerificationURIComplete string
	PublicOpenPath                string
	PollIntervalSeconds           int
	NextPollAt                    sql.NullTime
	ExpiresAt                     time.Time
}

// AuthorizationFlowDeviceStage 是多阶段 Device Flow 的原子推进结果。
type AuthorizationFlowDeviceStage struct {
	ExpectedConfigurationVersion  int64
	Stage                         string
	SecretEncrypted               string
	PublicUserCode                string
	PublicVerificationURI         string
	PublicVerificationURIComplete string
	PollIntervalSeconds           int
	NextPollAt                    time.Time
	ExpiresAt                     time.Time
}

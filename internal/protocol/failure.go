// INPUT: HTTP 或其他产品边界已确认的失败事实。
// OUTPUT: 不包含内部诊断、路由或业务身份的 FailureCore v1 wire 模型。
// POS: 全产品失败事实的最小协议真相；领域状态机仍拥有结果与阶段的决定权。
package protocol

const FailureCoreVersion = 1

// FailureCategory 是跨领域共享的粗粒度失败分类。
//
// Wire 使用开放字符串；新增服务端值不能使旧客户端解码失败。
type FailureCategory string

const (
	FailureCategoryValidation     FailureCategory = "validation"
	FailureCategoryAuthentication FailureCategory = "authentication"
	FailureCategoryAuthorization  FailureCategory = "authorization"
	FailureCategoryNotFound       FailureCategory = "not_found"
	FailureCategoryConflict       FailureCategory = "conflict"
	FailureCategoryRateLimited    FailureCategory = "rate_limited"
	FailureCategoryUnavailable    FailureCategory = "unavailable"
	FailureCategoryTimeout        FailureCategory = "timeout"
	FailureCategoryCanceled       FailureCategory = "canceled"
	FailureCategoryInternal       FailureCategory = "internal"
)

// FailureEffect 只表达已有证据能够证明的本次请求数据影响。
//
// 传输中断不得被推断为 not_applied；没有权威证据时必须使用 unknown。
// 多阶段操作不使用 partial，而由对应领域状态机分别投影各阶段结果。
type FailureEffect string

const (
	FailureEffectNotApplicable FailureEffect = "not_applicable"
	FailureEffectNotApplied    FailureEffect = "not_applied"
	FailureEffectAccepted      FailureEffect = "accepted"
	FailureEffectCommitted     FailureEffect = "committed"
	FailureEffectUnknown       FailureEffect = "unknown"
)

// FailureCore 是失败响应可选携带的最小机器可读事实。
//
// TransportRequestID 只关联一次传输尝试，不参与持久化、授权、路由、缓存、幂等或业务身份。
// Code 是带领域前缀的开放字符串；客户端不认识时必须按 Category 安全回退。
type FailureCore struct {
	Version            int             `json:"version"`
	Code               string          `json:"code"`
	Category           FailureCategory `json:"category"`
	Effect             FailureEffect   `json:"effect"`
	TransportRequestID string          `json:"transport_request_id,omitempty"`
}

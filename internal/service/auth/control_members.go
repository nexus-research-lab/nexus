// INPUT: 宿主绑定的本地 owner、真人 Session 与已批准的成员操作。
// OUTPUT: Control 成员目录与变更结果；不读取 Control 数据库。
// POS: Agent 用户管理到 Control 的服务间适配。
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type ControlMember struct {
	UserID           string    `json:"user_id"`
	Username         string    `json:"username"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role"`
	MembershipStatus string    `json:"membership_status"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateControlMemberInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type UpdateControlMemberInput struct {
	DisplayName *string `json:"display_name,omitempty"`
	Role        *string `json:"role,omitempty"`
	Status      *string `json:"status,omitempty"`
}

func (a *ControlAuthority) ManageMembers(ctx context.Context, owner, sessionID, operation, target string, input json.RawMessage, version int64) (json.RawMessage, error) {
	if sessionID == "" {
		return nil, errors.New("成员管理必须绑定管理员的有效登录 Session")
	}
	binding, err := a.bindings.controlIdentity(ctx, owner)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	err = a.call(ctx, http.MethodPost, "/internal/members/manage", struct {
		ActorUserID     string          `json:"actor_user_id"`
		SessionID       string          `json:"session_id"`
		Operation       string          `json:"operation"`
		Target          string          `json:"target"`
		Input           json.RawMessage `json:"input"`
		ExpectedVersion int64           `json:"expected_version"`
	}{binding.ControlUserID, sessionID, operation, target, input, version}, &result)
	return result, err
}

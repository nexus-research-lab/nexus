// INPUT: 管理员主智能体的可信会话、Control 成员快照和已批准变更。
// OUTPUT: 脱敏用户目录、精确目标校验与 Control 写入。
// POS: members 配置域；复用统一确认、秘密输入和审计链路。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"strings"
)

type memberController interface {
	ManageMembers(context.Context, string, string, string, string, json.RawMessage, int64) (json.RawMessage, error)
}

func (s *Service) memberSnapshot(ctx context.Context, actor *resolvedActor, target string) (any, int64, error) {
	if s.members == nil {
		return nil, 0, errors.New("当前宿主未启用 Control 成员管理")
	}
	data, err := s.members.ManageMembers(ctx, actor.OwnerUserID, actor.AuthSessionID, "list", "", nil, 0)
	if err != nil {
		return nil, 0, err
	}
	var members []authsvc.ControlMember
	if err = json.Unmarshal(data, &members); err != nil {
		return nil, 0, err
	}
	if target == "" {
		return members, 0, nil
	}
	for _, member := range members {
		if member.UserID == target {
			return member, member.UpdatedAt.UnixMicro(), nil
		}
	}
	return nil, 0, errors.New("当前部署中不存在目标用户；请先 inspect members")
}

func validateMemberChange(request ChangeRequest) error {
	switch request.Operation {
	case "create":
		if request.Target != "" {
			return errors.New("members.create 不接受 target")
		}
		var input authsvc.CreateControlMemberInput
		if err := strictDecodeJSON(request.Input, &input); err != nil {
			return err
		}
		if len(strings.TrimSpace(input.Username)) < 3 || input.Password == "" {
			return errors.New("创建账号要求 username 和安全输入的 password")
		}
		if input.Role != "member" && input.Role != "admin" {
			return errors.New("创建角色仅支持 member/admin")
		}
	case "update":
		if request.Target == "" {
			return errors.New("members.update 要求精确 user_id")
		}
		var input authsvc.UpdateControlMemberInput
		if err := strictDecodeJSON(request.Input, &input); err != nil {
			return err
		}
		if input.DisplayName == nil && input.Role == nil && input.Status == nil {
			return errors.New("没有指定用户修改内容")
		}
		if input.Role != nil && *input.Role != "member" && *input.Role != "admin" && *input.Role != "owner" {
			return errors.New("无效成员角色")
		}
		if input.Status != nil && *input.Status != "active" && *input.Status != "revoked" {
			return errors.New("无效成员状态")
		}
		if input.DisplayName != nil && (strings.TrimSpace(*input.DisplayName) == "" || len(*input.DisplayName) > 128) {
			return errors.New("显示名称需为 1-128 字符")
		}
	case "remove":
		if request.Target == "" {
			return errors.New("members.remove 要求精确 user_id")
		}
		if len(request.Input) > 0 {
			return strictDecodeJSON(request.Input, &struct{}{})
		}
	}
	return nil
}

// verifyMemberResult 用写后目录核对实际成员，而非把 HTTP 成功当作完成。
func verifyMemberResult(result any, snapshot DomainSnapshot, operation string) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	var expected authsvc.ControlMember
	if err = json.Unmarshal(payload, &expected); err != nil || expected.UserID == "" {
		return errors.New("Control 成员响应缺少有效 user_id")
	}
	payload, err = json.Marshal(snapshot.Values)
	if err != nil {
		return err
	}
	var actual authsvc.ControlMember
	if operation == "create" {
		var members []authsvc.ControlMember
		if err = json.Unmarshal(payload, &members); err != nil {
			return err
		}
		for _, member := range members {
			if member.UserID == expected.UserID {
				actual = member
				break
			}
		}
	} else if err = json.Unmarshal(payload, &actual); err != nil {
		return err
	}
	if actual.UserID != expected.UserID || actual.Role != expected.Role || actual.DisplayName != expected.DisplayName || actual.MembershipStatus != expected.MembershipStatus || !actual.UpdatedAt.Equal(expected.UpdatedAt) {
		return errors.New("成员写后状态与 Control 返回结果不一致，请重新 inspect 核对")
	}
	return nil
}

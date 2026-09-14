// INPUT: 已规范化的 domain/operation/target/input。
// OUTPUT: 严格字段、必填字段、目标与主机路径预检结果。
// POS: configuration 写入前的纯校验阶段。
package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func requireInputFields(input json.RawMessage, required []string) error {
	if len(required) == 0 {
		return nil
	}
	payload := input
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return fmt.Errorf("input 无效: %w", err)
	}
	missing := make([]string, 0)
	for _, field := range required {
		if _, ok := values[field]; !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("input 缺少必填字段: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateChangeRequest(request ChangeRequest) error {
	switch request.Domain {
	case DomainMembers:
		return validateMemberChange(request)
	case DomainPreferences:
		return validatePreferencesChange(request)
	case DomainProviders:
		return validateProvidersChange(request)
	case DomainAgents:
		return validateAgentsChange(request)
	case DomainEmotion:
		return validateEmotionChange(request)
	case DomainChannels:
		return validateChannelsChange(request)
	case DomainConnectors:
		return validateConnectorsChange(request)
	case DomainSkills:
		return validateSkillsChange(request)
	case DomainSessions:
		return validateSessionsChange(request)
	case DomainRooms:
		return validateRoomsChange(request)
	default:
		return unsupportedChange(request)
	}
}

func jsonFieldProvided(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func requireNonEmptyJSONObject(input json.RawMessage, operation string) error {
	var fields map[string]json.RawMessage
	payload := input
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("input 无效: %w", err)
	}
	if len(fields) == 0 {
		return fmt.Errorf("%s 至少要提供一个待修改字段", operation)
	}
	return nil
}

func (s *Service) validateScopedChange(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	switch request.Domain {
	case DomainProviders:
		return s.validateScopedProvidersChange(ctx, actor, request)
	case DomainRooms:
		return s.validateScopedRoomsChange(ctx, actor, request)
	case DomainAgents:
		return s.validateScopedAgentsChange(ctx, actor, request)
	default:
		return nil
	}
}

func strictDecodeJSON(payload []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("input 只能包含一个 JSON object")
		}
		return err
	}
	return nil
}

func unsupportedChange(request ChangeRequest) error {
	return fmt.Errorf("不支持配置操作 %s.%s", request.Domain, request.Operation)
}

func (request ChangeRequest) requireTarget() error {
	if strings.TrimSpace(request.Target) == "" {
		return fmt.Errorf("%s.%s 要求 target", request.Domain, request.Operation)
	}
	return nil
}

func (request ChangeRequest) decodeInput(destination any) error {
	if len(request.Input) == 0 {
		return strictDecodeJSON([]byte(`{}`), destination)
	}
	if !json.Valid(request.Input) {
		return errors.New("input 必须是 JSON object")
	}
	if err := strictDecodeJSON(request.Input, destination); err != nil {
		return fmt.Errorf("input 无效: %w", err)
	}
	return nil
}

func (request ChangeRequest) decodeAppliedInput(destination any) error {
	payload := request.Input
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return json.Unmarshal(payload, destination)
}

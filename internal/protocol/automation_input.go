package protocol

import (
	"errors"
	"strings"
)

// CreateJobInput 表示创建请求。
type CreateJobInput struct {
	Name          string         `json:"name"`
	AgentID       string         `json:"agent_id"`
	Schedule      Schedule       `json:"schedule"`
	Instruction   string         `json:"instruction"`
	ExecutionKind string         `json:"execution_kind,omitempty"`
	SessionTarget SessionTarget  `json:"session_target"`
	Delivery      DeliveryTarget `json:"delivery"`
	Source        Source         `json:"source"`
	OverlapPolicy string         `json:"overlap_policy,omitempty"`
	Enabled       bool           `json:"enabled"`
}

// Validate 校验创建请求。
func (i CreateJobInput) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(i.AgentID) == "" {
		return errors.New("agent_id is required")
	}
	if strings.TrimSpace(i.Instruction) == "" {
		return errors.New("instruction is required")
	}
	if err := validateExecutionKind(i.ExecutionKind); err != nil {
		return err
	}
	if err := i.Schedule.Normalized().Validate(); err != nil {
		return err
	}
	if err := i.SessionTarget.Normalized().Validate(); err != nil {
		return err
	}
	if err := i.Delivery.Normalized().Validate(); err != nil {
		return err
	}
	if err := i.Source.Normalized().Validate(); err != nil {
		return err
	}
	if err := validateOverlapPolicy(i.OverlapPolicy); err != nil {
		return err
	}
	return nil
}

// Normalized 返回标准化副本。
func (i CreateJobInput) Normalized() CreateJobInput {
	result := i
	result.Name = strings.TrimSpace(result.Name)
	result.AgentID = strings.TrimSpace(result.AgentID)
	result.Instruction = strings.TrimSpace(result.Instruction)
	result.ExecutionKind = NormalizeExecutionKind(result.ExecutionKind)
	result.Schedule = result.Schedule.Normalized()
	result.SessionTarget = result.SessionTarget.Normalized()
	result.Delivery = result.Delivery.Normalized()
	result.Source = result.Source.Normalized()
	result.OverlapPolicy = NormalizeOverlapPolicy(result.OverlapPolicy)
	return result
}

// NormalizeOverlapPolicy 返回重叠触发策略的默认值。
func NormalizeOverlapPolicy(policy string) string {
	normalized := strings.TrimSpace(policy)
	if normalized == "" {
		return OverlapPolicySkip
	}
	return normalized
}

func validateOverlapPolicy(policy string) error {
	switch NormalizeOverlapPolicy(policy) {
	case OverlapPolicySkip, OverlapPolicyAllow:
		return nil
	default:
		return errors.New("overlap_policy must be one of skip, allow")
	}
}

// NormalizeExecutionKind 返回执行体类型的默认值。
func NormalizeExecutionKind(kind string) string {
	normalized := strings.TrimSpace(kind)
	if normalized == "" {
		return ExecutionKindAgent
	}
	return normalized
}

func validateExecutionKind(kind string) error {
	switch NormalizeExecutionKind(kind) {
	case ExecutionKindAgent, ExecutionKindScript:
		return nil
	default:
		return errors.New("execution_kind must be one of agent, script")
	}
}

// UpdateJobInput 表示更新请求。
type UpdateJobInput struct {
	Name          *string         `json:"name,omitempty"`
	Schedule      *Schedule       `json:"schedule,omitempty"`
	Instruction   *string         `json:"instruction,omitempty"`
	ExecutionKind *string         `json:"execution_kind,omitempty"`
	SessionTarget *SessionTarget  `json:"session_target,omitempty"`
	Delivery      *DeliveryTarget `json:"delivery,omitempty"`
	Source        *Source         `json:"source,omitempty"`
	OverlapPolicy *string         `json:"overlap_policy,omitempty"`
	Enabled       *bool           `json:"enabled,omitempty"`
}

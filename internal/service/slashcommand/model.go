package slashcommand

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
)

const (
	modelCommandName         = "model"
	modelCommandDescription  = "Set the AI model for this Agent"
	modelCommandArgumentHint = "<provider>/<model>"
)

// ModelCommandAgentService 提供模型切换所需的最小 Agent 读写能力。
type ModelCommandAgentService interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
	UpdateAgent(
		context.Context,
		string,
		protocol.UpdateRequest,
	) (*protocol.Agent, error)
}

// ModelCommandProviderService 提供当前 runtime 可使用的完整 Provider/模型对。
type ModelCommandProviderService interface {
	ListOptionsForRuntime(
		context.Context,
		string,
	) (*providersvc.OptionsResponse, error)
}

// ModelCommandDependencies 是 `/model` 宿主事务依赖。
type ModelCommandDependencies struct {
	Agents      ModelCommandAgentService
	Preferences runtimeselectionsvc.PreferencesService
	Providers   ModelCommandProviderService
}

type modelCommand struct {
	agents      ModelCommandAgentService
	preferences runtimeselectionsvc.PreferencesService
	providers   ModelCommandProviderService
}

type modelSelection struct {
	Model               string
	ModelDisplayName    string
	Provider            string
	ProviderDisplayName string
}

// RegisterModelCommand 把 Nexus 权威模型切换事务注册为 DM/Room 共用宿主命令。
func RegisterModelCommand(
	registry *Registry,
	dependencies ModelCommandDependencies,
) error {
	if registry == nil {
		return errors.New("slash command registry is nil")
	}
	if dependencies.Agents == nil ||
		dependencies.Preferences == nil ||
		dependencies.Providers == nil {
		return errors.New("model command dependencies are incomplete")
	}
	command := &modelCommand{
		agents:      dependencies.Agents,
		preferences: dependencies.Preferences,
		providers:   dependencies.Providers,
	}
	return registry.Register(Definition{
		Name:         modelCommandName,
		Description:  modelCommandDescription,
		ArgumentHint: modelCommandArgumentHint,
		Scopes:       []Scope{ScopeDM, ScopeRoom},
		Enabled:      true,
		Handler:      command.execute,
	})
}

func (c *modelCommand) execute(
	ctx context.Context,
	invocation Invocation,
) (Result, error) {
	agentID := strings.TrimSpace(invocation.AgentID)
	if agentID == "" {
		return Result{}, errors.New("model command requires an agent")
	}
	argument := strings.TrimSpace(invocation.Arguments)
	if argument == "" {
		return Result{}, commandInputError{
			message: "用法：/model <provider>/<model>",
		}
	}
	agentValue, err := c.agents.GetAgent(ctx, agentID)
	if err != nil {
		return Result{}, err
	}
	runtimeSelection, err := runtimeselectionsvc.NewService(
		c.preferences,
	).Resolve(ctx, runtimeselectionsvc.Request{Agent: agentValue})
	if err != nil {
		return Result{}, err
	}
	options, err := c.providers.ListOptionsForRuntime(
		ctx,
		runtimeSelection.RuntimeKind,
	)
	if err != nil {
		return Result{}, err
	}
	selection, resolution := resolveModelCommandSelection(options, argument)
	switch resolution {
	case modelResolutionPassThrough:
		if isClaudeRuntimeKind(runtimeSelection.RuntimeKind) {
			return Result{PassThrough: true}, nil
		}
		return Result{}, commandInputError{
			message: fmt.Sprintf("当前 runtime 下找不到模型 %q。", argument),
		}
	case modelResolutionAmbiguous:
		return Result{}, commandInputError{
			message: fmt.Sprintf(
				"模型 %q 存在于多个 Provider，请使用 /model <provider>/<model>。",
				argument,
			),
		}
	case modelResolutionMissing:
		return Result{}, commandInputError{
			message: fmt.Sprintf("当前 runtime 下找不到模型 %q。", argument),
		}
	}
	updated, err := c.agents.UpdateAgent(ctx, agentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			Provider: selection.Provider,
			Model:    selection.Model,
		},
	})
	if err != nil {
		return Result{}, err
	}
	return Result{
		Events: []protocol.EventMessage{
			newModelChangedEvent(invocation, updated, selection),
		},
		DirectoryInvalidation: &DirectoryInvalidation{
			Reason: "agent_updated",
			Data: map[string]any{
				"agent_id": agentID,
				"model":    selection.Model,
				"provider": selection.Provider,
			},
		},
	}, nil
}

type modelResolution uint8

const (
	modelResolutionMatched modelResolution = iota
	modelResolutionPassThrough
	modelResolutionAmbiguous
	modelResolutionMissing
)

func resolveModelCommandSelection(
	options *providersvc.OptionsResponse,
	argument string,
) (modelSelection, modelResolution) {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return modelSelection{}, modelResolutionMissing
	}

	matches := findModelCommandMatches(options, argument)
	if len(matches) == 1 {
		return matches[0], modelResolutionMatched
	}
	if len(matches) > 1 {
		return modelSelection{}, modelResolutionAmbiguous
	}

	providerName, modelName, qualified := strings.Cut(argument, "/")
	if !qualified {
		return modelSelection{}, modelResolutionPassThrough
	}
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	if providerName == "" || modelName == "" {
		return modelSelection{}, modelResolutionMissing
	}
	for _, provider := range optionsForModelCommand(options) {
		if !modelCommandValueMatches(
			providerName,
			provider.Provider,
			provider.DisplayName,
		) {
			continue
		}
		for _, model := range provider.Models {
			if modelCommandValueMatches(
				modelName,
				model.ModelID,
				model.DisplayName,
			) {
				return newModelSelection(provider, model), modelResolutionMatched
			}
		}
	}
	return modelSelection{}, modelResolutionMissing
}

func findModelCommandMatches(
	options *providersvc.OptionsResponse,
	argument string,
) []modelSelection {
	matches := make([]modelSelection, 0, 1)
	for _, provider := range optionsForModelCommand(options) {
		for _, model := range provider.Models {
			if !modelCommandValueMatches(
				argument,
				model.ModelID,
				model.DisplayName,
			) {
				continue
			}
			matches = append(matches, newModelSelection(provider, model))
		}
	}
	return matches
}

func optionsForModelCommand(
	options *providersvc.OptionsResponse,
) []providersvc.Option {
	if options == nil {
		return nil
	}
	return options.Items
}

func newModelSelection(
	provider providersvc.Option,
	model providersvc.ModelOption,
) modelSelection {
	return modelSelection{
		Provider: provider.Provider,
		ProviderDisplayName: firstModelCommandValue(
			provider.DisplayName,
			provider.Provider,
		),
		Model: model.ModelID,
		ModelDisplayName: firstModelCommandValue(
			model.DisplayName,
			model.ModelID,
		),
	}
}

func modelCommandValueMatches(target string, candidates ...string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, candidate := range candidates {
		if strings.EqualFold(target, strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func firstModelCommandValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func isClaudeRuntimeKind(runtimeKind string) bool {
	switch strings.ToLower(strings.TrimSpace(runtimeKind)) {
	case "claude", "cc", "claude-code", "claudecode":
		return true
	default:
		return false
	}
}

func newModelChangedEvent(
	invocation Invocation,
	agentValue *protocol.Agent,
	selection modelSelection,
) protocol.EventMessage {
	timestamp := time.Now().UnixMilli()
	messageID := protocol.NewAssistantMessageID()
	agentID := strings.TrimSpace(invocation.AgentID)
	if agentValue != nil && strings.TrimSpace(agentValue.AgentID) != "" {
		agentID = strings.TrimSpace(agentValue.AgentID)
	}
	text := fmt.Sprintf(
		"Set model to %s / %s",
		selection.ProviderDisplayName,
		selection.ModelDisplayName,
	)
	event := protocol.NewEvent(protocol.EventTypeMessage, map[string]any{
		"message_id":  messageID,
		"session_key": strings.TrimSpace(invocation.SessionKey),
		"agent_id":    agentID,
		"round_id":    strings.TrimSpace(invocation.RoundID),
		"role":        "assistant",
		"timestamp":   timestamp,
		"content": []map[string]any{{
			"type": "text",
			"text": text,
		}},
	})
	event.SessionKey = strings.TrimSpace(invocation.SessionKey)
	event.AgentID = agentID
	event.MessageID = messageID
	event.RoundID = strings.TrimSpace(invocation.RoundID)
	event.DeliveryMode = "ephemeral"
	return event
}

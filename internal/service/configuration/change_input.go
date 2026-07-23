// INPUT: 对话友好的 merge-patch 输入与现有领域记录。
// OUTPUT: 不重置未声明字段的 Provider、Agent 与 Preferences 服务输入。
// POS: configuration 对话补丁到既有完整快照 API 的适配层。
package configuration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type providerModelMutation struct {
	ModelID string                       `json:"model_id"`
	Input   providersvc.UpdateModelInput `json:"input"`
}

type providerCreateRequest struct {
	ProviderKind string `json:"provider_kind"`
	Provider     string `json:"provider"`
	Visibility   string `json:"visibility,omitempty"`
	PresetKey    string `json:"preset_key"`
	APIFormat    string `json:"api_format"`
	DisplayName  string `json:"display_name"`
	AuthToken    string `json:"auth_token"`
	BaseURL      string `json:"base_url"`
	ModelsPath   string `json:"models_path"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

func (r providerCreateRequest) serviceInput() providersvc.CreateInput {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return providersvc.CreateInput{
		ProviderKind: r.ProviderKind, Provider: r.Provider, Visibility: r.Visibility,
		PresetKey: r.PresetKey, APIFormat: r.APIFormat, DisplayName: r.DisplayName,
		AuthToken: r.AuthToken, BaseURL: r.BaseURL, ModelsPath: r.ModelsPath, Enabled: enabled,
	}
}

type providerUpdateRequest struct {
	ProviderKind *string `json:"provider_kind,omitempty"`
	PresetKey    *string `json:"preset_key,omitempty"`
	APIFormat    *string `json:"api_format,omitempty"`
	DisplayName  *string `json:"display_name,omitempty"`
	AuthToken    *string `json:"auth_token,omitempty"`
	BaseURL      *string `json:"base_url,omitempty"`
	ModelsPath   *string `json:"models_path,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

func (r providerUpdateRequest) serviceInput(current providersvc.Record) providersvc.UpdateInput {
	input := providersvc.UpdateInput{
		ProviderKind: current.ProviderKind, PresetKey: current.PresetKey,
		APIFormat: current.APIFormat, DisplayName: current.DisplayName,
		BaseURL: current.BaseURL, ModelsPath: current.ModelsPath, Enabled: current.Enabled,
		AuthToken: r.AuthToken,
	}
	if r.ProviderKind != nil {
		input.ProviderKind = *r.ProviderKind
	}
	if r.PresetKey != nil {
		input.PresetKey = *r.PresetKey
	}
	if r.APIFormat != nil {
		input.APIFormat = *r.APIFormat
	}
	if r.DisplayName != nil {
		input.DisplayName = *r.DisplayName
	}
	if r.BaseURL != nil {
		input.BaseURL = *r.BaseURL
	}
	if r.ModelsPath != nil {
		input.ModelsPath = *r.ModelsPath
	}
	if r.Enabled != nil {
		input.Enabled = *r.Enabled
	}
	return input
}

type agentUpdatePatch struct {
	Name        *string         `json:"name,omitempty"`
	Options     json.RawMessage `json:"options,omitempty"`
	Avatar      *string         `json:"avatar,omitempty"`
	Description *string         `json:"description,omitempty"`
	VibeTags    []string        `json:"vibe_tags,omitempty"`
}

type providerModelTarget struct {
	ModelID string `json:"model_id"`
}

type channelAccountTarget struct {
	AccountID string `json:"account_id"`
}

type connectorCredentials struct {
	Credentials map[string]string `json:"credentials"`
}

type skillAgentTarget struct {
	AgentID string `json:"agent_id"`
}

func mergedPreferencesUpdate(
	previous preferencessvc.Preferences,
	parsed preferencessvc.UpdateRequest,
	rawInput json.RawMessage,
) (preferencessvc.UpdateRequest, error) {
	merged, err := mergeJSONObject(previous, rawInput, "web_search_api_key")
	if err != nil {
		return preferencessvc.UpdateRequest{}, err
	}
	var values preferencessvc.Preferences
	if err = strictDecodeJSON(merged, &values); err != nil {
		return preferencessvc.UpdateRequest{}, fmt.Errorf("合并 preferences patch: %w", err)
	}
	return preferencessvc.UpdateRequest{
		ChatDefaultDeliveryPolicy:       &values.ChatDefaultDeliveryPolicy,
		AgentRuntimeKind:                &values.AgentRuntimeKind,
		AgentSDKDiagnosticsEnabled:      &values.AgentSDKDiagnosticsEnabled,
		RuntimeSettings:                 &values.RuntimeSettings,
		WebSearch:                       &values.WebSearch,
		WebSearchAPIKey:                 parsed.WebSearchAPIKey,
		DefaultAgentOptions:             &values.DefaultAgentOptions,
		DefaultImageModelSelection:      &values.DefaultImageModelSelection,
		DefaultVisionModelSelection:     &values.DefaultVisionModelSelection,
		DefaultBackgroundModelSelection: &values.DefaultBackgroundModelSelection,
	}, nil
}

func (s *Service) agentUpdateInput(
	ctx context.Context,
	agentID string,
	patch agentUpdatePatch,
) (protocol.UpdateRequest, error) {
	result := protocol.UpdateRequest{
		Name: patch.Name, Avatar: patch.Avatar, Description: patch.Description, VibeTags: patch.VibeTags,
	}
	if len(patch.Options) == 0 || string(patch.Options) == "null" {
		return result, nil
	}
	current, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return protocol.UpdateRequest{}, err
	}
	merged, err := mergeJSONObject(current.Options, patch.Options)
	if err != nil {
		return protocol.UpdateRequest{}, err
	}
	var options protocol.Options
	if err = strictDecodeJSON(merged, &options); err != nil {
		return protocol.UpdateRequest{}, fmt.Errorf("合并 Agent options patch: %w", err)
	}
	result.Options = &options
	return result, nil
}

func mergeJSONObject(current any, patch json.RawMessage, excludedKeys ...string) (json.RawMessage, error) {
	currentPayload, err := json.Marshal(current)
	if err != nil {
		return nil, err
	}
	var currentMap map[string]any
	if err = json.Unmarshal(currentPayload, &currentMap); err != nil {
		return nil, err
	}
	var patchMap map[string]any
	if len(patch) == 0 {
		patch = json.RawMessage(`{}`)
	}
	if err = json.Unmarshal(patch, &patchMap); err != nil {
		return nil, err
	}
	for _, key := range excludedKeys {
		delete(patchMap, key)
	}
	mergeMap(currentMap, patchMap)
	return json.Marshal(currentMap)
}

func mergeMap(current map[string]any, patch map[string]any) {
	for key, patchValue := range patch {
		if patchValue == nil {
			delete(current, key)
			continue
		}
		patchMap, patchIsMap := patchValue.(map[string]any)
		currentMap, currentIsMap := current[key].(map[string]any)
		if patchIsMap && currentIsMap {
			mergeMap(currentMap, patchMap)
			current[key] = currentMap
			continue
		}
		current[key] = patchValue
	}
}

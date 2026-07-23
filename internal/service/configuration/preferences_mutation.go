// INPUT: Preferences merge patch、Provider 模型目录与 runtime manager。
// OUTPUT: 与 Web 设置一致的持久化默认值、WebSearch 热同步与失败回滚。
// POS: configuration 的 Preferences 事务式业务阶段。
package configuration

import (
	"context"
	"encoding/json"
	"errors"

	clientopts "github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

func (s *Service) updatePreferences(
	ctx context.Context,
	actor Actor,
	input preferencessvc.UpdateRequest,
	rawInput json.RawMessage,
) (preferencessvc.Preferences, error) {
	previous, err := s.prefs.Get(ctx, actor.OwnerUserID)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	input, err = mergedPreferencesUpdate(previous, input, rawInput)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	updated, err := s.prefs.Update(ctx, actor.OwnerUserID, input)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	updated, err = s.reconcileProviderPreferenceDefaults(ctx, actor.OwnerUserID, updated)
	if err != nil {
		restoreErr := s.restorePreferences(ctx, actor.OwnerUserID, previous)
		return preferencessvc.Preferences{}, errors.Join(err, restoreErr)
	}
	if s.runtime == nil || s.agents == nil || (input.WebSearch == nil && input.WebSearchAPIKey == nil) {
		return updated, nil
	}
	if err = s.syncWebSearchRuntime(ctx, updated); err == nil {
		return updated, nil
	}
	restoreErr := s.restorePreferences(ctx, actor.OwnerUserID, previous)
	runtimeRestoreErr := s.syncWebSearchRuntime(ctx, previous)
	return preferencessvc.Preferences{}, errors.Join(err, restoreErr, runtimeRestoreErr)
}

func (s *Service) reconcileProviderPreferenceDefaults(
	ctx context.Context,
	ownerUserID string,
	preferences preferencessvc.Preferences,
) (preferencessvc.Preferences, error) {
	if s.providers == nil {
		return preferences, nil
	}
	options, err := s.providers.ListOptionsForRuntime(ctx, preferences.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	adjusted, changed := preferencessvc.ReconcileImagegenDefaultTool(
		preferences,
		options.HasConfiguredImageSelection(
			preferences.DefaultImageModelSelection.Provider,
			preferences.DefaultImageModelSelection.Model,
		),
	)
	if !changed {
		return adjusted, nil
	}
	return s.prefs.Update(ctx, ownerUserID, preferencessvc.UpdateRequest{
		DefaultAgentOptions: &adjusted.DefaultAgentOptions,
	})
}

func (s *Service) restorePreferences(
	ctx context.Context,
	ownerUserID string,
	previous preferencessvc.Preferences,
) error {
	webSearchAPIKey := previous.WebSearchAPIKey()
	_, err := s.prefs.Update(ctx, ownerUserID, preferencessvc.UpdateRequest{
		ChatDefaultDeliveryPolicy:       &previous.ChatDefaultDeliveryPolicy,
		AgentRuntimeKind:                &previous.AgentRuntimeKind,
		AgentSDKDiagnosticsEnabled:      &previous.AgentSDKDiagnosticsEnabled,
		RuntimeSettings:                 &previous.RuntimeSettings,
		WebSearch:                       &previous.WebSearch,
		WebSearchAPIKey:                 &webSearchAPIKey,
		DefaultAgentOptions:             &previous.DefaultAgentOptions,
		DefaultImageModelSelection:      &previous.DefaultImageModelSelection,
		DefaultVisionModelSelection:     &previous.DefaultVisionModelSelection,
		DefaultBackgroundModelSelection: &previous.DefaultBackgroundModelSelection,
	})
	return err
}

func (s *Service) syncWebSearchRuntime(ctx context.Context, preferences preferencessvc.Preferences) error {
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	environment := clientopts.BuildWebSearchRuntimeEnv("nxs", preferences.WebSearch)
	for _, item := range agents {
		if err = s.runtime.UpdateEnvironmentForAgent(ctx, item.AgentID, environment); err != nil {
			return err
		}
	}
	return nil
}

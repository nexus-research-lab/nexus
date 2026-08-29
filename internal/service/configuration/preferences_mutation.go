// INPUT: Preferences merge patch、Provider 模型目录与 runtime manager。
// OUTPUT: 与 Web 设置一致的持久化默认值、运行偏好热同步与失败回滚。
// POS: configuration 的 Preferences 事务式业务阶段。
package configuration

import (
	"context"
	"encoding/json"
	"errors"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	"github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
)

func (s *Service) updatePreferences(
	ctx context.Context,
	actor Actor,
	input preferencessvc.UpdateRequest,
	rawInput json.RawMessage,
	stateVersion int64,
) (preferencessvc.Preferences, error) {
	runtimePreferencesChanged := inputContainsField(rawInput, "runtime_settings") ||
		inputContainsField(rawInput, "web_search") ||
		inputContainsField(rawInput, "web_search_api_key")
	var previous preferencessvc.Preferences
	updated, err := s.prefs.UpdatePreparedAtVersion(
		ctx,
		actor.OwnerUserID,
		stateVersion,
		func(current preferencessvc.Preferences) (preferencessvc.UpdateRequest, error) {
			previous = current
			merged, mergeErr := mergedPreferencesUpdate(current, input, rawInput)
			if mergeErr != nil {
				return preferencessvc.UpdateRequest{}, mergeErr
			}
			return s.reconcileProviderPreferenceDefaults(ctx, merged)
		},
	)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	if !runtimePreferencesChanged {
		return updated, nil
	}
	if s.runtime == nil || s.agents == nil {
		reconcileErr := errors.New("运行偏好已写入，但活跃 runtime 同步依赖未装配")
		_, _, restoreErr := s.prefs.RestoreIfVersion(
			ctx,
			actor.OwnerUserID,
			updated.Version,
			previous,
		)
		return preferencessvc.Preferences{}, errors.Join(reconcileErr, restoreErr)
	}
	if err = s.syncRuntimePreferences(ctx, updated); err == nil {
		return updated, nil
	}
	restoredValue, restored, restoreErr := s.prefs.RestoreIfVersion(
		ctx,
		actor.OwnerUserID,
		updated.Version,
		previous,
	)
	if restoreErr != nil {
		return preferencessvc.Preferences{}, errors.Join(err, restoreErr)
	}
	var rollbackStateErr error
	if !restored {
		rollbackStateErr = errors.New("运行偏好同步失败后的回滚已跳过：Preferences 已有后续写入")
	}
	runtimeRestoreErr := s.syncRuntimePreferences(ctx, restoredValue)
	return preferencessvc.Preferences{}, errors.Join(err, rollbackStateErr, runtimeRestoreErr)
}

func (s *Service) reconcileProviderPreferenceDefaults(
	ctx context.Context,
	request preferencessvc.UpdateRequest,
) (preferencessvc.UpdateRequest, error) {
	if s.providers == nil {
		return request, nil
	}
	if request.AgentRuntimeKind == nil ||
		request.DefaultAgentOptions == nil ||
		request.DefaultImageModelSelection == nil {
		return preferencessvc.UpdateRequest{}, errors.New("合并后的 Preferences 缺少默认 runtime 或模型字段")
	}
	options, err := s.providers.ListOptionsForRuntime(ctx, *request.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.UpdateRequest{}, err
	}
	preferences := preferencessvc.Preferences{
		DefaultAgentOptions:        *request.DefaultAgentOptions,
		DefaultImageModelSelection: *request.DefaultImageModelSelection,
	}
	adjusted, changed := preferencessvc.ReconcileImagegenDefaultTool(
		preferences,
		options.HasConfiguredImageSelection(
			preferences.DefaultImageModelSelection.Provider,
			preferences.DefaultImageModelSelection.Model,
		),
	)
	if !changed {
		return request, nil
	}
	request.DefaultAgentOptions = &adjusted.DefaultAgentOptions
	return request, nil
}

func (s *Service) syncRuntimePreferences(ctx context.Context, preferences preferencessvc.Preferences) error {
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	runtimeEnvironment := runtimeselection.RuntimeEnvironmentFromPreferences(preferences)
	errs := make([]error, 0)
	for _, item := range agents {
		if err = s.runtime.UpdateEnvironmentForAgent(ctx, item.AgentID, runtimeEnvironment); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

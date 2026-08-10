package core

import (
	"net/http"
	"strings"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func (h *Handlers) withProviderPreferenceDefaults(
	request *http.Request,
	prefs preferencessvc.Preferences,
) (preferencessvc.Preferences, error) {
	if h.providers == nil {
		return prefs, nil
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), prefs.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	adjusted, _ := applyImagegenDefaultTool(prefs, providerOptions)
	return adjusted, nil
}

func (h *Handlers) prepareProviderPreferenceDefaults(
	request *http.Request,
	current preferencessvc.Preferences,
	update preferencessvc.UpdateRequest,
) (preferencessvc.UpdateRequest, error) {
	if h.providers == nil {
		return update, nil
	}
	effective := current
	if update.AgentRuntimeKind != nil {
		effective.AgentRuntimeKind = *update.AgentRuntimeKind
	}
	if update.DefaultAgentOptions != nil {
		effective.DefaultAgentOptions = *update.DefaultAgentOptions
	}
	if update.DefaultImageModelSelection != nil {
		effective.DefaultImageModelSelection = *update.DefaultImageModelSelection
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), effective.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.UpdateRequest{}, err
	}
	adjusted, changed := applyImagegenDefaultTool(effective, providerOptions)
	if !changed {
		return update, nil
	}
	update.DefaultAgentOptions = &adjusted.DefaultAgentOptions
	return update, nil
}

func updatedDefaultAgentSelection(
	previous preferencessvc.Preferences,
	payload preferencessvc.UpdateRequest,
) (providercfg.DefaultAgentSelection, bool) {
	selection := providercfg.DefaultAgentSelection{
		Provider:    strings.TrimSpace(previous.DefaultAgentOptions.Provider),
		Model:       strings.TrimSpace(previous.DefaultAgentOptions.Model),
		RuntimeKind: strings.TrimSpace(previous.AgentRuntimeKind),
	}
	if payload.DefaultAgentOptions != nil {
		selection.Provider = strings.TrimSpace(payload.DefaultAgentOptions.Provider)
		selection.Model = strings.TrimSpace(payload.DefaultAgentOptions.Model)
	}
	if payload.AgentRuntimeKind != nil {
		selection.RuntimeKind = strings.TrimSpace(*payload.AgentRuntimeKind)
	}
	changed := selection.Provider != strings.TrimSpace(previous.DefaultAgentOptions.Provider) ||
		selection.Model != strings.TrimSpace(previous.DefaultAgentOptions.Model) ||
		selection.RuntimeKind != strings.TrimSpace(previous.AgentRuntimeKind)
	return selection, changed
}

func applyImagegenDefaultTool(
	prefs preferencessvc.Preferences,
	providerOptions *providercfg.OptionsResponse,
) (preferencessvc.Preferences, bool) {
	enabled := hasConfiguredImageModel(prefs, providerOptions)
	return preferencessvc.ReconcileImagegenDefaultTool(prefs, enabled)
}

func hasConfiguredImageModel(prefs preferencessvc.Preferences, providerOptions *providercfg.OptionsResponse) bool {
	return providerOptions.HasConfiguredImageSelection(
		prefs.DefaultImageModelSelection.Provider,
		prefs.DefaultImageModelSelection.Model,
	)
}

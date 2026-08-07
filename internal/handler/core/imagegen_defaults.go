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

func (h *Handlers) persistProviderPreferenceDefaults(
	request *http.Request,
	prefs preferencessvc.Preferences,
) (preferencessvc.Preferences, error) {
	if h.prefs == nil || h.providers == nil {
		return prefs, nil
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), prefs.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	adjusted, changed := applyImagegenDefaultTool(prefs, providerOptions)
	if !changed {
		return adjusted, nil
	}
	return h.prefs.Update(request.Context(), currentOwnerUserID(request), preferencessvc.UpdateRequest{
		DefaultAgentOptions: &adjusted.DefaultAgentOptions,
	})
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

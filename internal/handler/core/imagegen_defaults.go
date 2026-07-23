package core

import (
	"net/http"

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

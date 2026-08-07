package provider

import (
	"testing"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestCCSwitchDefaultPreferencesUpdateConfiguresBackgroundModel(t *testing.T) {
	prefs := preferencessvc.DefaultPreferences()
	selection := providercfg.ModelSelection{
		Provider: "ccswitch-claude-kimi",
		Model:    "kimi-for-coding",
	}

	request := ccSwitchDefaultPreferencesUpdate(prefs, selection)
	if request.DefaultAgentOptions == nil ||
		request.DefaultAgentOptions.Provider != selection.Provider ||
		request.DefaultAgentOptions.Model != selection.Model {
		t.Fatalf("默认对话模型未同步: %+v", request.DefaultAgentOptions)
	}
	if request.DefaultBackgroundModelSelection == nil ||
		request.DefaultBackgroundModelSelection.Provider != selection.Provider ||
		request.DefaultBackgroundModelSelection.Model != selection.Model {
		t.Fatalf("后台任务模型未同步: %+v", request.DefaultBackgroundModelSelection)
	}
}

package memorymaintenance

// 本文件只做宿主侧廉价 enabled 预检，nxs 仍会再次执行完整 gate 判断。

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func autoDreamEnabled(catalog agentCatalog, agentValue protocol.Agent) (bool, error) {
	if err := catalog.EnsureRuntimeSettingsProjection(agentValue); err != nil {
		return false, err
	}
	settings, err := catalog.LoadRuntimeSettingsProjection(agentValue)
	if err != nil {
		return false, err
	}
	if memorySettings, ok := settings["memory"].(map[string]any); ok {
		if dreamSettings, ok := memorySettings["dream"].(map[string]any); ok {
			if enabled, ok := dreamSettings["enabled"].(bool); ok {
				return enabled, nil
			}
		}
		if enabled, ok := memorySettings["dreamEnabled"].(bool); ok {
			return enabled, nil
		}
	}
	enabled, _ := settings["autoDreamEnabled"].(bool)
	return enabled, nil
}

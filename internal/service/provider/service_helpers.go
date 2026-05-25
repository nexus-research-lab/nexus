package provider

import (
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func filterAgentRuntimeProviders(items []providerstore.Entity) []providerstore.Entity {
	result := make([]providerstore.Entity, 0, len(items))
	for _, item := range items {
		if isAgentRuntimeProvider(item) {
			result = append(result, item)
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func boolPointer(value bool) *bool {
	return &value
}

package imagegen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

var pixelSizePattern = regexp.MustCompile(`^(\d+)\s*[xX*]\s*(\d+)$`)

func applyGenerateProviderDefaults(config *providercfg.ImageConfig, input GenerateInput) GenerateInput {
	if input.Size == defaultSize {
		if config != nil {
			if size := stringProviderOption(config.ProviderOptions, "size"); size != "" {
				input.Size = size
				return input
			}
		}
		if isSeedreamModel(config) {
			input.Size = "2K"
		}
	}
	return input
}

func cloneProviderOptions(options map[string]any) map[string]any {
	fields := map[string]any{}
	for key, value := range options {
		if strings.TrimSpace(key) != "" {
			fields[key] = value
		}
	}
	return fields
}

func stringProviderOption(options map[string]any, key string) string {
	value, ok := options[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func isSeedreamModel(config *providercfg.ImageConfig) bool {
	if config == nil {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(config.Model))
	return strings.Contains(model, "seedream")
}

func normalizeProviderImageSize(config *providercfg.ImageConfig, size string) string {
	size = strings.TrimSpace(size)
	if size == "" || !isImage2Model(config) {
		return size
	}
	matches := pixelSizePattern.FindStringSubmatch(size)
	if len(matches) != 3 {
		return size
	}
	width, widthErr := strconv.Atoi(matches[1])
	height, heightErr := strconv.Atoi(matches[2])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return size
	}
	width = nearestPositiveMultiple(width, 16)
	height = nearestPositiveMultiple(height, 16)
	return fmt.Sprintf("%dx%d", width, height)
}

func isImage2Model(config *providercfg.ImageConfig) bool {
	if config == nil {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(config.Model))
	normalized := strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(model)
	return strings.Contains(normalized, "image2")
}

func nearestPositiveMultiple(value int, multiple int) int {
	if multiple <= 0 || value <= 0 {
		return value
	}
	remainder := value % multiple
	if remainder == 0 {
		return value
	}
	down := value - remainder
	up := down + multiple
	if down <= 0 || up-value <= value-down {
		return up
	}
	return down
}

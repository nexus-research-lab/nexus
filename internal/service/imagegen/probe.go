// INPUT: Exact image route plus synthetic generation/edit challenge, never user files.
// OUTPUT: Bytes from the production provider adapters, without persisting artifacts.
// POS: Provider capability probe adapter; submission is single-attempt and caller-bounded.
package imagegen

import (
	"context"
	"errors"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func (s *Service) ProbeImage(ctx context.Context, request providercfg.ImageProbeRequest) ([]byte, error) {
	probe := *s
	probe.singleAttempt = true
	config := request.Config
	if request.Edit {
		input := EditInput{Model: config.Model, Prompt: request.Prompt, Size: defaultSize, OutputFormat: defaultOutputFormat, imageData: request.Image}
		input.Size = normalizeProviderImageSize(&config, input.Size)
		payload, _, _, err := probe.callEditProvider(ctx, &config, input)
		return payload, imageProbeError(err, request.Edit)
	}
	input := GenerateInput{Model: config.Model, Prompt: request.Prompt, Size: defaultSize, OutputFormat: defaultOutputFormat}
	input = applyGenerateProviderDefaults(&config, input)
	input.Size = normalizeProviderImageSize(&config, input.Size)
	payload, _, _, err := probe.callGenerateProvider(ctx, &config, input)
	return payload, imageProbeError(err, request.Edit)
}

// imageProbeError 只将精确的能力拒绝码转成否定，其余错误保留为未完成。
func imageProbeError(err error, edit bool) error {
	if err == nil {
		return nil
	}
	result := providercfg.CapabilityProbeResult{State: "error", Reason: "image_request_failed"}
	var response *imageResponseError
	if errors.As(err, &response) {
		switch response.status {
		case 401, 403:
			result.Reason = "authentication_or_permission"
		case 429:
			result.Reason = "quota_or_rate_limit"
		case 400, 422:
			if (!edit && response.code == "image_generation_not_supported") || (edit && response.code == "image_editing_not_supported") {
				result.State = "unsupported"
				result.Reason = "explicit_protocol_denial"
			}
		}
	}
	return &providercfg.ProbeExecutionError{Result: result}
}

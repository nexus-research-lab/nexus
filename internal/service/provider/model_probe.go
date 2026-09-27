// INPUT: Explicit model test, exact configuration version and optional single capability.
// OUTPUT: Independent, configuration-bound observations; failed checks never become denials.
// POS: Probe orchestration and shared bounded HTTP transport, not an Agent Session.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const capabilityProbeTimeout = 25 * time.Second

func (s *Service) runModelTest(ctx context.Context, item providerstore.Entity, modelID string, expectedVersion int64) (*TestResult, error) {
	return s.runCapabilityTests(ctx, item, modelID, expectedVersion, "")
}

func (s *Service) runCapabilityTests(ctx context.Context, item providerstore.Entity, modelID string, version int64, capability string) (*TestResult, error) {
	if version != item.ConfigurationVersion {
		return nil, ErrConfigurationVersionConflict
	}
	modelID = normalizeModelID(modelID)
	if modelID == "" || (capability != "" && capability != "all" && !validProbeCapability(capability)) {
		return nil, fmt.Errorf("%w: invalid model or capability", ErrInvalidInput)
	}
	model, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		model = &providerstore.ModelEntity{ProviderID: item.ID, ModelID: modelID, ProviderOptionsJSON: "{}"}
	}
	evidence := modelProbeEvidence{Version: modelProbeVersion, Fingerprint: modelProbeFingerprint(item, *model), TestedAt: s.now(), Attempts: map[string]CapabilityProbeResult{}, Verified: map[string]CapabilityProbeResult{}}
	if previous := currentModelProbe(item, *model); previous != nil {
		evidence = *previous
		evidence.TestedAt = s.now()
	}
	if evidence.Attempts == nil {
		evidence.Attempts = map[string]CapabilityProbeResult{}
	}
	if evidence.Verified == nil {
		evidence.Verified = map[string]CapabilityProbeResult{}
	}
	results := map[string]CapabilityProbeResult{}
	record := func(key string, result CapabilityProbeResult) {
		result.TestedAt = s.now()
		results[key] = result
		evidence.Attempts[key] = result
		evidence.Capabilities = mergeObservedCapabilities(evidence.Capabilities, probeCapabilities(key, result))
		if result.State == "supported" || result.State == "unsupported" {
			evidence.Verified[key] = result
		}
	}
	// Explicit all is used by list synchronization and preserves saved selection.
	evidence.PreserveSelection = capability == "all"
	evidence.BaselineAutoJSON = model.CapabilitiesAutoJSON
	evidence.BaselineModelID = model.ID
	keys := []string{capability}
	if capability == "" || capability == "all" {
		keys = probeCapabilityKeys
	}
	type check struct {
		key    string
		result CapabilityProbeResult
	}
	checks := make(chan check, len(keys))
	for _, key := range keys {
		go func(key string) {
			checks <- check{key, s.probeCapability(ctx, item, *model, key)}
		}(key)
	}
	for range keys {
		result := <-checks
		record(result.key, result.result)
	}
	var testErr error
	primary := capability
	if primary == "" || primary == "all" {
		primary = "text_output"
		if item.ProviderKind == ProviderKindImageGeneration {
			primary = "image_output"
		}
		if model.Category == "embedding" {
			primary = "embedding"
		}
	}
	if result := results[primary]; result.State != "supported" {
		testErr = fmt.Errorf("能力测试未完成: %s", result.Reason)
	}
	result, err := s.persistTestResult(ctx, item, modelID, testErr, version, &evidence)
	if result != nil {
		result.CapabilityResults = results
	}
	return result, err
}

func (s *Service) probeCapability(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity, capability string) CapabilityProbeResult {
	limit := capabilityProbeTimeout
	if capability == "image_output" || capability == "image_editing" {
		limit = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	switch capability {
	case "text_output":
		result, _ := s.probeText(ctx, item, model)
		return result
	case "vision":
		return s.checkVision(ctx, item, model)
	case "tool_calling":
		return s.checkTools(ctx, item, model)
	case "reasoning":
		return s.checkReasoning(ctx, item, model)
	case "embedding":
		return s.checkEmbedding(ctx, item, model)
	case "image_output", "image_editing":
		return s.checkImage(ctx, item, model, capability)
	}
	return probeResult("unknown", "route_unavailable")
}

func (s *Service) probeText(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity) (CapabilityProbeResult, probeResponse) {
	if item.ProviderKind != ProviderKindLLM {
		return probeResult("unknown", "route_unavailable"), probeResponse{}
	}
	payload, err := minimalPayload(item, model.ModelID)
	if err != nil {
		return probeResult("error", "invalid_configuration"), probeResponse{}
	}
	payload = applyProbeOptions(payload, model)
	status, body, err := s.sendProbeRequest(ctx, item, payload)
	if failure := probeFailure(status, body, err, "text_output"); failure != nil {
		return *failure, probeResponse{}
	}
	response := parseProbeResponse(body, item.APIFormat)
	if response.Valid && response.Text != "" {
		return probeResult("supported", "text_response"), response
	}
	return probeResult("unknown", "no_text_evidence"), response
}

func applyProbeOptions(payload []byte, model providerstore.ModelEntity) []byte {
	var fields map[string]any
	if json.Unmarshal(payload, &fields) != nil {
		return payload
	}
	// Model options cannot replace probe identity, challenge content, tools or budgets.
	options := decodeProviderOptions(model.ProviderOptionsJSON)
	for key, value := range fields {
		options[key] = value
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		return payload
	}
	return encoded
}

func (s *Service) sendProbeRequest(ctx context.Context, item providerstore.Entity, payload []byte) (int, []byte, error) {
	if err := validateModelEndpoint(item); err != nil {
		return 0, nil, err
	}
	return s.sendProbeRequestTo(ctx, item, endpointURL(item, item.APIFormat), payload)
}

func (s *Service) sendProbeRequestTo(ctx context.Context, item providerstore.Entity, endpoint string, payload []byte) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	applyProviderHeaders(request, item)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return 0, nil, err
	}
	if len(body) > 1<<20 {
		return 0, nil, fmt.Errorf("probe response too large")
	}
	return response.StatusCode, body, nil
}

// Only protocol-specific machine rejections are capability denials.
func probeFailure(status int, body []byte, err error, capability string) *CapabilityProbeResult {
	result := probeResult("error", "request_failed")
	if err != nil {
		result.Reason = "network_or_timeout"
		return &result
	}
	if probeExplicitlyUnsupported(status, body, capability) {
		result = probeResult("unsupported", "explicit_protocol_denial")
		return &result
	}
	switch status {
	case 401, 403:
		result.Reason = "authentication_or_permission"
		return &result
	case 429:
		result.Reason = "quota_or_rate_limit"
		return &result
	}
	if status < 200 || status >= 300 {
		return &result
	}
	var envelope struct {
		Error  json.RawMessage `json:"error"`
		Status string          `json:"status"`
	}
	if json.Unmarshal(body, &envelope) == nil && ((len(envelope.Error) > 0 && string(envelope.Error) != "null") || envelope.Status == "failed") {
		result.Reason = "provider_error"
		return &result
	}
	return nil
}

func probeExplicitlyUnsupported(status int, body []byte, capability string) bool {
	if status != 400 && status != 422 {
		return false
	}
	var wire struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &wire) != nil {
		return false
	}
	codes := map[string][]string{
		"vision":       {"image_input_not_supported", "vision_not_supported"},
		"tool_calling": {"tool_calling_not_supported", "tools_not_supported"},
		"reasoning":    {"reasoning_not_supported"},
		"embedding":    {"embeddings_not_supported", "embedding_not_supported"},
		"text_output":  {"text_output_not_supported"},
	}
	for _, code := range codes[capability] {
		if strings.EqualFold(wire.Error.Code, code) {
			return true
		}
	}
	return false
}

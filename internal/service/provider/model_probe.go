// INPUT: Explicit existing Provider/model test command and the exact configuration snapshot.
// OUTPUT: Connectivity result plus independently verified text, vision, tools and reasoning facts.
// POS: Bounded capability discovery during user-requested testing, never during chat startup.
package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const capabilityProbeTimeout = 12 * time.Second

func (s *Service) runModelTest(ctx context.Context, item providerstore.Entity, modelID string, expectedVersion int64) (*TestResult, error) {
	if expectedVersion != item.ConfigurationVersion {
		return nil, ErrConfigurationVersionConflict
	}
	model, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		model = &providerstore.ModelEntity{ProviderID: item.ID, ModelID: modelID, ProviderOptionsJSON: "{}"}
	}
	if err = validateModelEndpoint(item); err != nil {
		return s.persistTestResult(ctx, item, modelID, err, expectedVersion, nil)
	}
	payload, err := minimalPayload(item, modelID)
	if err != nil {
		return s.persistTestResult(ctx, item, modelID, err, expectedVersion, nil)
	}
	status, body, err := s.sendProbeRequest(ctx, item, payload)
	if err == nil && (status < 200 || status >= 300) {
		err = fmt.Errorf("模型请求失败: status=%d body=%s", status, sanitizeHTTPBody(body, item.AuthToken))
	}
	if err == nil {
		var envelope struct {
			Error  json.RawMessage `json:"error"`
			Status string          `json:"status"`
		}
		if json.Unmarshal(body, &envelope) == nil && ((len(envelope.Error) > 0 && string(envelope.Error) != "null") || envelope.Status == "failed") {
			err = fmt.Errorf("模型请求失败: %s", sanitizeHTTPBody(body, item.AuthToken))
		}
	}
	if err != nil {
		return s.persistTestResult(ctx, item, modelID, err, expectedVersion, nil)
	}
	evidence := modelProbeEvidence{Version: modelProbeVersion, Fingerprint: modelProbeFingerprint(item, *model), TestedAt: s.now()}
	if previous := currentModelProbe(item, *model); previous != nil {
		evidence.Capabilities = previous.Capabilities
	}
	if item.ProviderKind == ProviderKindLLM {
		response := parseProbeResponse(body, item.APIFormat)
		observed := observedResponseCapabilities(response)
		// An invalid/empty HTTP envelope is not capability evidence. Retain the
		// existing connectivity-test contract, without painting any capability icon.
		if response.Valid {
			observed = mergeObservedCapabilities(observed, s.probeChatCapabilities(ctx, item, modelID))
		}
		evidence.Capabilities = mergeObservedCapabilities(evidence.Capabilities, observed)
	} else if imageGenerationResponsePresent(body) {
		evidence.Capabilities.ImageOutput = adviceBool(true)
	}
	return s.persistTestResult(ctx, item, modelID, nil, expectedVersion, &evidence)
}

func (s *Service) probeChatCapabilities(ctx context.Context, item providerstore.Entity, modelID string) ModelCapabilities {
	ctx, cancel := context.WithTimeout(ctx, capabilityProbeTimeout)
	defer cancel()
	results := make(chan ModelCapabilities, 2)
	go func() { results <- s.probeVision(ctx, item, modelID) }()
	go func() { results <- s.probeTools(ctx, item, modelID) }()
	return mergeObservedCapabilities(<-results, <-results)
}

func (s *Service) probeVision(ctx context.Context, item providerstore.Entity, modelID string) ModelCapabilities {
	data, answer, err := newVisionProbeImage()
	if err != nil {
		return ModelCapabilities{}
	}
	payload, err := capabilityProbePayload(item, modelID, visionProbePrompt, data, "")
	if err != nil {
		return ModelCapabilities{}
	}
	status, body, err := s.sendProbeRequest(ctx, item, payload)
	if err != nil {
		return ModelCapabilities{}
	}
	if probeExplicitlyUnsupported(status, body, "vision") {
		return ModelCapabilities{Vision: adviceBool(false)}
	}
	if status < 200 || status >= 300 {
		return ModelCapabilities{}
	}
	response := parseProbeResponse(body, item.APIFormat)
	observed := observedResponseCapabilities(response)
	if !response.Valid || strings.Join(strings.Fields(response.Text), "") != answer {
		return observed
	}
	observed.Vision = adviceBool(true)
	return observed
}

func (s *Service) probeTools(ctx context.Context, item providerstore.Entity, modelID string) ModelCapabilities {
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ModelCapabilities{}
	}
	code := hex.EncodeToString(nonce[:])
	payload, err := capabilityProbePayload(item, modelID, "Call "+modelProbeToolName+" exactly once with code "+code+". Do not answer with text.", "", code)
	if err != nil {
		return ModelCapabilities{}
	}
	status, body, err := s.sendProbeRequest(ctx, item, payload)
	if err != nil {
		return ModelCapabilities{}
	}
	if probeExplicitlyUnsupported(status, body, "tool_calling") {
		return ModelCapabilities{ToolCalling: adviceBool(false)}
	}
	if status < 200 || status >= 300 {
		return ModelCapabilities{}
	}
	response := parseProbeResponse(body, item.APIFormat)
	observed := observedResponseCapabilities(response)
	if response.Valid && len(response.Tools) == 1 && response.Tools[0].Name == modelProbeToolName && response.Tools[0].Code == code {
		observed.ToolCalling = adviceBool(true)
	}
	return observed
}

func observedResponseCapabilities(response probeResponse) ModelCapabilities {
	result := ModelCapabilities{}
	if response.Valid && response.Text != "" {
		result.TextOutput = adviceBool(true)
	}
	if response.Valid && response.Reasoning {
		result.Reasoning = adviceBool(true)
	}
	return result
}

func (s *Service) sendProbeRequest(ctx context.Context, item providerstore.Entity, payload []byte) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL(item, item.APIFormat), bytes.NewReader(payload))
	if err != nil {
		return 0, nil, sanitizeHTTPError(err)
	}
	applyProviderHeaders(request, item)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("%s", sanitizeErrorMessage(err.Error(), item.AuthToken))
	}
	defer response.Body.Close()
	limit := int64(1 << 20)
	if item.ProviderKind == ProviderKindImageGeneration {
		limit = 16 << 20
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return 0, nil, sanitizeHTTPError(err)
	}
	if int64(len(body)) > limit {
		return 0, nil, fmt.Errorf("模型测试响应过大")
	}
	return response.StatusCode, body, nil
}

// Only exact machine error codes on a syntactically valid probe are negative
// evidence. Auth, quota, timeouts, generic 400s and wrong answers stay unknown.
func probeExplicitlyUnsupported(status int, body []byte, capability string) bool {
	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
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
	switch capability {
	case "vision":
		return wire.Error.Code == "image_input_not_supported" || wire.Error.Code == "vision_not_supported"
	case "tool_calling":
		return wire.Error.Code == "tool_calling_not_supported" || wire.Error.Code == "tools_not_supported"
	}
	return false
}

func imageGenerationResponsePresent(body []byte) bool {
	var wire struct {
		Data []struct {
			URL    string `json:"url"`
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &wire) != nil {
		return false
	}
	for _, item := range wire.Data {
		if parsed, err := url.Parse(item.URL); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" {
			return true
		}
		if decoded, err := base64.StdEncoding.DecodeString(item.Base64); err == nil && len(decoded) > 0 {
			return true
		}
	}
	return false
}

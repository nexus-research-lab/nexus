// INPUT: Exact Provider route/credential/model configuration and bounded probe observations.
// OUTPUT: Private, configuration-bound capability evidence; never a credential or raw response.
// POS: Capability probe persistence inside the existing model facts JSON and Provider CAS.
package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const modelProbeVersion = 2

type modelProbeEvidence struct {
	BaselineAutoJSON  string                           `json:"-"`
	BaselineModelID   string                           `json:"-"`
	PreserveSelection bool                             `json:"-"`
	Attempts          map[string]CapabilityProbeResult `json:"attempts,omitempty"`
	Verified          map[string]CapabilityProbeResult `json:"verified,omitempty"`
	Version           int                              `json:"version"`
	Fingerprint       string                           `json:"fingerprint"`
	TestedAt          time.Time                        `json:"tested_at"`
	Capabilities      ModelCapabilities                `json:"capabilities"`
}

// The aggregate revision guards the write; this identity keeps evidence valid
// across cosmetic changes but invalidates it when the actual route changes.
func modelProbeFingerprint(item providerstore.Entity, model providerstore.ModelEntity) string {
	return capabilityFingerprint([]any{modelProbeVersion, modelFactsFingerprint(item, model.ModelID), decodeProviderOptions(model.ProviderOptionsJSON)})
}

func modelFactsFingerprint(item providerstore.Entity, modelID string) string {
	return capabilityFingerprint([]any{item.ID, item.OwnerUserID, item.Visibility,
		item.ProviderKind, item.PresetKey, item.APIFormat, item.BaseURL,
		item.AuthToken, item.ModelsPath, normalizeModelID(modelID),
	})
}

func capabilityFingerprint(value any) string {
	payload, _ := json.Marshal(value)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func encodeScopedModelFacts(item providerstore.Entity, modelID string, capabilities ModelCapabilities) string {
	payload, _ := json.Marshal(struct {
		ModelCapabilities
		FactsVersion int    `json:"facts_version"`
		Fingerprint  string `json:"route_fingerprint"`
	}{capabilities, 1, modelFactsFingerprint(item, modelID)})
	return string(payload)
}

func scopedModelFacts(item providerstore.Entity, model providerstore.ModelEntity) ModelCapabilities {
	var metadata struct {
		Fingerprint string `json:"route_fingerprint"`
	}
	_ = json.Unmarshal([]byte(model.CapabilitiesAutoJSON), &metadata)
	if metadata.Fingerprint != "" && metadata.Fingerprint != modelFactsFingerprint(item, model.ModelID) {
		return ModelCapabilities{}
	}
	return decodeModelAutoCapabilities(model.CapabilitiesAutoJSON)
}

func currentModelProbe(item providerstore.Entity, model providerstore.ModelEntity) *modelProbeEvidence {
	var stored struct {
		Probe *modelProbeEvidence `json:"probe"`
	}
	_ = json.Unmarshal([]byte(model.CapabilitiesAutoJSON), &stored)
	if stored.Probe == nil || stored.Probe.Version != modelProbeVersion ||
		stored.Probe.Fingerprint != modelProbeFingerprint(item, model) {
		return nil
	}
	return stored.Probe
}

func withModelProbe(raw string, evidence modelProbeEvidence) string {
	var stored map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &stored) != nil || stored == nil {
		stored = map[string]json.RawMessage{}
	}
	// Do not upgrade unversioned remote guesses while adding independent evidence.
	stored["probe"], _ = json.Marshal(evidence)
	payload, _ := json.Marshal(stored)
	return string(payload)
}

func mergeObservedCapabilities(previous, observed ModelCapabilities) ModelCapabilities {
	choose := func(old, next *bool) *bool {
		if next != nil {
			return next
		}
		return old
	}
	return ModelCapabilities{
		TextOutput:   choose(previous.TextOutput, observed.TextOutput),
		Vision:       choose(previous.Vision, observed.Vision),
		ToolCalling:  choose(previous.ToolCalling, observed.ToolCalling),
		Reasoning:    choose(previous.Reasoning, observed.Reasoning),
		ImageOutput:  choose(previous.ImageOutput, observed.ImageOutput),
		ImageEditing: choose(previous.ImageEditing, observed.ImageEditing),
		Embedding:    choose(previous.Embedding, observed.Embedding),
	}
}

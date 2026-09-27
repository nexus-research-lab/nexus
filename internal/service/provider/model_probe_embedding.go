// INPUT: Explicit embedding check on an OpenAI-compatible route.
// OUTPUT: Evidence from indexed, finite, nonzero, dimension-consistent distinct vectors.
// POS: Embedding protocol probe; never inferred from chat responses.
package provider

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) checkEmbedding(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity) CapabilityProbeResult {
	if item.ProviderKind != ProviderKindLLM || (item.APIFormat != APIFormatChatCompletions && item.APIFormat != APIFormatResponses) {
		return probeResult("unknown", "route_unavailable")
	}
	nonce, err := probeNonce()
	if err != nil {
		return probeResult("error", "fixture_failed")
	}
	raw, _ := json.Marshal(map[string]any{"model": model.ModelID, "input": []string{"A red apple on a table. " + nonce, "Quantum gravity describes spacetime. " + nonce}, "encoding_format": "float"})
	status, body, err := s.sendProbeRequestTo(ctx, item, embeddingProbeEndpoint(item), raw)
	if failure := probeFailure(status, body, err, "embedding"); failure != nil {
		return *failure
	}
	if validEmbeddingProbe(body) {
		return probeResult("supported", "embedding_vectors")
	}
	return probeResult("unknown", "invalid_embedding_evidence")
}

func validEmbeddingProbe(body []byte) bool {
	var wire struct {
		Data []struct {
			Index     *int      `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &wire) != nil || len(wire.Data) != 2 {
		return false
	}
	vectors := [2][]float64{}
	for _, row := range wire.Data {
		if row.Index == nil || *row.Index < 0 || *row.Index > 1 || vectors[*row.Index] != nil || len(row.Embedding) == 0 || len(row.Embedding) > 65536 {
			return false
		}
		nonzero := false
		for _, value := range row.Embedding {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return false
			}
			nonzero = nonzero || value != 0
		}
		if !nonzero {
			return false
		}
		vectors[*row.Index] = row.Embedding
	}
	if len(vectors[0]) != len(vectors[1]) {
		return false
	}
	for i, value := range vectors[0] {
		if value != vectors[1][i] {
			return true
		}
	}
	return false
}

// embeddingProbeEndpoint 复用配置端点和 Azure 鉴权，保留 query 与 deployment identity。
func embeddingProbeEndpoint(item providerstore.Entity) string {
	base := item.BaseURL
	parsed, err := url.Parse(base)
	if err != nil {
		return joinEndpointURL(base, "/embeddings")
	}
	for _, suffix := range []string{"/chat/completions", "/responses", "/embeddings"} {
		if strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), suffix) {
			parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), suffix)
			parsed.RawPath = ""
			return joinEndpointURL(parsed.String(), "/embeddings")
		}
	}
	if IsAzureOpenAIEndpoint(base) && !strings.Contains(strings.ToLower(parsed.Path), "/deployments/") {
		resolved, _ := url.Parse(azureResponsesEndpointURL(base))
		resolved.Path = strings.TrimSuffix(resolved.Path, "/responses") + "/embeddings"
		return resolved.String()
	}
	return joinEndpointURL(base, "/embeddings")
}

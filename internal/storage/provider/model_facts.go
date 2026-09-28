// INPUT: Exact model identity and automatic facts inside a Provider CAS transaction.
// OUTPUT: Updated automatic evidence without changing user overrides or default selection.
// POS: Narrow write boundary for model capability discovery/probes.
package provider

import (
	"context"
	"strings"
)

func (m *Mutation) UpdateModelFacts(ctx context.Context, item ModelEntity) error {
	if strings.TrimSpace(item.ProviderID) != m.providerID {
		return ErrModelNotFound
	}
	result, err := m.tx.ExecContext(ctx, `UPDATE provider_models
		SET capabilities_auto_json = `+m.repository.bind(1)+`, updated_at = `+m.repository.bind(2)+`
		WHERE id = `+m.repository.bind(3)+` AND provider_id = `+m.repository.bind(4),
		item.CapabilitiesAutoJSON, item.UpdatedAt.UTC(), item.ID, m.providerID)
	return requireAffected(result, err, 1, ErrModelNotFound)
}

// INPUT: Explicit single-capability test, exact Provider scope and optional CAS version.
// OUTPUT: Existing model test result with per-capability outcomes.
// POS: Capability selection command; authentication and ownership precede network activity.
package provider

import (
	"context"
	"fmt"
)

func (s *Service) TestModelCapability(ctx context.Context, provider, model, capability string, expectedVersion *int64, public bool) (*TestResult, error) {
	if capability != "all" && !validProbeCapability(capability) {
		return nil, fmt.Errorf("%w: unknown capability", ErrInvalidInput)
	}
	lookup := s.requireProvider
	if public {
		lookup = s.requirePublicProvider
	}
	item, err := lookup(ctx, provider)
	if err != nil {
		return nil, err
	}
	if !public {
		if err = s.requireProviderManagement(ctx, *item); err != nil {
			return nil, err
		}
	}
	version := item.ConfigurationVersion
	if expectedVersion != nil {
		version = *expectedVersion
	}
	return s.runCapabilityTests(ctx, *item, model, version, capability)
}

// INPUT: Seven independent capability checks on an exact Provider/model route.
// OUTPUT: Timestamped outcomes, with inconclusive failures distinct from explicit denial.
// POS: Probe evidence contract shared by persistence, HTTP and image adapters.
package provider

import (
	"context"
	"time"
)

type CapabilityProbeResult struct {
	State    string    `json:"state"`
	Reason   string    `json:"reason,omitempty"`
	TestedAt time.Time `json:"tested_at"`
}

type ImageProbeRequest struct {
	Config ImageConfig
	Edit   bool
	Prompt string
	Image  []byte
}

type ImageProbeAdapter func(context.Context, ImageProbeRequest) ([]byte, error)

func (s *Service) SetImageProbeAdapter(adapter ImageProbeAdapter) { s.imageProbe = adapter }

var probeCapabilityKeys = []string{"text_output", "vision", "tool_calling", "reasoning", "image_output", "image_editing", "embedding"}

func validProbeCapability(capability string) bool {
	switch capability {
	case "text_output", "vision", "tool_calling", "reasoning", "image_output", "image_editing", "embedding":
		return true
	}
	return false
}

func probeCapabilities(capability string, result CapabilityProbeResult) ModelCapabilities {
	if result.State != "supported" && result.State != "unsupported" {
		return ModelCapabilities{}
	}
	value := adviceBool(result.State == "supported")
	var c ModelCapabilities
	switch capability {
	case "text_output":
		c.TextOutput = value
	case "vision":
		c.Vision = value
	case "tool_calling":
		c.ToolCalling = value
	case "reasoning":
		c.Reasoning = value
	case "image_output":
		c.ImageOutput = value
	case "image_editing":
		c.ImageEditing = value
	case "embedding":
		c.Embedding = value
	}
	return c
}

func probeResult(state, reason string) CapabilityProbeResult {
	return CapabilityProbeResult{State: state, Reason: reason}
}

// ProbeExecutionError transports a sanitized outcome from a production adapter.
type ProbeExecutionError struct{ Result CapabilityProbeResult }

func (e *ProbeExecutionError) Error() string { return e.Result.Reason }

package provider

import (
	"testing"
)

func TestModalitiesPreserveTextAndImageAndExplicitDenial(t *testing.T) {
	card := remoteModelFromCard(map[string]any{"id": "mixed", "architecture": map[string]any{"input_modalities": []any{"text", "image"}, "output_modalities": []any{"text", "image"}}})
	if card.Capabilities.TextOutput == nil || !*card.Capabilities.TextOutput || card.Capabilities.ImageOutput == nil || !*card.Capabilities.ImageOutput || card.Category != "chat" {
		t.Fatalf("lost independent modalities: %+v", card)
	}
	c := modelCapabilitiesFromCard(map[string]any{"image_output": false, "output_modalities": []any{"image"}})
	if c.ImageOutput == nil || *c.ImageOutput {
		t.Fatal("modalities overrode explicit denial")
	}
}

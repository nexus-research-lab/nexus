// INPUT: Explicit image generation/editing request through the production image adapter.
// OUTPUT: Decodable image evidence; edits additionally preserve a random input pattern.
// POS: Independent media verification, no user files or default chat-side image charges.
package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) checkImage(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity, capability string) CapabilityProbeResult {
	route, ok := imageRuntimeProvider(item)
	if !ok || !supportedImageRoute(route.APIFormat) || s.imageProbe == nil {
		return probeResult("unknown", "route_unavailable")
	}
	edit := capability == "image_editing"
	if edit && (route.APIFormat == APIFormatModelScopeImageGeneration || item.PresetKey == presetDoubao) {
		return probeResult("unknown", "route_unavailable")
	}
	request := ImageProbeRequest{Config: ImageConfig{Provider: item.Provider, APIFormat: route.APIFormat, BaseURL: route.BaseURL, AuthToken: route.AuthToken, Model: model.ModelID, ProviderOptions: decodeProviderOptions(model.ProviderOptionsJSON)}, Edit: edit,
		Prompt: "Generate a simple flat geometric illustration of three colored squares on a white background. No text."}
	answer := ""
	if edit {
		encoded, pattern, err := newVisionProbeImage()
		if err != nil {
			return probeResult("error", "fixture_failed")
		}
		request.Image, _ = base64.StdEncoding.DecodeString(encoded)
		answer = pattern
		request.Prompt = "Edit this 3 by 3 color grid. Change ONLY the center cell to magenta (#ff00ff). Preserve the other eight cell colors, their order, white gutters and grid geometry. Return the edited image."
	}
	payload, err := s.imageProbe(ctx, request)
	if err != nil {
		var failure *ProbeExecutionError
		if errors.As(err, &failure) {
			return failure.Result
		}
		return probeResult("error", "image_request_failed")
	}
	decoded, ok := decodeProbeImage(payload)
	if !ok {
		return probeResult("unknown", "invalid_image_evidence")
	}
	if edit && !verifyProbeEdit(decoded, answer) {
		return probeResult("unknown", "edit_semantics_unverified")
	}
	reason := "decoded_image"
	if edit {
		reason = "verified_image_edit"
	}
	return probeResult("supported", reason)
}

func decodeProbeImage(payload []byte) (image.Image, bool) {
	if len(payload) == 0 || len(payload) > 25<<20 {
		return nil, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 20_000_000 {
		return nil, false
	}
	decoded, _, err := image.Decode(bytes.NewReader(payload))
	return decoded, err == nil
}

// Check multiple inner-cell samples, tolerating resolution/codec changes.
// An unchanged source, unrelated generated image or correct center alone cannot pass.
func verifyProbeEdit(canvas image.Image, answer string) bool {
	if len(answer) != 9 {
		return false
	}
	bounds := canvas.Bounds()
	for cell := 0; cell < 9; cell++ {
		expected := answer[cell]
		if cell == 4 {
			expected = '5'
		}
		for _, offset := range [][2]int{{40, 40}, {60, 40}, {40, 60}, {60, 60}} {
			x := bounds.Min.X + ((cell%3)*100+offset[0])*bounds.Dx()/300
			y := bounds.Min.Y + ((cell/3)*100+offset[1])*bounds.Dy()/300
			r, g, b, a := canvas.At(x, y).RGBA()
			if a < 50000 {
				return false
			}
			color := byte('0')
			switch {
			case r > 40000 && b > 40000 && g < 26000:
				color = '5'
			case r > 40000 && g > 35000 && b < 26000:
				color = '4'
			case r > 35000 && r > g*2 && r > b*2:
				color = '1'
			case g > 25000 && g > r*2 && g > b*2:
				color = '2'
			case b > 35000 && b > r*2 && b > g*2:
				color = '3'
			}
			if color != expected {
				return false
			}
		}
	}
	return strings.TrimSpace(answer) != ""
}

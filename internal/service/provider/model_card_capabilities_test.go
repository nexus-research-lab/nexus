// INPUT: Remote capability booleans, nested modality declarations and absent fields.
// OUTPUT: Seven independent tri-state facts without guessing from model IDs.
// POS: Model discovery evidence regression.
package provider

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestModelCardCapabilityDeclarations(t *testing.T) {
	for _, tt := range []struct {
		name, card string
		want       ModelCapabilities
	}{
		{"all independent", `{"capabilities":{"text_output":true,"vision":true,"image_output":true,"image_editing":false,"tool_calling":true,"reasoning":false,"embedding":true}}`,
			ModelCapabilities{TextOutput: adviceBool(true), Vision: adviceBool(true), ImageOutput: adviceBool(true), ImageEditing: adviceBool(false), ToolCalling: adviceBool(true), Reasoning: adviceBool(false), Embedding: adviceBool(true)}},
		{"nested modalities", `{"architecture":{"input_modalities":["text","image"],"output_modalities":["text","image"]},"supported_parameters":["tools","reasoning_effort"]}`,
			ModelCapabilities{TextOutput: adviceBool(true), Vision: adviceBool(true), ImageOutput: adviceBool(true), ToolCalling: adviceBool(true), Reasoning: adviceBool(true), Embedding: adviceBool(false)}},
		{"explicit denial wins", `{"vision":false,"reasoning":false,"architecture":{"input_modalities":["image"]},"supported_parameters":["reasoning"]}`,
			ModelCapabilities{Vision: adviceBool(false), Reasoning: adviceBool(false)}},
		{"name alone is unknown", `{"id":"kimi-k2.6"}`, ModelCapabilities{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var card map[string]any
			if err := json.Unmarshal([]byte(tt.card), &card); err != nil {
				t.Fatal(err)
			}
			got := modelCapabilitiesFromCard(card)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v; want %+v", got, tt.want)
			}
		})
	}
}

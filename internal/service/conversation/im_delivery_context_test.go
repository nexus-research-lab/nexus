package conversation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

func TestIMDeliveryContextIncludesOriginAndUnknownWithoutUnsentRecords(t *testing.T) {
	inputs := IMDeliveryContextualInputs([]imdelivery.Delivery{{ID: "sent", State: "sent", Content: "任务草案", SourceTitle: "原任务"}, {ID: "unknown", State: "unknown", Content: "结果待核对"}, {ID: "not-sent", State: "not_sent", Content: "尚未发送"}})
	raw, _ := json.Marshal(inputs)
	text := string(raw)
	if !strings.Contains(text, "任务草案") || !strings.Contains(text, "原任务") || !strings.Contains(text, "unknown") || strings.Contains(text, "尚未发送") {
		t.Fatal(text)
	}
	row := protocol.Message{"role": "assistant", "metadata": map[string]any{"source": "automation_delivery", "job_id": "job", "run_id": "run", "im_delivery_id": "delivery"}}
	if isAutomationDeliveryAssistant(row) {
		t.Fatal("tracked automation would be injected twice")
	}
}

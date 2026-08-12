// INPUT: 目标会话 durable history 与即将启动的用户 round。
// OUTPUT: 自上一条真实用户输入后尚未进入 runtime transcript 的 Automation 结果上下文。
// POS: 外部投递的会话投影到下一轮模型续聊语义的唯一适配层。
package conversation

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const maxAutomationDeliveryContexts = 5

// AutomationDeliveryContextualInputs returns scheduled results that were projected
// into this Nexus conversation after its prior real user turn. currentRoundID lets
// detached transports ignore the just-persisted user marker while preparing runtime.
func AutomationDeliveryContextualInputs(
	history []protocol.Message,
	currentRoundID string,
) []runtimectx.ContextualInputBlock {
	cutoff := automationDeliveryHistoryCutoff(history, currentRoundID)
	results := make([]protocol.Message, 0, maxAutomationDeliveryContexts)
	for index := cutoff - 1; index >= 0; index-- {
		row := history[index]
		if protocol.MessageRole(row) == "user" && !boolValue(row["hidden_from_user"]) {
			break
		}
		if !isAutomationDeliveryAssistant(row) {
			continue
		}
		results = append(results, row)
		if len(results) == maxAutomationDeliveryContexts {
			break
		}
	}
	if len(results) == 0 {
		return nil
	}
	blocks := make([]runtimectx.ContextualInputBlock, 0, len(results))
	for index := len(results) - 1; index >= 0; index-- {
		if block, ok := automationDeliveryContextualInput(results[index]); ok {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func automationDeliveryHistoryCutoff(history []protocol.Message, currentRoundID string) int {
	currentRoundID = strings.TrimSpace(currentRoundID)
	if currentRoundID == "" {
		return len(history)
	}
	for index, row := range history {
		if protocol.MessageRole(row) == "user" &&
			strings.TrimSpace(stringValue(row["round_id"])) == currentRoundID {
			return index
		}
	}
	return len(history)
}

func isAutomationDeliveryAssistant(row protocol.Message) bool {
	if protocol.MessageRole(row) != "assistant" {
		return false
	}
	metadata := mapValue(row["metadata"])
	return stringValue(metadata["source"]) == "automation_delivery" &&
		stringValue(metadata["job_id"]) != "" &&
		stringValue(metadata["run_id"]) != ""
}

func automationDeliveryContextualInput(row protocol.Message) (runtimectx.ContextualInputBlock, bool) {
	metadata := mapValue(row["metadata"])
	jobID := stringValue(metadata["job_id"])
	runID := stringValue(metadata["run_id"])
	result := strings.TrimSpace(message.ExtractAssistantDisplayText(row))
	if jobID == "" || runID == "" || result == "" {
		return runtimectx.ContextualInputBlock{}, false
	}
	taskName := stringValue(metadata["task_name"])
	taskLabel := jobID
	if taskName != "" {
		taskLabel = taskName
	}
	content := fmt.Sprintf(
		"A scheduled-task result was delivered into this conversation while its execution ran in a separate runtime session. Treat it as your own prior assistant answer in this conversation and continue naturally from it. Do not repeat it unless the user asks.\nTask: %s\nJob ID: %s\nRun ID: %s\nDelivered result:\n%s",
		taskLabel,
		jobID,
		runID,
		result,
	)
	return runtimectx.NewContextualInputBlock(
		runtimectx.ContextualInputNameAutomationDelivery,
		content,
		0,
		map[string]string{"job_id": jobID, "run_id": runID},
	), true
}

func mapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case protocol.Message:
		return map[string]any(typed)
	default:
		return nil
	}
}

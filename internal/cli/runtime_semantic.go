// INPUT: nexus goal/execution CLI flags、宿主私有输入槽与 round command capability。
// OUTPUT: 带自描述精确命令顺序和输入槽写入前置的 contract、inspect、invoke 稳定 JSON envelope。
// POS: Goal/WorkGraph Skill 的唯一命令传输层；业务 identity 与授权只来自宿主 broker。
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"

	"github.com/spf13/cobra"
)

func newRuntimeSemanticCommand(domain, short string) *cobra.Command {
	command := &cobra.Command{Use: domain, Short: short}
	command.AddCommand(
		newRuntimeSemanticContractCommand(domain),
		newRuntimeSemanticInspectCommand(domain),
		newRuntimeSemanticInvokeCommand(domain),
	)
	return command
}

func newRuntimeSemanticContractCommand(domain string) *cobra.Command {
	var operation string
	command := &cobra.Command{
		Use: "contract", Short: "按需读取当前 round 可用的操作 contract",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeSemanticController()
			if err != nil {
				return err
			}
			var contract runtimecommand.Contract
			err = controller.call(commandContext(cmd), runtimecommand.Request{
				Domain: domain, Action: runtimecommand.ActionContract,
				Operation: strings.TrimSpace(operation),
			}, &contract)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"domain": domain, "action": runtimecommand.ActionContract, "contract": contract,
				"command_usage": runtimeSemanticContractCommandUsage(
					domain,
					strings.TrimSpace(operation),
					contract.Inspect,
				),
			}
			if inputPath := strings.TrimSpace(os.Getenv(protocol.NexusCommandInputPathEnvName)); inputPath != "" {
				payload["input_staging"] = runtimeCommandInputStaging(inputPath)
			}
			return emitJSON(payload)
		},
	}
	command.Flags().StringVar(&operation, "operation", "", "可选；只返回一个精确 operation contract")
	return command
}

// runtimeSemanticContractCommandUsage keeps the CLI transport self-describing
// without copying domain schemas into Skills or the operation directory.
func runtimeSemanticContractCommandUsage(domain, operation, inspectOperation string) map[string]string {
	domain = strings.TrimSpace(domain)
	operation = strings.TrimSpace(operation)
	inspectOperation = strings.TrimSpace(inspectOperation)
	usage := map[string]string{
		"inspect": fmt.Sprintf(
			`"${NEXUS_COMMAND_PATH}" --json %s inspect`,
			domain,
		),
	}
	if domain == runtimecommand.DomainExecution {
		usage["inspect_explicit"] = `"${NEXUS_COMMAND_PATH}" --json execution inspect --execution-id '<execution-id>'`
	}
	if operation == "" {
		usage["next"] = "choose one allowed action, then read its exact operation contract before invoking"
		usage["operation_contract"] = fmt.Sprintf(
			`"${NEXUS_COMMAND_PATH}" --json %s contract --operation '<operation>'`,
			domain,
		)
		return usage
	}
	if operation == inspectOperation {
		usage["next"] = "use inspect; the domain read operation is not invokable"
		return usage
	}
	usage["next"] = "read the pre-created input_staging.path once with Read, overwrite it with one complete JSON object, then invoke with one stable request id; invoke always reads the host-managed staging slot"
	usage["invoke"] = fmt.Sprintf(
		`"${NEXUS_COMMAND_PATH}" --json %s invoke --operation '%s' --request-id '<stable-request-id>'`,
		domain,
		operation,
	)
	return usage
}

func runtimeCommandInputStaging(inputPath string) map[string]any {
	return map[string]any{
		"path":               inputPath,
		"max_bytes":          maxRuntimeCommandInputBytes,
		"initial_content":    map[string]any{},
		"write_precondition": "read this pre-created file once with Read before the first Write",
	}
}

func newRuntimeSemanticInspectCommand(domain string) *cobra.Command {
	var executionID string
	stateName := "Execution / WorkGraph"
	if domain == runtimecommand.DomainGoal {
		stateName = "Goal"
	}
	command := &cobra.Command{
		Use: "inspect", Short: "读取当前 actor 的权威 " + stateName + " 状态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeSemanticController()
			if err != nil {
				return err
			}
			input := map[string]any(nil)
			if executionID = strings.TrimSpace(executionID); executionID != "" {
				input = map[string]any{"execution_id": executionID}
			}
			var result runtimecommand.Result
			if err = controller.call(commandContext(cmd), runtimecommand.Request{
				Domain: domain, Action: runtimecommand.ActionInspect,
				Input: input,
			}, &result); err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": domain, "action": runtimecommand.ActionInspect, "result": result})
		},
	}
	if domain == runtimecommand.DomainExecution {
		command.Flags().StringVar(&executionID, "execution-id", "", "可选；读取同一可信 scope 中的一个显式历史 Execution")
	}
	return command
}

func newRuntimeSemanticInvokeCommand(domain string) *cobra.Command {
	var operation string
	var requestID string
	command := &cobra.Command{
		Use: "invoke", Short: "在当前 exact round authority 下执行一个语义操作",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeSemanticController()
			if err != nil {
				return err
			}
			input, err := decodeRuntimeSemanticInputForCommand(cmd)
			if err != nil {
				return err
			}
			requestID = strings.TrimSpace(requestID)
			if !runtimecommand.ValidRequestID(requestID) {
				return usageErrorf("invoke --request-id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符；同一意图重试时复用")
			}
			var result runtimecommand.Result
			if err = controller.call(commandContext(cmd), runtimecommand.Request{
				Domain: domain, Action: runtimecommand.ActionInvoke,
				Operation: strings.TrimSpace(operation), Input: input, RequestID: requestID,
			}, &result); err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": domain, "action": runtimecommand.ActionInvoke,
				"operation": strings.TrimSpace(operation), "request_id": requestID,
				"result": result,
			})
		},
	}
	command.Flags().StringVar(&operation, "operation", "", "contract 返回的精确 operation")
	_ = command.MarkFlagRequired("operation")
	command.Flags().StringVar(&requestID, "request-id", "", "8-128 位稳定命令 ID；同一意图重试时必须复用")
	return command
}

func decodeRuntimeSemanticInputForCommand(command *cobra.Command) (map[string]any, error) {
	inputPath := strings.TrimSpace(os.Getenv(protocol.NexusCommandInputPathEnvName))
	if inputPath == "" || inputPath == "-" {
		return nil, usageErrorf("当前 physical round 缺少宿主签发的 %s", protocol.NexusCommandInputPathEnvName)
	}
	raw, err := readRuntimeCommandInput(command, inputPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "" {
		raw = []byte("{}")
	}
	var input map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&input); err != nil {
		return nil, usageErrorf("command input 必须是有效 JSON 对象: %v", err)
	}
	if input == nil {
		return nil, usageErrorf("command input 必须是 JSON 对象")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, usageErrorf("command input 必须只包含一个 JSON 对象: %v", err)
	}
	return input, nil
}

type runtimeSemanticController struct {
	endpoint string
	token    string
	client   *http.Client
}

func newRuntimeSemanticController() (*runtimeSemanticController, error) {
	endpoint := strings.TrimSpace(os.Getenv(protocol.NexusCommandBrokerURLEnvName))
	token := strings.TrimSpace(os.Getenv(protocol.NexusCommandCapabilityTokenEnvName))
	if endpoint == "" || token == "" {
		return nil, errors.New("当前进程没有宿主签发的 Nexus runtime command capability")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() == "" {
		return nil, errors.New("NEXUS_COMMAND_BROKER_URL 无效")
	}
	host := strings.TrimSpace(parsed.Hostname())
	address := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (address == nil || !address.IsLoopback()) {
		return nil, errors.New("NEXUS_COMMAND_BROKER_URL 必须是宿主注入的 loopback HTTP 地址")
	}
	return &runtimeSemanticController{
		endpoint: parsed.String(), token: token, client: &http.Client{Timeout: 2 * time.Minute},
	}, nil
}

func (c *runtimeSemanticController) call(ctx context.Context, command runtimecommand.Request, target any) error {
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(protocol.NexusCommandCapabilityHeader, c.token)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("调用 Nexus runtime command broker: %w", err)
	}
	defer response.Body.Close()
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("解析 runtime command broker 响应: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !envelope.Success {
		message := strings.TrimSpace(envelope.Error.Message)
		if message == "" {
			message = response.Status
		}
		return errors.New(message)
	}
	if target == nil {
		return nil
	}
	if len(envelope.Data) == 0 {
		return errors.New("runtime command broker 响应缺少 data")
	}
	return json.Unmarshal(envelope.Data, target)
}

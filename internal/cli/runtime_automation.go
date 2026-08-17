// INPUT: nexus automation CLI flags 与宿主注入的 broker capability。
// OUTPUT: contract、inspect、plan、apply 的稳定 JSON envelope。
// POS: Automation Skill 的命令传输层；业务授权、确认和写入只发生在宿主 service。
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

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/spf13/cobra"
)

type runtimeAutomationFlags struct {
	operation string
	input     string
}

type runtimeAutomationController interface {
	Contract(context.Context) (*automationdomain.AutomationCommandContract, error)
	Inspect(context.Context, string, automationdomain.AutomationCommandInput) (json.RawMessage, error)
	Plan(context.Context, string, automationdomain.AutomationCommandInput) (*automationdomain.AutomationCommandPlan, error)
	Replay(context.Context, automationdomain.AutomationCommandRequest) (*automationdomain.AutomationCommandReplayResult, error)
	Apply(context.Context, automationdomain.AutomationCommandRequest) (*automationdomain.AutomationCommandApplyResult, error)
}

func newRuntimeAutomationCommand() *cobra.Command {
	command := &cobra.Command{Use: "automation", Short: "管理 scheduled task 与 heartbeat"}
	command.AddCommand(
		newRuntimeAutomationContractCommand(),
		newRuntimeAutomationInspectCommand(),
		newRuntimeAutomationPlanCommand(),
		newRuntimeAutomationApplyCommand(),
	)
	return command
}

func newRuntimeAutomationContractCommand() *cobra.Command {
	return &cobra.Command{
		Use: "contract", Short: "读取当前 round 可用的 Automation 操作目录",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeAutomationController()
			if err != nil {
				return err
			}
			contract, err := controller.Contract(commandContext(cmd))
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation", "action": "contract", "contract": contract})
		},
	}
}

func newRuntimeAutomationInspectCommand() *cobra.Command {
	flags := runtimeAutomationFlags{}
	command := &cobra.Command{
		Use: "inspect", Short: "执行只读 Automation 查询",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeAutomationController()
			if err != nil {
				return err
			}
			input, err := decodeRuntimeAutomationInput(flags.input)
			if err != nil {
				return err
			}
			data, err := controller.Inspect(commandContext(cmd), flags.operation, input)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation", "action": "inspect", "operation": flags.operation,
				"data": json.RawMessage(data),
			})
		},
	}
	bindRuntimeAutomationFlags(command, &flags)
	return command
}

func newRuntimeAutomationPlanCommand() *cobra.Command {
	flags := runtimeAutomationFlags{}
	command := &cobra.Command{
		Use: "plan", Short: "预检 Automation 变更，不写入",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeAutomationController()
			if err != nil {
				return err
			}
			input, err := decodeRuntimeAutomationInput(flags.input)
			if err != nil {
				return err
			}
			plan, err := controller.Plan(commandContext(cmd), flags.operation, input)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation", "action": "plan", "plan": plan})
		},
	}
	bindRuntimeAutomationFlags(command, &flags)
	return command
}

func newRuntimeAutomationApplyCommand() *cobra.Command {
	flags := runtimeAutomationFlags{}
	var requestID string
	var expectedRevision string
	command := &cobra.Command{
		Use: "apply", Short: "重新预检、请求真人确认并应用 Automation 变更",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := newRuntimeAutomationController()
			if err != nil {
				return err
			}
			input, err := decodeRuntimeAutomationInput(flags.input)
			if err != nil {
				return err
			}
			requestID = strings.TrimSpace(requestID)
			if requestID == "" {
				return usageErrorf("apply 必须提供稳定 --request-id；同一调用重试时复用")
			}
			baseRequest := automationdomain.AutomationCommandRequest{
				Operation: flags.operation, Input: input, RequestID: requestID,
			}
			replay, err := controller.Replay(commandContext(cmd), baseRequest)
			if err != nil {
				return err
			}
			if replay.Found {
				return emitJSON(map[string]any{
					"domain": "automation", "action": "apply", "result": replay.Result,
				})
			}
			plan, err := controller.Plan(commandContext(cmd), flags.operation, input)
			if err != nil {
				return err
			}
			if expected := strings.TrimSpace(expectedRevision); expected != "" && expected != plan.CurrentRevision {
				return fmt.Errorf("Automation 状态已变化：expected_revision=%s current_revision=%s；请重新 inspect/plan", expected, plan.CurrentRevision)
			}
			result, err := controller.Apply(commandContext(cmd), automationdomain.AutomationCommandRequest{
				Action: automationdomain.AutomationCommandActionApply, Operation: flags.operation,
				Input: input, RequestID: requestID,
				ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
			})
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation", "action": "apply", "plan": plan, "result": result,
			})
		},
	}
	bindRuntimeAutomationFlags(command, &flags)
	command.Flags().StringVar(&requestID, "request-id", "", "8-128 位稳定命令 ID；重试同一意图时必须复用")
	command.Flags().StringVar(&expectedRevision, "expected-revision", "", "可选的先前 plan revision；变化时拒绝写入")
	return command
}

func bindRuntimeAutomationFlags(command *cobra.Command, flags *runtimeAutomationFlags) {
	command.Flags().StringVar(&flags.operation, "operation", "", "contract 返回的精确 operation")
	command.Flags().StringVar(&flags.input, "input", "{}", "Automation command JSON 对象")
	_ = command.MarkFlagRequired("operation")
}

func decodeRuntimeAutomationInput(raw string) (automationdomain.AutomationCommandInput, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	var input automationdomain.AutomationCommandInput
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, usageErrorf("--input 必须是有效 Automation JSON 对象: %v", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return input, usageErrorf("--input 必须只包含一个 JSON 对象: %v", err)
	}
	return input, nil
}

type remoteRuntimeAutomationController struct {
	endpoint string
	token    string
	client   *http.Client
}

func newRuntimeAutomationController() (runtimeAutomationController, error) {
	endpoint := strings.TrimSpace(os.Getenv(protocol.NexusCommandBrokerURLEnvName))
	token := strings.TrimSpace(os.Getenv(protocol.NexusCommandCapabilityTokenEnvName))
	if endpoint == "" || token == "" {
		return nil, errors.New("当前进程没有宿主签发的 Nexus runtime command capability")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || !runtimeAutomationLoopbackHost(parsed.Hostname()) {
		return nil, errors.New("NEXUS_COMMAND_BROKER_URL 必须是宿主注入的 loopback HTTP 地址")
	}
	return remoteRuntimeAutomationController{
		endpoint: endpoint, token: token, client: &http.Client{Timeout: 2 * time.Minute},
	}, nil
}

func (c remoteRuntimeAutomationController) Contract(ctx context.Context) (*automationdomain.AutomationCommandContract, error) {
	var result automationdomain.AutomationCommandContract
	err := c.do(ctx, automationdomain.AutomationCommandRequest{Action: automationdomain.AutomationCommandActionContract}, &result)
	return &result, err
}

func (c remoteRuntimeAutomationController) Inspect(
	ctx context.Context,
	operation string,
	input automationdomain.AutomationCommandInput,
) (json.RawMessage, error) {
	var result json.RawMessage
	err := c.do(ctx, automationdomain.AutomationCommandRequest{
		Action: automationdomain.AutomationCommandActionInspect, Operation: operation, Input: input,
	}, &result)
	return result, err
}

func (c remoteRuntimeAutomationController) Plan(
	ctx context.Context,
	operation string,
	input automationdomain.AutomationCommandInput,
) (*automationdomain.AutomationCommandPlan, error) {
	var result automationdomain.AutomationCommandPlan
	err := c.do(ctx, automationdomain.AutomationCommandRequest{
		Action: automationdomain.AutomationCommandActionPlan, Operation: operation, Input: input,
	}, &result)
	return &result, err
}

func (c remoteRuntimeAutomationController) Apply(
	ctx context.Context,
	request automationdomain.AutomationCommandRequest,
) (*automationdomain.AutomationCommandApplyResult, error) {
	var result automationdomain.AutomationCommandApplyResult
	err := c.do(ctx, request, &result)
	return &result, err
}

func (c remoteRuntimeAutomationController) Replay(
	ctx context.Context,
	request automationdomain.AutomationCommandRequest,
) (*automationdomain.AutomationCommandReplayResult, error) {
	request.Action = automationdomain.AutomationCommandActionReplay
	var result automationdomain.AutomationCommandReplayResult
	err := c.do(ctx, request, &result)
	return &result, err
}

func (c remoteRuntimeAutomationController) do(ctx context.Context, command automationdomain.AutomationCommandRequest, result any) error {
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
		return fmt.Errorf("调用 Nexus Automation broker: %w", err)
	}
	defer response.Body.Close()
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err = decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("解析 Nexus Automation broker 响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !envelope.Success {
		message := strings.TrimSpace(envelope.Error.Message)
		if message == "" {
			message = response.Status
		}
		return errors.New(message)
	}
	if result == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, result)
}

func runtimeAutomationLoopbackHost(host string) bool {
	address := net.ParseIP(strings.TrimSpace(host))
	return strings.EqualFold(strings.TrimSpace(host), "localhost") || (address != nil && address.IsLoopback())
}

// INPUT: nexus automation CLI flags 与宿主注入的 broker capability。
// OUTPUT: 带输入槽写入前置的 contract、inspect、plan、apply 稳定 JSON envelope。
// POS: Automation Skill 的命令传输层；业务授权、确认和写入只发生在宿主 service。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/spf13/cobra"
)

type runtimeAutomationFlags struct {
	operation string
	input     string
	inputFile string
}

const maxRuntimeCommandInputBytes = 1 << 20

type runtimeAutomationController interface {
	Contract(context.Context) (*automationdomain.AutomationCommandContract, error)
	Inspect(context.Context, string, automationdomain.AutomationCommandInput) (json.RawMessage, error)
	Plan(context.Context, string, automationdomain.AutomationCommandInput) (*automationdomain.AutomationCommandPlan, error)
	Replay(context.Context, automationdomain.AutomationCommandRequest) (*automationdomain.AutomationCommandReplayResult, error)
	Apply(context.Context, automationdomain.AutomationCommandRequest) (*automationdomain.AutomationCommandApplyResult, error)
}

func newRuntimeAutomationCommand() *cobra.Command {
	command := &cobra.Command{Use: "automation", Short: "管理 scheduled task"}
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
			contract, err := controller.Contract(cmd.Context())
			if err != nil {
				return err
			}
			payload := map[string]any{
				"domain": "automation", "action": "contract", "contract": contract,
				"command_usage": map[string]string{
					"next": "immediately before every new input write, run automation contract again and use only the input_staging.path in that fresh result; never reuse a remembered path from an earlier physical round; Read each newly returned path once before its first Write",
				},
			}
			if inputPath := strings.TrimSpace(os.Getenv(protocol.NexusCommandInputPathEnvName)); inputPath != "" {
				payload["input_staging"] = runtimeCommandInputStaging(inputPath)
			}
			return emitJSON(payload)
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
			input, err := decodeRuntimeAutomationInputForCommand(cmd, flags)
			if err != nil {
				return err
			}
			data, err := controller.Inspect(cmd.Context(), flags.operation, input)
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
			input, err := decodeRuntimeAutomationInputForCommand(cmd, flags)
			if err != nil {
				return err
			}
			plan, err := controller.Plan(cmd.Context(), flags.operation, input)
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
			input, err := decodeRuntimeAutomationInputForCommand(cmd, flags)
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
			replay, err := controller.Replay(cmd.Context(), baseRequest)
			if err != nil {
				return err
			}
			if replay.Found {
				return emitJSON(map[string]any{
					"domain": "automation", "action": "apply", "result": replay.Result,
				})
			}
			plan, err := controller.Plan(cmd.Context(), flags.operation, input)
			if err != nil {
				return err
			}
			if expected := strings.TrimSpace(expectedRevision); expected != "" && expected != plan.CurrentRevision {
				return fmt.Errorf("Automation 状态已变化：expected_revision=%s current_revision=%s；请重新 inspect/plan", expected, plan.CurrentRevision)
			}
			result, err := controller.Apply(cmd.Context(), automationdomain.AutomationCommandRequest{
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
	command.Flags().StringVar(&flags.inputFile, "input-file", "", "从宿主签发的 round 私有文件读取 JSON；- 表示 stdin")
	_ = command.MarkFlagRequired("operation")
}

func decodeRuntimeAutomationInputForCommand(
	command *cobra.Command,
	flags runtimeAutomationFlags,
) (automationdomain.AutomationCommandInput, error) {
	inlineChanged := command.Flags().Changed("input")
	fileChanged := command.Flags().Changed("input-file")
	if inlineChanged && fileChanged {
		return automationdomain.AutomationCommandInput{}, usageErrorf("--input 与 --input-file 不能同时使用")
	}
	if inlineChanged {
		return decodeRuntimeAutomationInput(flags.input)
	}
	inputPath := strings.TrimSpace(flags.inputFile)
	if !fileChanged {
		inputPath = strings.TrimSpace(os.Getenv(protocol.NexusCommandInputPathEnvName))
	}
	if inputPath == "" {
		if fileChanged {
			return automationdomain.AutomationCommandInput{}, usageErrorf("--input-file 不能为空")
		}
		return decodeRuntimeAutomationInput("{}")
	}
	raw, err := readRuntimeCommandInput(command, inputPath)
	if err != nil {
		return automationdomain.AutomationCommandInput{}, err
	}
	input, err := decodeRuntimeAutomationInput(string(raw))
	if err != nil {
		return input, usageErrorf("读取 %s: %v", inputPath, err)
	}
	return input, nil
}

func readRuntimeCommandInput(command *cobra.Command, inputPath string) ([]byte, error) {
	if inputPath == "-" {
		return readLimitedRuntimeCommandInput(command.InOrStdin(), "stdin")
	}
	managedPath := strings.TrimSpace(os.Getenv(protocol.NexusCommandInputPathEnvName))
	if managedPath != "" && !sameRuntimeCommandInputPath(managedPath, inputPath) {
		return nil, usageErrorf("--input-file 必须使用宿主签发的 %s", protocol.NexusCommandInputPathEnvName)
	}
	cleanPath := filepath.Clean(inputPath)
	if !filepath.IsAbs(cleanPath) {
		return nil, usageErrorf("--input-file 必须是绝对路径或 -")
	}
	root, err := confinedfs.Open(filepath.Dir(cleanPath))
	if err != nil {
		return nil, usageErrorf("打开 --input-file 目录失败: %v", err)
	}
	defer root.Close()
	file, err := root.OpenFileNoSymlink(filepath.Base(cleanPath), os.O_RDONLY, 0)
	if err != nil {
		return nil, usageErrorf("打开 --input-file 失败: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, usageErrorf("校验 --input-file 失败: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		if managedPath == "" {
			return nil, usageErrorf("--input-file 必须是 owner 私有文件（不能授予 group/other 权限）")
		}
		// Runtime editors may atomically replace the host-created 0600 file with
		// their default mode. OpenFileNoSymlink has already fenced symlinks,
		// hardlinks and replacement races, so normalize the verified managed
		// inode here instead of making the model run chmod.
		if err = file.Chmod(0o600); err != nil {
			return nil, usageErrorf("收紧受管 --input-file 权限失败: %v", err)
		}
		info, err = file.Stat()
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			return nil, usageErrorf("校验受管 --input-file 权限失败")
		}
	}
	return readLimitedRuntimeCommandInput(file, "--input-file")
}

func readLimitedRuntimeCommandInput(reader io.Reader, source string) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxRuntimeCommandInputBytes+1))
	if err != nil {
		return nil, usageErrorf("读取 %s 失败: %v", source, err)
	}
	if len(raw) > maxRuntimeCommandInputBytes {
		return nil, usageErrorf("%s 超过 %d 字节上限", source, maxRuntimeCommandInputBytes)
	}
	return raw, nil
}

func sameRuntimeCommandInputPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func decodeRuntimeAutomationInput(raw string) (automationdomain.AutomationCommandInput, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	var input automationdomain.AutomationCommandInput
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, usageErrorf("Automation input 必须是有效 JSON 对象: %v", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return input, usageErrorf("Automation input 必须只包含一个 JSON 对象: %v", err)
	}
	return input, nil
}

type remoteRuntimeAutomationController struct {
	command *runtimeSemanticController
}

func newRuntimeAutomationController() (runtimeAutomationController, error) {
	controller, err := newRuntimeSemanticController()
	if err != nil {
		return nil, err
	}
	return remoteRuntimeAutomationController{command: controller}, nil
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
	payload, err := json.Marshal(command.Input)
	if err != nil {
		return err
	}
	input := map[string]any{}
	if err = json.Unmarshal(payload, &input); err != nil {
		return err
	}
	return c.command.call(ctx, runtimecommand.Request{
		Domain: runtimecommand.DomainAutomation,
		Action: command.Action, Operation: command.Operation, Input: input,
		RequestID: command.RequestID, ExpectedRevision: command.ExpectedRevision,
		PlanDigest: command.PlanDigest,
	}, result)
}

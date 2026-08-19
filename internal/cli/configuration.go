package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"

	"github.com/spf13/cobra"
)

const maxConfigurationSecretsBytes = 1 << 20

type configurationChangeFlags struct {
	domain    string
	operation string
	target    string
	input     string
}

type configurationController interface {
	Inspect(context.Context, []string, bool) (*configurationsvc.Inspection, error)
	Plan(context.Context, configurationsvc.ChangeRequest) (*configurationsvc.ChangePlan, error)
	Apply(context.Context, configurationsvc.ChangeRequest, configurationsvc.CLIApplyOptions) (*configurationsvc.ApplyResult, error)
	History(context.Context, string, int) ([]configurationsvc.AuditRecord, error)
}

// NewConfiguration 创建只负责产品配置的 nexuscfg 应用。
func NewConfiguration(cfg config.Config) (*App, error) {
	services := newCLIServiceProvider(cfg)
	root, err := newScopedRoot(
		cfg,
		services,
		"nexuscfg",
		"Nexus 配置 CLI",
		"读取、预检并修改当前 owner 的 Nexus 产品配置。写操作始终在同一进程内重新预检并校验 revision。",
	)
	if err != nil {
		return nil, err
	}
	root.AddCommand(newConfigurationInspectCommand(services))
	root.AddCommand(newConfigurationPlanCommand(services))
	root.AddCommand(newConfigurationApplyCommand(services))
	root.AddCommand(newConfigurationHistoryCommand(services))
	return &App{command: root, services: services}, nil
}

func newConfigurationInspectCommand(services *cliServiceProvider) *cobra.Command {
	var domains []string
	var verify bool
	command := &cobra.Command{
		Use:   "inspect",
		Short: "读取脱敏配置、操作定义与健康检查",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := configurationCLIController(cmd, services)
			if err != nil {
				return err
			}
			inspection, err := controller.Inspect(
				cmd.Context(),
				domains,
				verify,
			)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"action":     "inspect",
				"inspection": inspection,
			})
		},
	}
	command.Flags().StringSliceVar(&domains, "domain", nil, "要读取的配置域，可重复；省略表示全部可读域")
	command.Flags().BoolVar(&verify, "verify", false, "执行不会泄漏凭据的健康检查")
	return command
}

func newConfigurationPlanCommand(services *cliServiceProvider) *cobra.Command {
	flags := configurationChangeFlags{}
	command := &cobra.Command{
		Use:   "plan",
		Short: "预检配置变更，不写入",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := configurationCLIController(cmd, services)
			if err != nil {
				return err
			}
			request, err := flags.request()
			if err != nil {
				return err
			}
			plan, err := controller.Plan(cmd.Context(), request)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"action": "plan", "plan": plan})
		},
	}
	bindConfigurationChangeFlags(command, &flags)
	return command
}

func newConfigurationApplyCommand(services *cliServiceProvider) *cobra.Command {
	flags := configurationChangeFlags{}
	var requestID string
	var expectedRevision string
	var confirm bool
	var secretsStdin bool
	command := &cobra.Command{
		Use:   "apply",
		Short: "重新预检并应用配置变更",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := configurationCLIController(cmd, services)
			if err != nil {
				return err
			}
			request, err := flags.request()
			if err != nil {
				return err
			}
			plan, err := controller.Plan(cmd.Context(), request)
			if err != nil {
				return err
			}
			if expected := strings.TrimSpace(expectedRevision); expected != "" && expected != plan.CurrentRevision {
				return fmt.Errorf(
					"配置已变化：expected_revision=%s current_revision=%s；请重新 inspect/plan 后核对",
					expected,
					plan.CurrentRevision,
				)
			}
			secretValues := map[string]string(nil)
			if secretsStdin {
				if hasHostManagedCLIScope() || runtimeConfigurationBrokerConfigured() {
					return usageErrorf("--secrets-stdin 仅供人工终端使用，Agent runtime 不可用")
				}
				if len(plan.SecretSlots) == 0 {
					return usageErrorf("当前 plan 不含 secret slot，不能使用 --secrets-stdin")
				}
				secretValues, err = readConfigurationSecrets(cmd.InOrStdin())
				if err != nil {
					return err
				}
			} else if len(plan.SecretSlots) > 0 {
				return usageErrorf("该变更需要 secret slot；请在人工终端使用 --secrets-stdin，Agent 不得代填秘密")
			}
			request.RequestID = strings.TrimSpace(requestID)
			if request.RequestID == "" {
				request.RequestID, err = newConfigurationRequestID()
				if err != nil {
					return err
				}
			}
			request.ExpectedRevision = plan.CurrentRevision
			request.PlanDigest = plan.PlanDigest
			result, err := controller.Apply(
				cmd.Context(),
				request,
				configurationsvc.CLIApplyOptions{
					Confirmed:    confirm,
					SecretValues: secretValues,
				},
			)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"action": "apply",
				"plan":   plan,
				"result": result,
			})
		},
	}
	bindConfigurationChangeFlags(command, &flags)
	command.Flags().StringVar(&requestID, "request-id", "", "8-128 位审计请求 ID；每次新命令应使用新 ID，省略时自动生成")
	command.Flags().StringVar(&expectedRevision, "expected-revision", "", "可选的先前 inspect/plan revision；变化时拒绝写入")
	command.Flags().BoolVar(&confirm, "confirm", false, "确认执行 plan 标记为需确认的变更")
	command.Flags().BoolVar(&secretsStdin, "secrets-stdin", false, "从 stdin 读取 secret slot 到值的 JSON 对象，仅供人工终端使用")
	return command
}

func newConfigurationHistoryCommand(services *cliServiceProvider) *cobra.Command {
	var domain string
	var limit int
	command := &cobra.Command{
		Use:   "history",
		Short: "读取脱敏配置变更审计",
		RunE: func(cmd *cobra.Command, _ []string) error {
			controller, err := configurationCLIController(cmd, services)
			if err != nil {
				return err
			}
			items, err := controller.History(
				cmd.Context(),
				domain,
				limit,
			)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"action": "history", "items": items})
		},
	}
	command.Flags().StringVar(&domain, "domain", "", "按配置域筛选")
	command.Flags().IntVar(&limit, "limit", 20, "返回条数，最大 100")
	return command
}

func bindConfigurationChangeFlags(command *cobra.Command, flags *configurationChangeFlags) {
	command.Flags().StringVar(&flags.domain, "domain", "", "配置域")
	command.Flags().StringVar(&flags.operation, "operation", "", "inspect 返回的精确操作名")
	command.Flags().StringVar(&flags.target, "target", "", "可选目标 ID")
	command.Flags().StringVar(&flags.input, "input", "{}", "不含明文秘密的 JSON 对象")
	_ = command.MarkFlagRequired("domain")
	_ = command.MarkFlagRequired("operation")
}

func (f configurationChangeFlags) request() (configurationsvc.ChangeRequest, error) {
	input, err := configurationJSONObject(f.input)
	if err != nil {
		return configurationsvc.ChangeRequest{}, err
	}
	return configurationsvc.ChangeRequest{
		Domain:    strings.TrimSpace(f.domain),
		Operation: strings.TrimSpace(f.operation),
		Target:    strings.TrimSpace(f.target),
		Input:     input,
	}, nil
}

func configurationJSONObject(raw string) (json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var value map[string]json.RawMessage
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return nil, usageErrorf("--input 必须是有效 JSON 对象: %v", err)
	}
	if value == nil {
		return nil, usageErrorf("--input 必须是 JSON 对象")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, usageErrorf("--input 必须只包含一个 JSON 对象: %v", err)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func readConfigurationSecrets(reader io.Reader) (map[string]string, error) {
	if reader == nil {
		return nil, usageErrorf("--secrets-stdin 缺少 stdin")
	}
	decoder := json.NewDecoder(io.LimitReader(reader, maxConfigurationSecretsBytes+1))
	values := map[string]string{}
	if err := decoder.Decode(&values); err != nil {
		return nil, usageErrorf("stdin 必须是 secret slot 到值的 JSON 对象: %v", err)
	}
	if len(values) == 0 {
		return nil, usageErrorf("stdin secret JSON 不能为空")
	}
	if err := requireJSONEOF(decoder); err != nil {
		clear(values)
		return nil, usageErrorf("stdin 必须只包含一个 secret JSON 对象: %v", err)
	}
	for slot := range values {
		if strings.TrimSpace(slot) == "" {
			clear(values)
			return nil, usageErrorf("secret slot ID 不能为空")
		}
	}
	return values, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("存在额外 JSON 值")
		}
		return err
	}
	return nil
}

func configurationCLIController(
	cmd *cobra.Command,
	services *cliServiceProvider,
) (configurationController, error) {
	remote, configured, err := newRuntimeConfigurationController()
	if err != nil {
		return nil, err
	}
	if configured {
		return remote, nil
	}
	appServices, err := services.AppServices()
	if err != nil {
		return nil, err
	}
	ownerUserID := strings.TrimSpace(currentCLIUserID(cmd))
	mainAgent, err := appServices.Core.Agent.GetDefaultAgent(cmd.Context())
	if err != nil {
		return nil, fmt.Errorf("读取当前 owner 主智能体: %w", err)
	}
	if mainAgent == nil || !mainAgent.IsMain || strings.TrimSpace(mainAgent.OwnerUserID) != ownerUserID {
		return nil, errors.New("nexuscfg 未解析到当前 owner 的主智能体")
	}
	return localConfigurationController{
		service: appServices.Configuration,
		actor: configurationsvc.Actor{
			OwnerUserID:     ownerUserID,
			AgentID:         strings.TrimSpace(mainAgent.AgentID),
			IsMainAgent:     true,
			ContextKind:     configurationsvc.ContextKindAgent,
			ContextID:       strings.TrimSpace(mainAgent.AgentID),
			SourceContext:   "nexuscfg",
			PrincipalRole:   authctx.RoleOwner,
			AuthMethod:      "nexuscfg",
			LocalSingleUser: ownerUserID == authctx.SystemUserID,
		},
	}, nil
}

type localConfigurationController struct {
	service *configurationsvc.Service
	actor   configurationsvc.Actor
}

func (c localConfigurationController) Inspect(
	ctx context.Context,
	domains []string,
	verify bool,
) (*configurationsvc.Inspection, error) {
	return c.service.Inspect(ctx, c.actor, domains, verify)
}

func (c localConfigurationController) Plan(
	ctx context.Context,
	request configurationsvc.ChangeRequest,
) (*configurationsvc.ChangePlan, error) {
	return c.service.PlanChange(ctx, c.actor, request)
}

func (c localConfigurationController) Apply(
	ctx context.Context,
	request configurationsvc.ChangeRequest,
	options configurationsvc.CLIApplyOptions,
) (*configurationsvc.ApplyResult, error) {
	return c.service.ApplyChangeFromCLI(ctx, c.actor, request, options)
}

func (c localConfigurationController) History(
	ctx context.Context,
	domain string,
	limit int,
) ([]configurationsvc.AuditRecord, error) {
	return c.service.ListChanges(ctx, c.actor, domain, limit)
}

type runtimeConfigurationController struct {
	endpoint string
	token    string
	client   *http.Client
}

type runtimeConfigurationCommand struct {
	Action    string                         `json:"action"`
	Domains   []string                       `json:"domains,omitempty"`
	Verify    bool                           `json:"verify,omitempty"`
	Change    configurationsvc.ChangeRequest `json:"change,omitempty"`
	Confirmed bool                           `json:"confirmed,omitempty"`
	Domain    string                         `json:"domain,omitempty"`
	Limit     int                            `json:"limit,omitempty"`
}

type runtimeConfigurationResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
}

func newRuntimeConfigurationController() (*runtimeConfigurationController, bool, error) {
	endpoint := strings.TrimSpace(os.Getenv(protocol.NexusConfigBrokerURLEnvName))
	token := strings.TrimSpace(os.Getenv(protocol.NexusConfigCapabilityTokenEnvName))
	if endpoint == "" && token == "" {
		return nil, false, nil
	}
	if endpoint == "" || token == "" {
		return nil, false, errors.New("nexuscfg runtime capability 环境不完整")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() == "" {
		return nil, false, errors.New("nexuscfg broker 地址无效")
	}
	host := strings.TrimSpace(parsed.Hostname())
	address := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (address == nil || !address.IsLoopback()) {
		return nil, false, errors.New("nexuscfg broker 只能使用本机地址")
	}
	return &runtimeConfigurationController{
		endpoint: parsed.String(),
		token:    token,
		client:   &http.Client{Timeout: 2 * time.Minute},
	}, true, nil
}

func runtimeConfigurationBrokerConfigured() bool {
	return strings.TrimSpace(os.Getenv(protocol.NexusConfigBrokerURLEnvName)) != "" ||
		strings.TrimSpace(os.Getenv(protocol.NexusConfigCapabilityTokenEnvName)) != ""
}

func (c *runtimeConfigurationController) Inspect(
	ctx context.Context,
	domains []string,
	verify bool,
) (*configurationsvc.Inspection, error) {
	var result configurationsvc.Inspection
	err := c.call(ctx, runtimeConfigurationCommand{
		Action: "inspect", Domains: domains, Verify: verify,
	}, &result)
	return &result, err
}

func (c *runtimeConfigurationController) Plan(
	ctx context.Context,
	request configurationsvc.ChangeRequest,
) (*configurationsvc.ChangePlan, error) {
	var result configurationsvc.ChangePlan
	err := c.call(ctx, runtimeConfigurationCommand{Action: "plan", Change: request}, &result)
	return &result, err
}

func (c *runtimeConfigurationController) Apply(
	ctx context.Context,
	request configurationsvc.ChangeRequest,
	options configurationsvc.CLIApplyOptions,
) (*configurationsvc.ApplyResult, error) {
	if len(options.SecretValues) > 0 {
		return nil, errors.New("Agent runtime 不能通过 nexuscfg broker 提交秘密")
	}
	var result configurationsvc.ApplyResult
	err := c.call(ctx, runtimeConfigurationCommand{
		Action: "apply", Change: request, Confirmed: options.Confirmed,
	}, &result)
	return &result, err
}

func (c *runtimeConfigurationController) History(
	ctx context.Context,
	domain string,
	limit int,
) ([]configurationsvc.AuditRecord, error) {
	var result []configurationsvc.AuditRecord
	err := c.call(ctx, runtimeConfigurationCommand{
		Action: "history", Domain: domain, Limit: limit,
	}, &result)
	return result, err
}

func (c *runtimeConfigurationController) call(
	ctx context.Context,
	command runtimeConfigurationCommand,
	target any,
) error {
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(protocol.NexusConfigCapabilityHeader, c.token)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("调用 nexuscfg broker: %w", err)
	}
	defer response.Body.Close()
	var envelope runtimeConfigurationResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("解析 nexuscfg broker 响应: %w", err)
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
		return errors.New("nexuscfg broker 响应缺少 data")
	}
	return json.Unmarshal(envelope.Data, target)
}

func newConfigurationRequestID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("生成 request_id: %w", err)
	}
	return "cfg-" + hex.EncodeToString(random), nil
}

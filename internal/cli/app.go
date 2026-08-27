package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	nexusctlUserIDEnvName         = "NEXUSCTL_USER_ID"
	nexusRuntimeScopeModeEnvName  = "NEXUS_RUNTIME_SCOPE_MODE"
	runtimeScopeModeSingleUser    = "single_user"
	runtimeScopeModeUserScoped    = "user_scoped"
	hostManagedScopeOverrideError = "当前运行时已由宿主注入 owner 作用域，不能再显式选择 CLI 作用域"
)

// App 持有单次 CLI 执行及其延迟创建的服务资源。
type App struct {
	command  *cobra.Command
	services *cliServiceProvider
	executed bool
}

func (a *App) SetArgs(args []string) {
	a.command.SetArgs(args)
}

func (a *App) SetIn(reader io.Reader) {
	a.command.SetIn(reader)
}

func (a *App) PersistentFlags() *pflag.FlagSet {
	return a.command.PersistentFlags()
}

// Execute 无论命令成功或失败都释放本次执行拥有的服务资源。
func (a *App) Execute() (err error) {
	if a == nil || a.command == nil {
		return errors.New("CLI 应用未初始化")
	}
	if a.executed {
		return errors.New("CLI 应用只能执行一次")
	}
	a.executed = true
	if a.services != nil {
		defer func() {
			err = errors.Join(err, a.services.Close(context.Background()))
		}()
	}
	return a.command.Execute()
}

// New 创建 CLI 应用。
func New(cfg config.Config) (*App, error) {
	services := newCLIServiceProvider(cfg)
	root, err := newScopedRoot(
		cfg,
		services,
		"nexusctl",
		"Nexus 控制面 CLI",
		"面向 Agent 与脚本的 Nexus 控制面 CLI。stdout 只输出数据，stderr 只输出诊断；参数错误返回 64，执行错误返回 1。",
	)
	if err != nil {
		return nil, err
	}

	root.AddCommand(newAgentCommand(services))
	root.AddCommand(newAuthCommand(services))
	root.AddCommand(newUserCommand(services))
	root.AddCommand(newRoomCommand(services))
	root.AddCommand(newConversationCommand(services))
	root.AddCommand(newSessionCommand(services))
	root.AddCommand(newWorkspaceCommand(services))
	root.AddCommand(newSkillCommand(services))
	root.AddCommand(newConnectorCommand(services))
	root.AddCommand(newLauncherCommand(services))
	root.AddCommand(newChannelCommand(services))
	root.AddCommand(newAutomationCommand(services))
	root.AddCommand(newImagegenCommand(services))
	root.AddCommand(newEmotionCommand())

	return &App{command: root, services: services}, nil
}

func newScopedRoot(
	cfg config.Config,
	services *cliServiceProvider,
	use string,
	short string,
	long string,
) (*cobra.Command, error) {
	root := &cobra.Command{
		Use:           use,
		Short:         short,
		Long:          long,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var (
		scopeUserID string
		globalScope bool
	)
	runtimeConfigurationScope := use == "nexuscfg" && runtimeConfigurationBrokerConfigured()
	hostManagedScope := hasHostManagedCLIScope() || runtimeConfigurationScope
	outputOptions := configureRootOutput(root)
	root.PersistentFlags().StringVar(
		&scopeUserID,
		"scope-user-id",
		strings.TrimSpace(os.Getenv(nexusctlUserIDEnvName)),
		"仅在宿主未注入 owner 时显式指定当前命令所属的 user_id",
	)
	root.PersistentFlags().BoolVar(
		&globalScope,
		"global-scope",
		false,
		"仅在宿主未注入 owner 的本机管理员场景下使用全局作用域",
	)
	if hostManagedScope {
		for _, name := range []string{"scope-user-id", "global-scope"} {
			if err := root.PersistentFlags().MarkHidden(name); err != nil {
				return nil, err
			}
		}
	}
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := applyOutputOptions(cfg, services, *outputOptions); err != nil {
			return err
		}
		if hostManagedScope &&
			(root.PersistentFlags().Changed("scope-user-id") ||
				root.PersistentFlags().Changed("global-scope")) {
			return usageErrorf(hostManagedScopeOverrideError)
		}
		if runtimeConfigurationScope {
			return nil
		}
		var authService *authsvc.Service
		needsScopeState := commandRequiresUserScope(cmd) &&
			strings.TrimSpace(scopeUserID) == "" &&
			!globalScope
		if needsScopeState {
			var err error
			authService, err = services.AuthService()
			if err != nil {
				return err
			}
		}
		nextCtx, err := buildScopedCLIContext(cmd.Context(), authService, cmd, scopeUserID, globalScope)
		if err != nil {
			return err
		}
		cmd.SetContext(nextCtx)
		return nil
	}

	return root, nil
}

func hasHostManagedCLIScope() bool {
	if strings.TrimSpace(os.Getenv(nexusctlUserIDEnvName)) == "" {
		return false
	}
	switch strings.TrimSpace(os.Getenv(nexusRuntimeScopeModeEnvName)) {
	case runtimeScopeModeSingleUser, runtimeScopeModeUserScoped:
		return true
	default:
		return false
	}
}

func currentCLIUserID(cmd *cobra.Command) string {
	return authsvc.OwnerUserID(cmd.Context())
}

func buildScopedCLIContext(
	base context.Context,
	authService *authsvc.Service,
	cmd *cobra.Command,
	scopeUserID string,
	globalScope bool,
) (context.Context, error) {
	trimmedUserID := strings.TrimSpace(scopeUserID)
	if existingUserID, ok := authsvc.CurrentUserID(base); ok && strings.TrimSpace(existingUserID) != "" {
		if trimmedUserID != "" && trimmedUserID != strings.TrimSpace(existingUserID) {
			return nil, usageErrorf("命令上下文中的 user_id 与 --scope-user-id 不一致")
		}
		if globalScope {
			return nil, usageErrorf("命令上下文中已存在 user_id，不能再显式指定 --global-scope")
		}
		return base, nil
	}
	if globalScope {
		if trimmedUserID != "" {
			return nil, usageErrorf("--scope-user-id 与 --global-scope 不能同时使用")
		}
		return base, nil
	}
	if !commandRequiresUserScope(cmd) {
		return base, nil
	}
	if hasHostManagedCLIScope() &&
		strings.TrimSpace(os.Getenv(nexusRuntimeScopeModeEnvName)) == runtimeScopeModeSingleUser &&
		trimmedUserID == authctx.SystemUserID {
		base = authsvc.WithPrincipal(base, &authsvc.Principal{
			UserID:     authctx.SystemUserID,
			Username:   authctx.SystemUserID,
			Role:       authctx.RoleOwner,
			AuthMethod: authctx.AuthMethodLocal,
		})
		return authsvc.WithState(base, authsvc.State{AuthRequired: false}), nil
	}
	if trimmedUserID == "" {
		if authService != nil {
			state, err := authService.GetState(context.Background())
			if err != nil {
				return nil, err
			}
			if state.UserCount > 0 {
				return nil, usageErrorf(
					"当前 CLI 运行在多用户模式下，%s 必须显式提供 --scope-user-id，或在本机管理员场景下显式加 --global-scope",
					cmd.CommandPath(),
				)
			}
			return authsvc.WithState(base, state), nil
		}
		return base, nil
	}
	return authsvc.WithPrincipal(base, &authsvc.Principal{
		UserID:     trimmedUserID,
		Username:   trimmedUserID,
		AuthMethod: "nexusctl_scope",
	}), nil
}

func commandRequiresUserScope(cmd *cobra.Command) bool {
	switch commandDomain(cmd) {
	case "", "auth", "user", "emotion", "channel", "imagegen", "completion", "help":
		return false
	default:
		return true
	}
}

func commandDomain(cmd *cobra.Command) string {
	current := cmd
	for current != nil {
		parent := current.Parent()
		if parent == nil {
			return ""
		}
		if parent.Parent() == nil {
			return strings.Fields(current.Use)[0]
		}
		current = parent
	}
	return ""
}

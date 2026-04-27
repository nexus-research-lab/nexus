package cli

import (
	"context"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	"github.com/nexus-research-lab/nexus/internal/config"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"

	"github.com/spf13/cobra"
)

const nexusctlUserIDEnvName = "NEXUSCTL_USER_ID"

// New 创建 CLI 应用。
func New(cfg config.Config) (*cobra.Command, error) {
	appServices, err := bootstrap.NewAppServices(cfg, nil)
	if err != nil {
		return nil, err
	}
	agentService := appServices.Core.Agent
	roomService := appServices.Core.Room
	sessionService := appServices.Core.Session
	authService := appServices.Auth
	workspaceService := appServices.Workspace
	skillService := appServices.Skills
	connectorService := appServices.Connectors
	launcherService := appServices.Launcher
	ingressService := appServices.Ingress
	automationService := appServices.Automation

	root := &cobra.Command{
		Use:           "nexusctl",
		Short:         "Nexus 主智能体操作系统 CLI",
		Long:          "面向 Agent 与脚本的 Nexus 控制面 CLI。stdout 只输出数据，stderr 只输出诊断；参数错误返回 64，执行错误返回 1。",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var (
		scopeUserID string
		globalScope bool
	)
	outputOptions := configureRootOutput(root)
	root.PersistentFlags().StringVar(
		&scopeUserID,
		"scope-user-id",
		strings.TrimSpace(os.Getenv(nexusctlUserIDEnvName)),
		"显式指定当前命令所属的 user_id",
	)
	root.PersistentFlags().BoolVar(
		&globalScope,
		"global-scope",
		false,
		"显式允许在本机管理员场景下使用全局作用域",
	)
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := applyOutputOptions(cfg, appServices, *outputOptions); err != nil {
			return err
		}
		nextCtx, err := buildScopedCLIContext(commandContext(cmd), authService, cmd, scopeUserID, globalScope)
		if err != nil {
			return err
		}
		cmd.SetContext(nextCtx)
		return nil
	}

	root.AddCommand(newAgentCommand(agentService))
	root.AddCommand(newAuthCommand(authService))
	root.AddCommand(newUserCommand(authService))
	root.AddCommand(newRoomCommand(roomService))
	root.AddCommand(newConversationCommand(roomService, sessionService))
	root.AddCommand(newSessionCommand(sessionService))
	root.AddCommand(newWorkspaceCommand(workspaceService))
	root.AddCommand(newSkillCommand(skillService))
	root.AddCommand(newConnectorCommand(connectorService))
	root.AddCommand(newLauncherCommand(launcherService))
	root.AddCommand(newChannelCommand(ingressService))
	root.AddCommand(newAutomationCommand(automationService))
	root.AddCommand(newMemoryCommand())

	return root, nil
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd == nil || cmd.Context() == nil {
		return context.Background()
	}
	return cmd.Context()
}

func currentCLIUserID(cmd *cobra.Command) string {
	if userID, ok := authsvc.CurrentUserID(commandContext(cmd)); ok {
		return userID
	}
	return authsvc.SystemUserID
}

func buildScopedCLIContext(
	base context.Context,
	authService *authsvc.Service,
	cmd *cobra.Command,
	scopeUserID string,
	globalScope bool,
) (context.Context, error) {
	if base == nil {
		base = context.Background()
	}
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
	if trimmedUserID == "" || authService == nil || !commandRequiresUserScope(cmd) {
		if trimmedUserID == "" && authService != nil && commandRequiresUserScope(cmd) {
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
	case "", "auth", "user", "memory", "channel", "completion", "help":
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

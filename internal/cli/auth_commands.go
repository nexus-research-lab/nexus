package cli

import (
	"fmt"
	"io"
	"strings"

	auth2 "github.com/nexus-research-lab/nexus/internal/auth"

	"github.com/spf13/cobra"
)

func newAuthCommand(service *auth2.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "auth",
		Short: "auth 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "读取认证系统状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := service.GetState(commandContext(cmd))
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "auth",
				"action": "status",
				"item":   state,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var (
			username      string
			displayName   string
			password      string
			passwordStdin bool
		)
		initOwner := &cobra.Command{
			Use:   "init-owner",
			Short: "初始化首个 owner 用户",
			RunE: func(cmd *cobra.Command, args []string) error {
				resolvedPassword, err := resolvePasswordInput(cmd, password, passwordStdin)
				if err != nil {
					return err
				}
				item, err := service.InitOwner(commandContext(cmd), auth2.InitOwnerInput{
					Username:    username,
					DisplayName: displayName,
					Password:    resolvedPassword,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "auth",
					"action": "init_owner",
					"item":   item,
				})
			},
		}
		initOwner.Flags().StringVar(&username, "username", "admin", "owner username")
		initOwner.Flags().StringVar(&displayName, "display-name", "", "owner display name")
		initOwner.Flags().StringVar(&password, "password", "", "owner password")
		initOwner.Flags().BoolVar(&passwordStdin, "password-stdin", false, "从标准输入读取 owner password")
		return initOwner
	}())

	return command
}

func newUserCommand(service *auth2.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "user",
		Short: "user 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部认证用户",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.ListUsers(commandContext(cmd))
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "user",
				"action": "list",
				"items":  items,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var (
			username      string
			displayName   string
			password      string
			role          string
			passwordStdin bool
		)
		createUser := &cobra.Command{
			Use:   "create",
			Short: "创建认证用户",
			RunE: func(cmd *cobra.Command, args []string) error {
				resolvedPassword, err := resolvePasswordInput(cmd, password, passwordStdin)
				if err != nil {
					return err
				}
				item, err := service.CreateUser(commandContext(cmd), auth2.CreateUserInput{
					Username:    username,
					DisplayName: displayName,
					Password:    resolvedPassword,
					Role:        role,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "user",
					"action": "create",
					"item":   item,
				})
			},
		}
		createUser.Flags().StringVar(&username, "username", "", "target username")
		createUser.Flags().StringVar(&displayName, "display-name", "", "target display name")
		createUser.Flags().StringVar(&password, "password", "", "initial password")
		createUser.Flags().BoolVar(&passwordStdin, "password-stdin", false, "从标准输入读取 initial password")
		createUser.Flags().StringVar(&role, "role", auth2.RoleMember, "user role")
		_ = createUser.MarkFlagRequired("username")
		return createUser
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			userID        string
			username      string
			password      string
			passwordStdin bool
		)
		resetPassword := &cobra.Command{
			Use:   "reset-password",
			Short: "重置用户密码",
			RunE: func(cmd *cobra.Command, args []string) error {
				resolvedPassword, err := resolvePasswordInput(cmd, password, passwordStdin)
				if err != nil {
					return err
				}
				item, err := service.ResetPassword(commandContext(cmd), auth2.ResetPasswordInput{
					UserID:   userID,
					Username: username,
					Password: resolvedPassword,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "user",
					"action": "reset_password",
					"item":   item,
				})
			},
		}
		resetPassword.Flags().StringVar(&userID, "user-id", "", "target user id")
		resetPassword.Flags().StringVar(&username, "username", "", "target username")
		resetPassword.Flags().StringVar(&password, "password", "", "new password")
		resetPassword.Flags().BoolVar(&passwordStdin, "password-stdin", false, "从标准输入读取 new password")
		return resetPassword
	}())

	return command
}

func resolvePasswordInput(cmd *cobra.Command, password string, passwordStdin bool) (string, error) {
	if passwordStdin {
		if strings.TrimSpace(password) != "" {
			return "", fmt.Errorf("--password 与 --password-stdin 不能同时使用")
		}
		content, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", err
		}
		resolved := strings.TrimRight(string(content), "\r\n")
		if strings.TrimSpace(resolved) == "" {
			return "", fmt.Errorf("stdin 密码不能为空")
		}
		return resolved, nil
	}
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("必须通过 --password 或 --password-stdin 提供密码")
	}
	return password, nil
}

// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：auth_commands.go
// @Date   ：2026/04/12 01:18:00
// @Author ：leemysw
// 2026/04/12 01:18:00   Create
// =====================================================

package cli

import (
	"context"

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
			state, err := service.GetState(context.Background())
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
			username    string
			displayName string
			password    string
		)
		initOwner := &cobra.Command{
			Use:   "init-owner",
			Short: "初始化首个 owner 用户",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.InitOwner(context.Background(), auth2.InitOwnerInput{
					Username:    username,
					DisplayName: displayName,
					Password:    password,
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
		_ = initOwner.MarkFlagRequired("password")
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
			items, err := service.ListUsers(context.Background())
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
			username    string
			displayName string
			password    string
			role        string
		)
		createUser := &cobra.Command{
			Use:   "create",
			Short: "创建认证用户",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.CreateUser(context.Background(), auth2.CreateUserInput{
					Username:    username,
					DisplayName: displayName,
					Password:    password,
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
		createUser.Flags().StringVar(&role, "role", auth2.RoleMember, "user role")
		_ = createUser.MarkFlagRequired("username")
		_ = createUser.MarkFlagRequired("password")
		return createUser
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			userID   string
			username string
			password string
		)
		resetPassword := &cobra.Command{
			Use:   "reset-password",
			Short: "重置用户密码",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.ResetPassword(context.Background(), auth2.ResetPasswordInput{
					UserID:   userID,
					Username: username,
					Password: password,
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
		_ = resetPassword.MarkFlagRequired("password")
		return resetPassword
	}())

	return command
}

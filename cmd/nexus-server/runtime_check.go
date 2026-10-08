// INPUT: 显式 nxs 路径；不加载 server 环境、数据库或迁移。
// OUTPUT: 安装包实际 sidecar 的兼容性 JSON，失败返回非零退出码。
// POS: 打包诊断子命令的薄装配入口，不属于用户聊天流程。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app/runtimecheck"
	"github.com/spf13/cobra"
)

func buildRuntimeCheckCommand() *cobra.Command {
	var binary string
	command := &cobra.Command{
		Use: "check-desktop-runtime", Short: "检查随包 nxs 与当前 App 的能力兼容性（不调用模型）",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			report, err := runtimecheck.Check(ctx, binary)
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err)
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
		},
	}
	command.Flags().StringVar(&binary, "nxs", "", "安装包内 nxs 的绝对路径")
	_ = command.MarkFlagRequired("nxs")
	return command
}

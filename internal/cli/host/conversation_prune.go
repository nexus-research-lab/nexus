// INPUT: nexusctl conversation prune-empty flags 与 owner-scoped Room service。
// OUTPUT: 默认 dry-run、显式 --apply 的 JSON 清理报告。
// POS: 历史重复空白 conversation 维护命令的薄 CLI 适配层。
package host

import (
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/spf13/cobra"
)

func newConversationPruneEmptyCommand(services *cliServiceProvider) *cobra.Command {
	var (
		roomID string
		apply  bool
	)
	command := &cobra.Command{
		Use:   "prune-empty",
		Short: "每个 Room 只保留一个未产生用户输入的空白会话",
		Long: "扫描 canonical Room/DM 历史与文件证据，默认只输出计划。" +
			" 只有显式传入 --apply 才会删除；执行时应先停止正在使用同一数据目录的 Nexus 服务。",
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			report, err := appServices.Core.Room.PruneEmptyConversations(
				cmd.Context(),
				room.PruneEmptyConversationsOptions{
					RoomID: roomID,
					Apply:  apply,
				},
			)
			if err != nil {
				return err
			}
			return emitEmptyConversationPruneReport(report)
		},
	}
	command.Flags().StringVar(&roomID, "room-id", "", "只扫描指定 room id；省略时扫描当前 user scope 的全部 Room")
	command.Flags().BoolVar(&apply, "apply", false, "执行删除；省略时只输出 dry-run 计划")
	return command
}

func emitEmptyConversationPruneReport(report room.EmptyConversationPruneReport) error {
	failed := report.Applied && (report.DeleteFailed > 0 || report.DraftRepairFailed > 0)
	if err := emitJSON(map[string]any{
		"success": !failed,
		"domain":  "conversation",
		"action":  "prune-empty",
		"report":  report,
	}); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf(
			"空白会话清理未完全完成：%d 个删除失败，%d 个 draft 修复失败",
			report.DeleteFailed,
			report.DraftRepairFailed,
		)
	}
	return nil
}

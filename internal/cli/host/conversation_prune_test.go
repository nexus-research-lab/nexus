package host

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/spf13/cobra"
)

func TestEmitEmptyConversationPruneReportReturnsExecutionFailureAfterOutput(t *testing.T) {
	previousOptions := currentOutputOptions
	currentOutputOptions = outputOptions{json: true}
	t.Cleanup(func() { currentOutputOptions = previousOptions })

	command := &cobra.Command{
		Use:           "test-prune-report",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return emitEmptyConversationPruneReport(room.EmptyConversationPruneReport{
				Applied:           true,
				DeleteFailed:      1,
				DraftRepairFailed: 2,
			})
		},
	}
	stdout, _, executeErr := captureCLIStreams(t, command)
	if executeErr == nil || ExitCode(executeErr) != exitCodeExecution {
		t.Fatalf("apply 部分失败必须返回 execution failure: %v", executeErr)
	}
	if !strings.Contains(executeErr.Error(), "1 个删除失败") ||
		!strings.Contains(executeErr.Error(), "2 个 draft 修复失败") {
		t.Fatalf("错误应包含失败计数: %v", executeErr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("失败前仍应输出 JSON report: %v stdout=%s", err, stdout)
	}
	if payload["success"] != false || payload["report"] == nil {
		t.Fatalf("部分失败 report 应标记 success=false: %+v", payload)
	}
}

func TestEmitEmptyConversationPruneReportDoesNotFailForUnknownSkip(t *testing.T) {
	previousOptions := currentOutputOptions
	currentOutputOptions = outputOptions{json: true}
	t.Cleanup(func() { currentOutputOptions = previousOptions })

	command := &cobra.Command{
		Use:           "test-prune-unknown",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return emitEmptyConversationPruneReport(room.EmptyConversationPruneReport{
				Applied: true,
				Unknown: 3,
			})
		},
	}
	stdout, _, executeErr := captureCLIStreams(t, command)
	if executeErr != nil {
		t.Fatalf("纯 unknown/skip 不应返回失败: %v", executeErr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("解析 report 失败: %v stdout=%s", err, stdout)
	}
	if payload["success"] != true {
		t.Fatalf("unknown/skip report 应保持 success=true: %+v", payload)
	}
}

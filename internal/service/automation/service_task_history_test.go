package automation

import (
	"context"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/config"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func TestServiceSearchTaskHistoryIncludesDeletedTaskCandidates(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	activeTask, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "当前新闻日报",
		AgentID:     "agent-1",
		Instruction: "搜索新闻",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("创建 active task 失败: %v", err)
	}
	deletedTask, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "旧新闻日报",
		AgentID:     "agent-1",
		Instruction: "发送旧新闻摘要",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeExplicit, Channel: "feishu", To: "oc_group"},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("创建 deleted task 失败: %v", err)
	}
	scheduledFor := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:        "run-old-news",
		JobID:        deletedTask.JobID,
		OwnerUserID:  deletedTask.OwnerUserID,
		ScheduledFor: &scheduledFor,
		TriggerKind:  "cron",
		DeliveryMode: automationdomain.DeliveryModeExplicit,
		DeliveryTo:   "feishu:oc_group",
	}); err != nil {
		t.Fatalf("插入 deleted task run 失败: %v", err)
	}
	if err = service.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:             "run-old-news",
		Status:            automationdomain.RunStatusSucceeded,
		FinishedAt:        scheduledFor.Add(time.Minute),
		DeliveryStatus:    automationdomain.DeliveryStatusSucceeded,
		DeliveryAttempted: true,
	}); err != nil {
		t.Fatalf("结束 deleted task run 失败: %v", err)
	}
	if _, err = service.DeleteTask(context.Background(), deletedTask.JobID); err != nil {
		t.Fatalf("删除 task 失败: %v", err)
	}

	items, err := service.SearchTaskHistory(context.Background(), automationdomain.CronTaskHistorySearchInput{
		Query:          "新闻",
		IncludeActive:  true,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("SearchTaskHistory 失败: %v", err)
	}
	var activeFound, deletedFound *automationdomain.CronTaskHistoryItem
	for index := range items {
		switch items[index].JobID {
		case activeTask.JobID:
			activeFound = &items[index]
		case deletedTask.JobID:
			deletedFound = &items[index]
		}
	}
	if activeFound == nil || activeFound.Deleted || activeFound.Enabled == nil || !*activeFound.Enabled {
		t.Fatalf("active candidate 不正确: %+v", items)
	}
	if deletedFound == nil || !deletedFound.Deleted || deletedFound.Name != "旧新闻日报" {
		t.Fatalf("deleted candidate 不正确: %+v", items)
	}
	if deletedFound.RunCount != 1 || deletedFound.LastRunStatus != automationdomain.RunStatusSucceeded ||
		deletedFound.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("deleted candidate run 摘要不正确: %+v", deletedFound)
	}

	instructionMatches, err := service.SearchTaskHistory(context.Background(), automationdomain.CronTaskHistorySearchInput{
		Query:          "旧新闻摘要",
		IncludeActive:  false,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("按 instruction 搜索历史失败: %v", err)
	}
	if len(instructionMatches) != 1 || instructionMatches[0].JobID != deletedTask.JobID {
		t.Fatalf("应能按已删除任务指令定位历史候选: %+v", instructionMatches)
	}

	deliveryMatches, err := service.SearchTaskHistory(context.Background(), automationdomain.CronTaskHistorySearchInput{
		Query:          "oc_group",
		IncludeActive:  false,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("按 delivery target 搜索历史失败: %v", err)
	}
	if len(deliveryMatches) != 1 || deliveryMatches[0].JobID != deletedTask.JobID {
		t.Fatalf("应能按已删除任务投递目标定位历史候选: %+v", deliveryMatches)
	}

	deliveryAliasMatches, err := service.SearchTaskHistory(context.Background(), automationdomain.CronTaskHistorySearchInput{
		Query:          "飞书群",
		IncludeActive:  false,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("按 delivery alias 搜索历史失败: %v", err)
	}
	if len(deliveryAliasMatches) != 1 || deliveryAliasMatches[0].JobID != deletedTask.JobID {
		t.Fatalf("应能按已删除任务投递通道别名定位历史候选: %+v", deliveryAliasMatches)
	}
}

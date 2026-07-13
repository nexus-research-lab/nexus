package automation

import (
	"github.com/nexus-research-lab/nexus/internal/automation/types"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestScheduledTaskMatchesQueryUsesDeliveryAndStatusAliases(t *testing.T) {
	job := types.ScheduledTask{
		JobID:   "job-1",
		Enabled: true,
		Running: true,
		Delivery: types.DeliveryTarget{
			Channel: protocol.SessionChannelFeishu,
		},
	}

	for _, query := range []string{"飞书群", "fs", "运行中", "enabled"} {
		if !ScheduledTaskMatchesQuery(job, query) {
			t.Fatalf("expected query %q to match job", query)
		}
	}
	if ScheduledTaskMatchesQuery(job, "停用") {
		t.Fatalf("did not expect disabled alias to match enabled job")
	}
}

func TestQueryVariantsExpandsChannelAliases(t *testing.T) {
	variants := QueryVariants("飞书群")

	for _, expected := range []string{"飞书群", "feishu", "fs", "飞书"} {
		if !slices.Contains(variants, expected) {
			t.Fatalf("expected variants to contain %q, got %#v", expected, variants)
		}
	}
}

func TestBestMatchingScheduledTasksPrefersSpecificNaturalLanguageTarget(t *testing.T) {
	jobs := []types.ScheduledTask{
		{
			JobID:       "job-feishu-weather",
			Name:        "飞书群天气",
			AgentID:     "agent-1",
			Instruction: "发送天气",
			Enabled:     true,
			Delivery: types.DeliveryTarget{
				Channel: protocol.SessionChannelFeishu,
			},
		},
		{
			JobID:       "job-disabled-water",
			Name:        "暂停的喝水提醒",
			AgentID:     "agent-1",
			Instruction: "提醒喝水",
			Enabled:     false,
		},
		{
			JobID:       "job-feishu-news",
			Name:        "暂停的每日新闻摘要",
			AgentID:     "agent-1",
			Instruction: "搜索新闻并投递",
			Enabled:     false,
			Delivery: types.DeliveryTarget{
				Channel: protocol.SessionChannelFeishu,
			},
		},
	}

	matches := BestMatchingScheduledTasks(jobs, "飞书群暂停新闻")

	if len(matches) != 1 || matches[0].JobID != "job-feishu-news" {
		t.Fatalf("expected specific disabled Feishu news task, got %+v", matches)
	}
}

func TestBestMatchingScheduledTasksKeepsEqualTopCandidatesAmbiguous(t *testing.T) {
	jobs := []types.ScheduledTask{
		{JobID: "job-news-a", Name: "早间新闻", AgentID: "agent-1", Enabled: true},
		{JobID: "job-news-b", Name: "晚间新闻", AgentID: "agent-1", Enabled: true},
		{JobID: "job-water", Name: "喝水提醒", AgentID: "agent-1", Enabled: true},
	}

	matches := BestMatchingScheduledTasks(jobs, "新闻")

	if len(matches) != 2 {
		t.Fatalf("expected two equally strong news candidates, got %+v", matches)
	}
}

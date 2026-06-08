package automation

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestCronJobMatchesQueryUsesDeliveryAndStatusAliases(t *testing.T) {
	job := protocol.CronJob{
		JobID:   "job-1",
		Enabled: true,
		Running: true,
		Delivery: protocol.DeliveryTarget{
			Channel: protocol.SessionChannelFeishu,
		},
	}

	for _, query := range []string{"飞书群", "fs", "运行中", "enabled"} {
		if !CronJobMatchesQuery(job, query) {
			t.Fatalf("expected query %q to match job", query)
		}
	}
	if CronJobMatchesQuery(job, "停用") {
		t.Fatalf("did not expect disabled alias to match enabled job")
	}
}

func TestQueryVariantsExpandsChannelAliases(t *testing.T) {
	variants := QueryVariants("飞书群")

	for _, expected := range []string{"飞书群", "feishu", "fs", "飞书"} {
		if !containsString(variants, expected) {
			t.Fatalf("expected variants to contain %q, got %#v", expected, variants)
		}
	}
}

func TestBestMatchingCronJobsPrefersSpecificNaturalLanguageTarget(t *testing.T) {
	jobs := []protocol.CronJob{
		{
			JobID:       "job-feishu-weather",
			Name:        "飞书群天气",
			AgentID:     "agent-1",
			Instruction: "发送天气",
			Enabled:     true,
			Delivery: protocol.DeliveryTarget{
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
			Delivery: protocol.DeliveryTarget{
				Channel: protocol.SessionChannelFeishu,
			},
		},
	}

	matches := BestMatchingCronJobs(jobs, "飞书群暂停新闻")

	if len(matches) != 1 || matches[0].JobID != "job-feishu-news" {
		t.Fatalf("expected specific disabled Feishu news task, got %+v", matches)
	}
}

func TestBestMatchingCronJobsKeepsEqualTopCandidatesAmbiguous(t *testing.T) {
	jobs := []protocol.CronJob{
		{JobID: "job-news-a", Name: "早间新闻", AgentID: "agent-1", Enabled: true},
		{JobID: "job-news-b", Name: "晚间新闻", AgentID: "agent-1", Enabled: true},
		{JobID: "job-water", Name: "喝水提醒", AgentID: "agent-1", Enabled: true},
	}

	matches := BestMatchingCronJobs(jobs, "新闻")

	if len(matches) != 2 {
		t.Fatalf("expected two equally strong news candidates, got %+v", matches)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

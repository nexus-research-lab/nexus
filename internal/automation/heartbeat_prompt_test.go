package automation

import "testing"

func TestParseHeartbeatTasksReadsTasksBlock(t *testing.T) {
	tasks := parseHeartbeatTasks("tasks:\n- name: inbox\n  interval: 30m\n  prompt: \"check inbox\"\n")
	if len(tasks) != 1 {
		t.Fatalf("期望 1 个任务，实际 %d", len(tasks))
	}
	if tasks[0].Name != "inbox" || tasks[0].Interval != "30m" || tasks[0].Prompt != "check inbox" {
		t.Fatalf("任务解析错误: %+v", tasks[0])
	}
}

func TestParseHeartbeatTasksIgnoresNonTaskSections(t *testing.T) {
	tasks := parseHeartbeatTasks(
		"title: heartbeat\n" +
			"notes: keep this short\n" +
			"tasks:\n" +
			"- name: sync\n" +
			"  interval: 15m\n" +
			"  prompt: run sync\n" +
			"\n" +
			"summary: done\n",
	)
	if len(tasks) != 1 || tasks[0].Name != "sync" {
		t.Fatalf("任务段解析错误: %+v", tasks)
	}
}

func TestParseHeartbeatTasksSupportsIndentedFields(t *testing.T) {
	tasks := parseHeartbeatTasks(
		"tasks:\n" +
			"-\n" +
			"  name: backlog\n" +
			"  interval: 1h\n" +
			"  prompt: review backlog\n",
	)
	if len(tasks) != 1 {
		t.Fatalf("期望 1 个任务，实际 %d", len(tasks))
	}
	if tasks[0].Name != "backlog" || tasks[0].Interval != "1h" || tasks[0].Prompt != "review backlog" {
		t.Fatalf("缩进字段解析错误: %+v", tasks[0])
	}
}

func TestParseHeartbeatTasksSupportsMultilinePrompt(t *testing.T) {
	tasks := parseHeartbeatTasks(
		"tasks:\n" +
			"- name: report\n" +
			"  interval: 1h\n" +
			"  prompt: |\n" +
			"    gather metrics\n" +
			"    and summarize\n",
	)
	if len(tasks) != 1 {
		t.Fatalf("期望 1 个任务，实际 %d", len(tasks))
	}
	if tasks[0].Prompt != "gather metrics\nand summarize" {
		t.Fatalf("多行 prompt 解析错误: %q", tasks[0].Prompt)
	}
}

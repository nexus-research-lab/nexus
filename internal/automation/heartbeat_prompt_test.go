package automation

import (
	"reflect"
	"testing"
)

func TestParseHeartbeatTasks(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []HeartbeatTask
	}{
		{
			name: "task block",
			text: "tasks:\n- name: inbox\n  interval: 30m\n  prompt: \"check inbox\"\n",
			want: []HeartbeatTask{{Name: "inbox", Interval: "30m", Prompt: "check inbox"}},
		},
		{
			name: "other sections",
			text: "title: heartbeat\nnotes: keep this short\ntasks:\n- name: sync\n  interval: 15m\n  prompt: run sync\n\nsummary: done\n",
			want: []HeartbeatTask{{Name: "sync", Interval: "15m", Prompt: "run sync"}},
		},
		{
			name: "indented fields",
			text: "tasks:\n-\n  name: backlog\n  interval: 1h\n  prompt: review backlog\n",
			want: []HeartbeatTask{{Name: "backlog", Interval: "1h", Prompt: "review backlog"}},
		},
		{
			name: "multiline prompt",
			text: "tasks:\n- name: report\n  interval: 1h\n  prompt: |\n    gather metrics\n    and summarize\n",
			want: []HeartbeatTask{{Name: "report", Interval: "1h", Prompt: "gather metrics\nand summarize"}},
		},
		{
			name: "next task after multiline prompt",
			text: "tasks:\n- name: report\n  prompt: |\n    gather metrics\n- name: inbox\n  prompt: check inbox\nsummary: done\n",
			want: []HeartbeatTask{{Name: "report", Prompt: "gather metrics"}, {Name: "inbox", Prompt: "check inbox"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseHeartbeatTasks(test.text); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ParseHeartbeatTasks() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestFilterHeartbeatResponse(t *testing.T) {
	tests := []struct {
		response      string
		ackMaxChars   int
		shouldDeliver bool
		text          string
	}{
		{response: "HEARTBEAT_OK", ackMaxChars: 300},
		{response: "HEARTBEAT_OK\nwarn", ackMaxChars: 4},
		{response: "HEARTBEAT_OK\nwarn", ackMaxChars: 3, shouldDeliver: true, text: "warn"},
		{response: "HEARTBEAT_OK\nalert: disk space is low", ackMaxChars: 8, shouldDeliver: true, text: "alert: disk space is low"},
	}
	for _, test := range tests {
		result := FilterHeartbeatResponse(test.response, test.ackMaxChars)
		if result.ShouldDeliver != test.shouldDeliver || result.Text != test.text {
			t.Fatalf("FilterHeartbeatResponse(%q, %d) = %+v", test.response, test.ackMaxChars, result)
		}
	}
}

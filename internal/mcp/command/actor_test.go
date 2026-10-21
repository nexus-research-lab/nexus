package command

import "testing"

func TestActorSubagentCommandAuthorityFollowsSourceBoundary(t *testing.T) {
	base := Actor{
		OwnerUserID: "owner", AgentID: "agent", SessionKey: "session", RoundID: "round",
		LeaseSessionKey: "session", LeaseRoundID: "round",
	}
	for _, test := range []struct {
		name   string
		source string
		want   bool
	}{
		{name: "ordinary agent", source: "agent", want: true},
		{name: "paired agent", source: "agent_paired", want: true},
		{name: "untrusted source", source: "agent_untrusted", want: false},
		{name: "ordinary room handoff", source: "room_handoff", want: false},
		{name: "hidden graph editor", source: "workgraph_editor", want: false},
		{name: "scheduled automation", source: "automation_run", want: false},
		{name: "workgraph execution", source: "workgraph_execution", want: true},
		{name: "room without physical lease context", source: "room", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			actor := base
			actor.SourceContextType = test.source
			if got := actor.SubagentCommandAuthority(); got != test.want {
				t.Fatalf("source %q authority = %t, want %t", test.source, got, test.want)
			}
		})
	}
}

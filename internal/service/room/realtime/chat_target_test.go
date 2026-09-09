package realtime

import (
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBrowserRoomDefaultTargetsRespectHostSettings(t *testing.T) {
	for _, test := range []struct {
		name     string
		host     string
		enabled  bool
		members  map[string]string
		explicit []string
		want     []string
	}{
		{name: "enabled host", host: "lead", enabled: true, want: []string{"lead"}},
		{name: "disabled host", host: "lead"},
		{name: "missing host", enabled: true},
		{name: "removed host", host: "removed", enabled: true},
		{name: "explicit member wins", host: "lead", enabled: true, explicit: []string{"peer"}, want: []string{"peer"}},
		{name: "single group member requires opt in", host: "lead", members: map[string]string{"lead": "Lead"}},
		{name: "single group host opted in", host: "lead", enabled: true, members: map[string]string{"lead": "Lead"}, want: []string{"lead"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			members := test.members
			if members == nil {
				members = map[string]string{"lead": "Lead", "peer": "Peer"}
			}
			room := &protocol.ConversationContextAggregate{Room: protocol.RoomRecord{
				RoomType: protocol.RoomTypeGroup, HostAgentID: test.host, HostAutoReplyEnabled: test.enabled,
			}}
			targets, _ := resolveDefaultRoomTargets(room, members, test.explicit, "explicit_target", true)
			if !slices.Equal(targets, test.want) {
				t.Fatalf("targets = %v, want %v", targets, test.want)
			}
		})
	}
}

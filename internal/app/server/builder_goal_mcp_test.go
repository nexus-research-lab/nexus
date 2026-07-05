package server

import "testing"

func TestResolveGoalMCPSessionKey(t *testing.T) {
	tests := []struct {
		name       string
		sessionKey string
		serverName string
		want       string
	}{
		{
			name:       "shared room goal for group room",
			sessionKey: "agent:devin:ws:group:conversation-1",
			serverName: "room",
			want:       "room:group:conversation-1",
		},
		{
			name:       "keeps room shared key",
			sessionKey: "room:group:conversation-1",
			serverName: "room",
			want:       "room:group:conversation-1",
		},
		{
			name:       "keeps room dm on agent goal",
			sessionKey: "agent:devin:ws:dm:conversation-1",
			serverName: "room",
			want:       "agent:devin:ws:dm:conversation-1",
		},
		{
			name:       "keeps non-room session",
			sessionKey: "agent:devin:ws:group:conversation-1",
			serverName: "automation",
			want:       "agent:devin:ws:group:conversation-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveGoalMCPSessionKey(test.sessionKey, test.serverName); got != test.want {
				t.Fatalf("resolveGoalMCPSessionKey() = %q, want %q", got, test.want)
			}
		})
	}
}

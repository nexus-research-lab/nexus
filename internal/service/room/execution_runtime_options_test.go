package room

import "testing"

func TestRoomContextColdStartUsesWarmRuntimeBeforeResume(t *testing.T) {
	tests := []struct {
		name           string
		resumeID       string
		hadWarmSession bool
		want           bool
	}{
		{name: "new session", want: true},
		{name: "persisted resume", resumeID: "sdk-session-1", want: false},
		{name: "warm manager session", hadWarmSession: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := roomContextColdStart(test.resumeID, test.hadWarmSession); got != test.want {
				t.Fatalf("roomContextColdStart() = %v, want %v", got, test.want)
			}
		})
	}
}

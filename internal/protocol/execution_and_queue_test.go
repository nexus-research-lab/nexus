// execution_and_queue_test.go 覆盖执行输出作用域、Goal 与输入队列协议测试。
package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeWorkOutputScopeRejectsInvalidGrammar(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		mode  WorkOutputScopeMode
	}{
		{name: "empty", scope: ""},
		{name: "untyped", scope: "reports/final"},
		{name: "unknown kind", scope: "resource:reports/final"},
		{name: "empty file", scope: "file:"},
		{name: "empty directory", scope: "dir:  "},
		{name: "empty semantic", scope: "semantic:  "},
		{name: "absolute", scope: "file:/etc/passwd"},
		{name: "windows absolute", scope: "dir:C:/workspace"},
		{name: "dot", scope: "dir:."},
		{name: "parent", scope: "file:../outside"},
		{name: "cleaned parent escape", scope: "dir:web/../../outside"},
		{name: "backslash", scope: `file:web\main.go`},
		{name: "path control", scope: "file:web/\nmain.go"},
		{name: "semantic control", scope: "semantic:report\nfinal"},
		{name: "invalid mode", scope: "semantic:report", mode: "private"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeWorkOutputScope(WorkOutputScope{
				Scope: test.scope,
				Mode:  test.mode,
			})
			if !errors.Is(err, ErrInvalidWorkOutputScope) {
				t.Fatalf("error = %v, want ErrInvalidWorkOutputScope", err)
			}
		})
	}
}

func TestWorkOutputScopesConflictMatrix(t *testing.T) {
	exclusive := WorkOutputScopeExclusive
	shared := WorkOutputScopeShared
	tests := []struct {
		name      string
		left      string
		leftMode  WorkOutputScopeMode
		right     string
		rightMode WorkOutputScopeMode
		want      bool
	}{
		{name: "same file exclusive exclusive", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, want: true},
		{name: "same file exclusive shared", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: shared, want: true},
		{name: "same file shared exclusive", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: exclusive, want: true},
		{name: "same file shared shared", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: shared, want: false},
		{name: "different files", left: "file:web/main.go", leftMode: exclusive, right: "file:web/app.go", rightMode: exclusive, want: false},
		{name: "directory contains file", left: "dir:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: shared, want: true},
		{name: "file inside directory reverse", left: "file:web/src/main.go", leftMode: shared, right: "dir:web", rightMode: exclusive, want: true},
		{name: "directory equals file", left: "dir:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, want: true},
		{name: "parent directory", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: exclusive, want: true},
		{name: "child directory reverse", left: "dir:web/src", leftMode: exclusive, right: "dir:web", rightMode: shared, want: true},
		{name: "parent directory shared", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: shared, want: false},
		{name: "directory boundary", left: "dir:web/src", leftMode: exclusive, right: "file:web/src2/main.go", rightMode: exclusive, want: false},
		{name: "file is not ancestor", left: "file:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: exclusive, want: false},
		{name: "file case fold", left: "file:Web/Main.go", leftMode: exclusive, right: "file:web/main.GO", rightMode: shared, want: true},
		{name: "directory ancestor case fold", left: "dir:Réports", leftMode: exclusive, right: "file:re\u0301ports/Final.md", rightMode: shared, want: true},
		{name: "directory boundary after case fold", left: "dir:WEB/src", leftMode: exclusive, right: "file:web/src2/main.go", rightMode: exclusive, want: false},
		{name: "unicode composed decomposed file", left: "file:résumé.md", leftMode: exclusive, right: "file:re\u0301sume\u0301.md", rightMode: shared, want: true},
		{name: "same semantic", left: "semantic:report", leftMode: exclusive, right: "semantic:report", rightMode: shared, want: true},
		{name: "same semantic shared", left: "semantic:report", leftMode: shared, right: "semantic:report", rightMode: shared, want: false},
		{name: "different semantic", left: "semantic:report", leftMode: exclusive, right: "semantic:appendix", rightMode: exclusive, want: false},
		{name: "semantic remains case sensitive", left: "semantic:Report", leftMode: exclusive, right: "semantic:report", rightMode: exclusive, want: false},
		{name: "semantic and path", left: "semantic:web", leftMode: exclusive, right: "dir:web", rightMode: exclusive, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := WorkOutputScopesConflict(
				WorkOutputScope{Scope: test.left, Mode: test.leftMode},
				WorkOutputScope{Scope: test.right, Mode: test.rightMode},
			)
			if err != nil {
				t.Fatalf("conflict: %v", err)
			}
			if got != test.want {
				t.Fatalf("conflict = %t, want %t", got, test.want)
			}
		})
	}
}

func TestWorkOutputScopesConflictRejectsInvalidOperands(t *testing.T) {
	tests := []struct {
		name  string
		left  WorkOutputScope
		right WorkOutputScope
	}{
		{
			name:  "invalid left grammar",
			left:  WorkOutputScope{Scope: "web/main.go"},
			right: WorkOutputScope{Scope: "file:web/main.go"},
		},
		{
			name:  "invalid right mode",
			left:  WorkOutputScope{Scope: "semantic:report"},
			right: WorkOutputScope{Scope: "semantic:report", Mode: "private"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := WorkOutputScopesConflict(test.left, test.right)
			if !errors.Is(err, ErrInvalidWorkOutputScope) {
				t.Fatalf("error = %v, want ErrInvalidWorkOutputScope", err)
			}
		})
	}
}

func TestGoalJSONProjectsServerDerivedContinuationState(t *testing.T) {
	tests := []struct {
		name  string
		goal  Goal
		state GoalContinuationState
	}{
		{name: "active is ready", goal: Goal{Status: GoalStatusActive}, state: GoalContinuationStateReady},
		{name: "first empty turn is recovering", goal: Goal{Status: GoalStatusActive, EmptyProgressCount: 1}, state: GoalContinuationStateRecovering},
		{name: "second empty turn is suspended", goal: Goal{Status: GoalStatusActive, EmptyProgressCount: GoalContinuationSuppressionThreshold}, state: GoalContinuationStateSuspended},
		{name: "runtime error is suspended", goal: Goal{Status: GoalStatusActive, LastError: "runtime failed"}, state: GoalContinuationStateSuspended},
		{name: "paused lifecycle is inactive", goal: Goal{Status: GoalStatusPaused}, state: GoalContinuationStateInactive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.goal.ContinuationState(); got != test.state {
				t.Fatalf("continuation state = %q, want %q", got, test.state)
			}
			data, err := json.Marshal(test.goal)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if got := wire["continuation_state"]; got != string(test.state) {
				t.Fatalf("wire continuation_state = %v, want %q; payload=%s", got, test.state, data)
			}
		})
	}
}

func TestGoalJSONProjectsContinuationSuspensionReason(t *testing.T) {
	stopped := map[string]any{GoalMetadataContinuationStoppedByUser: true}
	tests := []struct {
		name   string
		goal   Goal
		reason GoalContinuationSuspensionReason
	}{
		{name: "ready has no reason", goal: Goal{Status: GoalStatusActive, Metadata: stopped}},
		{name: "no progress", goal: Goal{Status: GoalStatusActive, EmptyProgressCount: GoalContinuationSuppressionThreshold}, reason: GoalContinuationSuspensionNoProgress},
		{name: "user stop", goal: Goal{Status: GoalStatusActive, EmptyProgressCount: GoalContinuationSuppressionThreshold, Metadata: stopped}, reason: GoalContinuationSuspensionUserStopped},
		{name: "runtime failure wins over stale stop marker", goal: Goal{Status: GoalStatusActive, LastError: "runtime failed", Metadata: stopped}, reason: GoalContinuationSuspensionRuntimeFailure},
		{name: "paused lifecycle has no reason", goal: Goal{Status: GoalStatusPaused, EmptyProgressCount: GoalContinuationSuppressionThreshold, Metadata: stopped}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.goal.ContinuationSuspensionReason(); got != test.reason {
				t.Fatalf("suspension reason = %q, want %q", got, test.reason)
			}
			data, err := json.Marshal(test.goal)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			got, present := wire["continuation_suspension_reason"]
			if test.reason == "" {
				if present {
					t.Fatalf("wire reason = %v, want omitted; payload=%s", got, data)
				}
				return
			}
			if got != string(test.reason) {
				t.Fatalf("wire reason = %v, want %q; payload=%s", got, test.reason, data)
			}
		})
	}
}

func TestValidateInputQueueCapabilityEnvelopeSeparatesConversationAndResponsibility(t *testing.T) {
	validWorkBinding := &ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	validReviewBinding := &ExecutionReviewBinding{
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-worker",
	}
	base := InputQueueItem{
		Scope:          InputQueueScopeRoom,
		AgentID:        "agent-worker",
		TargetAgentIDs: []string{"agent-worker"},
		Source:         InputQueueSourceAgentRoomMessage,
		DeliveryPolicy: ChatDeliveryPolicyQueue,
		WorkBinding:    validWorkBinding,
	}

	tests := []struct {
		name    string
		mutate  func(InputQueueItem) InputQueueItem
		wantErr bool
	}{
		{
			name: "valid work dispatch",
			mutate: func(item InputQueueItem) InputQueueItem {
				return item
			},
		},
		{
			name: "ordinary guidance remains conversation",
			mutate: func(item InputQueueItem) InputQueueItem {
				item.WorkBinding = nil
				item.DeliveryPolicy = ChatDeliveryPolicyGuide
				return item
			},
		},
		{
			name: "bound dispatch cannot become guidance",
			mutate: func(item InputQueueItem) InputQueueItem {
				item.DeliveryPolicy = ChatDeliveryPolicyGuide
				return item
			},
			wantErr: true,
		},
		{
			name: "work and review bindings are exclusive",
			mutate: func(item InputQueueItem) InputQueueItem {
				item.ReviewBinding = validReviewBinding
				return item
			},
			wantErr: true,
		},
		{
			name: "incomplete work binding",
			mutate: func(item InputQueueItem) InputQueueItem {
				cloned := *validWorkBinding
				cloned.AttemptID = ""
				item.WorkBinding = &cloned
				return item
			},
			wantErr: true,
		},
		{
			name: "wrong exact target",
			mutate: func(item InputQueueItem) InputQueueItem {
				item.TargetAgentIDs = []string{"agent-other"}
				return item
			},
			wantErr: true,
		},
		{
			name: "review target must match queue target",
			mutate: func(item InputQueueItem) InputQueueItem {
				item.WorkBinding = nil
				item.ReviewBinding = validReviewBinding
				item.AgentID = "agent-lead"
				item.TargetAgentIDs = []string{"agent-lead"}
				return item
			},
			wantErr: true,
		},
		{
			name: "raw public mention cannot carry responsibility",
			mutate: func(item InputQueueItem) InputQueueItem {
				item.Source = InputQueueSourceAgentPublicMention
				return item
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateInputQueueCapabilityEnvelope(test.mutate(base))
			if test.wantErr && !errors.Is(err, ErrInvalidInputQueueCapabilityEnvelope) {
				t.Fatalf("error = %v, want capability envelope error", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

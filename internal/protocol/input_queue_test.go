package protocol

import (
	"errors"
	"testing"
)

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

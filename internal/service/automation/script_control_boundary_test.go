package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

func TestServiceRejectsAgentActorAtEveryScriptControlEntry(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")

	if _, err := service.CreateTask(agentCtx, scriptControlTaskInput("agent-script-create", automationdomain.ExecutionKindScript)); !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("Agent script create error = %v, want boundary rejection", err)
	}

	for _, test := range []struct {
		name string
		call func(string) error
	}{
		{
			name: "update",
			call: func(jobID string) error {
				name := "changed"
				_, err := service.UpdateTask(agentCtx, jobID, automationdomain.UpdateJobInput{Name: &name})
				return err
			},
		},
		{
			name: "delete",
			call: func(jobID string) error {
				_, err := service.DeleteTask(agentCtx, jobID)
				return err
			},
		},
		{
			name: "run",
			call: func(jobID string) error {
				_, err := service.RunTaskNow(agentCtx, jobID)
				return err
			},
		},
		{
			name: "retry_delivery",
			call: func(jobID string) error {
				_, err := service.RetryRunDelivery(agentCtx, jobID, "missing-run")
				return err
			},
		},
		{
			name: "recover",
			call: func(jobID string) error {
				_, err := service.RecoverTaskRunningRun(agentCtx, jobID, "")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := service.CreateTask(context.Background(), scriptControlTaskInput("script-"+test.name, automationdomain.ExecutionKindScript))
			if err != nil {
				t.Fatalf("human script create failed: %v", err)
			}
			if err = test.call(task.JobID); !errors.Is(err, errAgentScriptControl) {
				t.Fatalf("%s error = %v, want boundary rejection", test.name, err)
			}
		})
	}
}

func TestServiceSerializesConcurrentHumanScriptTransitionBeforeAgentRun(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	task, err := service.CreateTask(context.Background(), scriptControlTaskInput("concurrent-agent", automationdomain.ExecutionKindAgent))
	if err != nil {
		t.Fatalf("create Agent task: %v", err)
	}

	humanPersisted := make(chan struct{})
	releaseHuman := make(chan struct{})
	service.SetTaskEventNotifier(TaskEventNotifierFunc(func(_ context.Context, event automationdomain.ScheduledTaskEvent) {
		if event.JobID != task.JobID || event.Action != automationdomain.TaskEventActionUpdate {
			return
		}
		close(humanPersisted)
		<-releaseHuman
	}))

	humanDone := make(chan error, 1)
	go func() {
		scriptKind := automationdomain.ExecutionKindScript
		_, updateErr := service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{
			ExecutionKind: &scriptKind,
		})
		humanDone <- updateErr
	}()
	<-humanPersisted

	agentDone := make(chan error, 1)
	go func() {
		agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")
		_, runErr := service.RunTaskNow(agentCtx, task.JobID)
		agentDone <- runErr
	}()
	select {
	case runErr := <-agentDone:
		close(releaseHuman)
		<-humanDone
		t.Fatalf("Agent run crossed the in-flight human control boundary: %v", runErr)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseHuman)
	if err = <-humanDone; err != nil {
		t.Fatalf("human script transition failed: %v", err)
	}
	if err = <-agentDone; !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("Agent run after script transition error = %v, want boundary rejection", err)
	}
}

func newScriptControlBoundaryService(t *testing.T) *Service {
	t.Helper()
	return NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
}

func scriptControlTaskInput(name string, executionKind string) automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:          name,
		AgentID:       "agent-1",
		Instruction:   "echo safe",
		ExecutionKind: executionKind,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	}
}

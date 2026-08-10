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

func TestServiceRejectsAgentActorAfterHumanSwitchesTaskToScript(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	task, err := service.CreateTask(context.Background(), scriptControlTaskInput("initial-agent", automationdomain.ExecutionKindAgent))
	if err != nil {
		t.Fatalf("create Agent task: %v", err)
	}
	stale, err := service.GetTask(context.Background(), task.JobID)
	if err != nil || stale == nil {
		t.Fatalf("read stale Agent snapshot: task=%+v err=%v", stale, err)
	}
	if automationdomain.NormalizeExecutionKind(stale.ExecutionKind) == automationdomain.ExecutionKindScript {
		t.Fatalf("precondition task already script: %+v", stale)
	}

	scriptKind := automationdomain.ExecutionKindScript
	if _, err = service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{
		ExecutionKind: &scriptKind,
	}); err != nil {
		t.Fatalf("human switch to script failed: %v", err)
	}

	agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")
	changedName := "stale-agent-update"
	if _, err = service.UpdateTask(agentCtx, task.JobID, automationdomain.UpdateJobInput{Name: &changedName}); !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("stale Agent update error = %v, want boundary rejection", err)
	}
	if _, err = service.RunTaskNow(agentCtx, task.JobID); !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("stale Agent run error = %v, want boundary rejection", err)
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

func TestHumanControlPlaneRetainsScriptTaskManagement(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	task, err := service.CreateTask(context.Background(), scriptControlTaskInput("human-script", automationdomain.ExecutionKindScript))
	if err != nil {
		t.Fatalf("human script create failed: %v", err)
	}
	name := "human-updated-script"
	updated, err := service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{Name: &name})
	if err != nil {
		t.Fatalf("human script update failed: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("human script update not applied: %+v", updated)
	}
	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("human script delete failed: %v", err)
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

// plan_test.go 覆盖编排计划提案、文档渲染与校验测试。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"gopkg.in/yaml.v3"
)

const createPlanProposalDocument = `nexus_plan: 1
operation: create
objective: Deliver a verified report
completion_criteria:
  - report.md exists and verification passes
items:
  - logical_key: produce
    kind: produce
    subject: Produce report
    objective: Write the report
    deliverable: report.md
    acceptance_criteria:
      - report.md contains the requested summary
    required: true
    output_scopes:
      - file:report.md
  - logical_key: verify
    kind: verify
    subject: Verify report
    objective: Verify the report
    deliverable: verification result
    acceptance_criteria:
      - verification passes
    required: true
    terminal: true
    depends_on:
      - produce
`

func TestResolvePlanExecutionProposalCarriesExactBindingAcrossRounds(t *testing.T) {
	repository := &planProposalTestRepository{fakeRepository: &fakeRepository{}}
	service := testService(repository)
	prepareActor := coordinatorActor()
	prepareActor.RootRoundID = "round-plan-mode"
	prepareActor.AgentRoundID = "agent-round-plan-mode"
	prepareActor.PlanMode = true
	prepared, err := service.PreparePlanExecution(
		context.Background(),
		prepareActor,
		PreparePlanExecutionInput{
			CommandID:    "tool-plan-mode-prepare",
			PlanDocument: createPlanProposalDocument,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	commitActor := prepareActor
	commitActor.RootRoundID = "round-after-plan-mode"
	commitActor.AgentRoundID = "agent-round-after-plan-mode"
	commitActor.PlanMode = false
	resolved, err := service.ResolvePlanExecutionProposal(context.Background(), commitActor)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != prepared.ID ||
		resolved.ContentDigest != prepared.ContentDigest ||
		resolved.RootRoundID != prepareActor.RootRoundID {
		t.Fatalf("resolved proposal = %#v, prepared = %#v", resolved, prepared)
	}

	wrongSession := commitActor
	wrongSession.SessionKey = "another-session"
	if _, err = service.ResolvePlanExecutionProposal(context.Background(), wrongSession); err == nil {
		t.Fatal("cross-session proposal binding unexpectedly resolved")
	} else {
		var domainErr *DomainError
		if !errors.As(err, &domainErr) || domainErr.Code != ErrorCodePlanProposalBinding {
			t.Fatalf("cross-session error = %v", err)
		}
	}
}

func TestMaterializePlanExecutionCreatesOnceAndReplaysReceipt(t *testing.T) {
	createCalls := 0
	main := &fakeRepository{}
	main.createWithPlan = func(
		_ context.Context,
		command orchestrationstore.CreateWithPlanCommand,
	) (*protocol.ExecutionSnapshot, error) {
		createCalls++
		main.snapshot = snapshotFromInitialPlan(command.Execution, command.Plan)
		return main.snapshot, nil
	}
	repository := &planProposalTestRepository{fakeRepository: main}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-materialize"

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-create",
		PlanDocument: createPlanProposalDocument,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.Snapshot == nil || result.Snapshot.Plan == nil ||
		createCalls != 1 || repository.proposal.Status != protocol.ExecutionPlanProposalStatusMaterialized ||
		repository.proposal.MaterializedExecutionID != result.ExecutionID ||
		repository.proposal.ReservedExecutionID != result.ExecutionID {
		t.Fatalf("result=%#v proposal=%#v create_calls=%d", result, repository.proposal, createCalls)
	}

	replayed, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Outcome != MutationNoOp || createCalls != 1 || replayed.ExecutionID != result.ExecutionID {
		t.Fatalf("replay=%#v create_calls=%d", replayed, createCalls)
	}
}

func TestGoalReservedExecutionIdentityIsSealedAndReused(t *testing.T) {
	const canonicalObjective = "Deliver the exact persistent Goal objective"
	main := &fakeRepository{}
	main.createWithPlan = func(
		_ context.Context,
		command orchestrationstore.CreateWithPlanCommand,
	) (*protocol.ExecutionSnapshot, error) {
		if command.Execution.ID != "execution-goal-reserved" ||
			command.Execution.Objective != canonicalObjective {
			t.Fatalf("materialized Execution = %#v", command.Execution)
		}
		main.snapshot = snapshotFromInitialPlan(command.Execution, command.Plan)
		return main.snapshot, nil
	}
	repository := &planProposalTestRepository{fakeRepository: main}
	service := testService(repository)
	service.SetExplicitGoalBindingGateway(&proposalGoalGateway{activation: ExplicitGoalActivation{
		GoalID:                "goal-1",
		GoalObjectiveRevision: 4,
		Objective:             canonicalObjective,
		ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
		ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		ReservedExecutionID:   "execution-goal-reserved",
		ReplacesExecutionID:   "execution-goal-predecessor",
	}})
	actor := coordinatorActor()
	actor.RootRoundID = "round-goal-reserved"
	actor.GoalID = "goal-1"
	actor.GoalObjectiveRevision = 4

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-goal-reserved",
		PlanDocument: createPlanProposalDocument,
		GoalBinding:  PlanGoalBindingCurrent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.GoalReservedExecutionID != "execution-goal-reserved" {
		t.Fatalf("sealed Goal reservation = %q", proposal.GoalReservedExecutionID)
	}
	wantDigest, err := protocol.DigestExecutionPlanProposalImmutable(*proposal)
	if err != nil || wantDigest != proposal.ContentDigest {
		t.Fatalf("sealed digest = %q, want %q, err=%v", proposal.ContentDigest, wantDigest, err)
	}
	result, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionID != "execution-goal-reserved" ||
		repository.proposal.ReservedExecutionID != "execution-goal-reserved" ||
		result.Snapshot.Execution.ReplacesExecutionID != "execution-goal-predecessor" ||
		result.GoalAuthority == nil ||
		result.GoalAuthority.GoalID != "goal-1" ||
		result.GoalAuthority.ObjectiveRevision != 4 ||
		result.GoalAuthority.ExecutionID != "execution-goal-reserved" {
		t.Fatalf("result=%#v proposal=%#v", result, repository.proposal)
	}
}

func TestGoalBoundMaterializationDoesNotMintAuthorityWhileConfirmationIsPending(t *testing.T) {
	main := &fakeRepository{}
	main.createWithPlan = func(
		_ context.Context,
		command orchestrationstore.CreateWithPlanCommand,
	) (*protocol.ExecutionSnapshot, error) {
		main.snapshot = snapshotFromInitialPlan(command.Execution, command.Plan)
		return main.snapshot, nil
	}
	repository := &planProposalTestRepository{fakeRepository: main}
	service := testService(repository)
	confirmationErr := errors.New("Goal confirmation unavailable")
	service.SetExplicitGoalBindingGateway(&proposalGoalGateway{
		activation: ExplicitGoalActivation{
			GoalID:                "goal-pending",
			GoalObjectiveRevision: 2,
			Objective:             "Deliver the exact persistent Goal objective",
			ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
			ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
			ReservedExecutionID:   "execution-pending",
		},
		confirmationErr: confirmationErr,
	})
	actor := coordinatorActor()
	actor.RootRoundID = "round-goal-pending"
	actor.GoalID = "goal-pending"
	actor.GoalObjectiveRevision = 2
	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "prepare-goal-pending",
		PlanDocument: createPlanProposalDocument,
		GoalBinding:  PlanGoalBindingCurrent,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.Snapshot == nil ||
		result.GoalAuthority != nil ||
		repository.proposal.ConfirmationState != protocol.ExecutionPlanProposalConfirmationPending ||
		!strings.Contains(result.Message, "confirmation will retry") {
		t.Fatalf("pending confirmation result=%#v proposal=%#v", result, repository.proposal)
	}
}

func TestGoalBoundCreateCanonicalizesPlanObjectiveFromActivation(t *testing.T) {
	const canonicalObjective = "Deliver the exact persistent Goal objective"
	testCases := []struct {
		name     string
		document string
	}{
		{
			name: "omitted transport objective",
			document: strings.Replace(
				createPlanProposalDocument,
				"objective: Deliver a verified report\n",
				"",
				1,
			),
		},
		{
			name: "paraphrased transport objective",
			document: strings.Replace(
				createPlanProposalDocument,
				"objective: Deliver a verified report",
				"objective: Summarize the delivery goal",
				1,
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := &planProposalTestRepository{fakeRepository: &fakeRepository{}}
			service := testService(repository)
			gateway := &proposalGoalGateway{activation: ExplicitGoalActivation{
				GoalID:                "goal-canonical",
				GoalObjectiveRevision: 3,
				Objective:             canonicalObjective,
				ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
				ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
				ReservedExecutionID:   "execution-canonical",
			}}
			service.SetExplicitGoalBindingGateway(gateway)
			actor := coordinatorActor()
			actor.RootRoundID = "round-canonical-" + strings.ReplaceAll(testCase.name, " ", "-")
			actor.GoalID = "goal-canonical"
			actor.GoalObjectiveRevision = 3

			proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
				CommandID:    "prepare-canonical-" + strings.ReplaceAll(testCase.name, " ", "-"),
				PlanDocument: testCase.document,
			})
			if err != nil {
				t.Fatal(err)
			}
			if proposal.Document.Objective != canonicalObjective ||
				proposal.GoalID != "goal-canonical" ||
				proposal.GoalObjectiveRevision != 3 {
				t.Fatalf("sealed proposal boundary = %#v", proposal)
			}
			if gateway.resolveCalls != 1 ||
				gateway.lastActivationRequest.ExistingGoalID != actor.GoalID ||
				gateway.lastActivationRequest.GoalObjectiveRevision != actor.GoalObjectiveRevision {
				t.Fatalf("exact Goal authority request = %#v calls=%d", gateway.lastActivationRequest, gateway.resolveCalls)
			}
			wantDigest, err := protocol.DigestExecutionPlanProposalImmutable(*proposal)
			if err != nil || proposal.ContentDigest != wantDigest {
				t.Fatalf("canonical digest = %q, want %q, err=%v", proposal.ContentDigest, wantDigest, err)
			}
		})
	}
}

func TestGoalFreeCreateStillRequiresDocumentObjective(t *testing.T) {
	repository := &planProposalTestRepository{fakeRepository: &fakeRepository{}}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-goal-free-objective"
	document := strings.Replace(
		createPlanProposalDocument,
		"objective: Deliver a verified report\n",
		"",
		1,
	)

	_, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "prepare-goal-free-objective",
		PlanDocument: document,
	})
	var domainErr *DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != ErrorCodeInvalidInput {
		t.Fatalf("PreparePlanExecution() error = %v, want objective-required invalid input", err)
	}
}

func TestGoalBindingCurrentRequiresExactRoundAuthority(t *testing.T) {
	repository := &planProposalTestRepository{fakeRepository: &fakeRepository{}}
	service := testService(repository)
	gateway := &switchingProposalGoalGateway{active: &ExplicitGoalActivation{
		GoalID:                "goal-ambient",
		GoalObjectiveRevision: 2,
		Objective:             "Ambient Goal must not grant authority",
		ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
		ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		ReservedExecutionID:   "execution-ambient",
	}}
	service.SetExplicitGoalBindingGateway(gateway)
	actor := coordinatorActor()
	actor.RootRoundID = "round-current-without-authority"

	_, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "prepare-current-without-authority",
		PlanDocument: createPlanProposalDocument,
		GoalBinding:  PlanGoalBindingCurrent,
	})
	var domainErr *DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != ErrorCodeGoalBindingConflict {
		t.Fatalf("PreparePlanExecution() error = %v, want Goal binding conflict", err)
	}
	if gateway.resolveCalls != 0 || repository.proposal != nil {
		t.Fatalf("untrusted current resolved ambient Goal or sealed proposal: calls=%d proposal=%#v",
			gateway.resolveCalls, repository.proposal)
	}
}

func TestGoalBindingNoneOverridesExactRoundAuthority(t *testing.T) {
	repository := &planProposalTestRepository{fakeRepository: &fakeRepository{}}
	service := testService(repository)
	gateway := &proposalGoalGateway{activation: ExplicitGoalActivation{
		GoalID:                "goal-exact",
		GoalObjectiveRevision: 7,
		Objective:             "Exact Goal intentionally excluded",
		ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
		ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		ReservedExecutionID:   "execution-exact",
	}}
	service.SetExplicitGoalBindingGateway(gateway)
	actor := coordinatorActor()
	actor.RootRoundID = "round-explicit-goal-free"
	actor.GoalID = "goal-exact"
	actor.GoalObjectiveRevision = 7

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "prepare-explicit-goal-free",
		PlanDocument: createPlanProposalDocument,
		GoalBinding:  PlanGoalBindingNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.GoalID != "" || proposal.GoalObjectiveRevision != 0 || gateway.resolveCalls != 0 {
		t.Fatalf("explicit Goal-free proposal = %#v resolver_calls=%d", proposal, gateway.resolveCalls)
	}
}

func TestPlanGoalBindingIntentRejectsOperationBoundaryChanges(t *testing.T) {
	replanDocument := strings.Replace(
		createPlanProposalDocument,
		"operation: create\nobjective: Deliver a verified report\ncompletion_criteria:\n  - report.md exists and verification passes\n",
		"operation: replan\nrevision_reason: preserve the current boundary\n",
		1,
	)
	replaceDocument := strings.Replace(
		createPlanProposalDocument,
		"operation: create\n",
		"operation: replace\nreplacement_reason: user requested a different transient objective\n",
		1,
	)
	for _, testCase := range []struct {
		name        string
		document    string
		goalBinding PlanGoalBindingIntent
	}{
		{name: "create cannot inherit", document: createPlanProposalDocument, goalBinding: PlanGoalBindingInherit},
		{name: "replan cannot detach", document: replanDocument, goalBinding: PlanGoalBindingNone},
		{name: "replan cannot bind current", document: replanDocument, goalBinding: PlanGoalBindingCurrent},
		{name: "replace cannot detach", document: replaceDocument, goalBinding: PlanGoalBindingNone},
		{name: "replace cannot bind current", document: replaceDocument, goalBinding: PlanGoalBindingCurrent},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repository := &planProposalTestRepository{fakeRepository: &fakeRepository{snapshot: executionSnapshot()}}
			service := testService(repository)
			actor := coordinatorActor()
			actor.RootRoundID = "round-" + strings.ReplaceAll(testCase.name, " ", "-")
			actor.GoalID = "goal-exact"
			actor.GoalObjectiveRevision = 3

			_, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
				CommandID:    "prepare-" + strings.ReplaceAll(testCase.name, " ", "-"),
				PlanDocument: testCase.document,
				GoalBinding:  testCase.goalBinding,
			})
			var domainErr *DomainError
			if !errors.As(err, &domainErr) || domainErr.Code != ErrorCodeInvalidInput {
				t.Fatalf("PreparePlanExecution() error = %v, want invalid input", err)
			}
			if repository.proposal != nil {
				t.Fatalf("invalid boundary intent sealed proposal %#v", repository.proposal)
			}
		})
	}
}

func TestGoalBoundCreateRejectsCanonicalObjectiveDriftBeforeMaterialization(t *testing.T) {
	createCalls := 0
	main := &fakeRepository{
		createWithPlan: func(context.Context, orchestrationstore.CreateWithPlanCommand) (*protocol.ExecutionSnapshot, error) {
			createCalls++
			return nil, nil
		},
	}
	repository := &planProposalTestRepository{fakeRepository: main}
	service := testService(repository)
	gateway := &proposalGoalGateway{activation: ExplicitGoalActivation{
		GoalID:                "goal-objective-fence",
		GoalObjectiveRevision: 5,
		Objective:             "Deliver the exact persistent Goal objective",
		ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
		ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		ReservedExecutionID:   "execution-objective-fence",
	}}
	service.SetExplicitGoalBindingGateway(gateway)
	actor := coordinatorActor()
	actor.RootRoundID = "round-objective-fence"
	actor.GoalID = "goal-objective-fence"
	actor.GoalObjectiveRevision = 5

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "prepare-objective-fence",
		PlanDocument: createPlanProposalDocument,
		GoalBinding:  PlanGoalBindingCurrent,
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway.activation.Objective = "A different persisted objective at the same revision"
	result, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected || result.ReasonCode != ErrorCodePlanProposalStale ||
		createCalls != 0 || repository.proposal.Status != protocol.ExecutionPlanProposalStatusBlocked {
		t.Fatalf("result=%#v proposal=%#v create_calls=%d", result, repository.proposal, createCalls)
	}
}

func TestReplanProposalCreatesFirstPlanForExistingPlanlessExecution(t *testing.T) {
	main := &fakeRepository{snapshot: executionSnapshot()}
	main.snapshot.Execution.CompletionCriteria = []string{"verified"}
	repository := &planProposalTestRepository{fakeRepository: main}
	main.writePlan = func(
		_ context.Context,
		command orchestrationstore.WritePlanCommand,
	) (*protocol.ExecutionSnapshot, error) {
		if command.Plan.BasePlanID != "" || command.Plan.Revision != 1 {
			t.Fatalf("first Plan fence = %#v", command.Plan)
		}
		main.snapshot = snapshotFromInitialPlan(main.snapshot.Execution, command)
		repository.receiptCommandID = strings.TrimSuffix(command.Meta.CommandID, ":plan")
		repository.receiptPlanID = command.Plan.ID
		return main.snapshot, nil
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-planless"
	document := strings.Replace(
		createPlanProposalDocument,
		"operation: create\nobjective: Deliver a verified report\ncompletion_criteria:\n  - report.md exists and verification passes\n",
		"operation: replan\nrevision_reason: establish the first active Plan\n",
		1,
	)
	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-planless",
		PlanDocument: document,
		GoalBinding:  PlanGoalBindingInherit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.BasePlanID != "" || proposal.TargetExecutionID != main.snapshot.Execution.ID {
		t.Fatalf("planless proposal = %#v", proposal)
	}
	result, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.Snapshot == nil ||
		result.Snapshot.Plan == nil || result.Snapshot.Plan.Revision != 1 {
		t.Fatalf("materialized first Plan = %#v", result)
	}
}

func TestMaterializePlanExecutionRejectsDigestAndAccessWithoutWriting(t *testing.T) {
	createCalls := 0
	main := &fakeRepository{
		createWithPlan: func(context.Context, orchestrationstore.CreateWithPlanCommand) (*protocol.ExecutionSnapshot, error) {
			createCalls++
			return nil, nil
		},
	}
	repository := &planProposalTestRepository{fakeRepository: main}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-fence"
	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-fence",
		PlanDocument: createPlanProposalDocument,
	})
	if err != nil {
		t.Fatal(err)
	}

	badDigest, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: strings.Repeat("0", len(proposal.ContentDigest)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if badDigest.Outcome != MutationRejected || badDigest.ReasonCode != ErrorCodePlanProposalDigest {
		t.Fatalf("bad digest result = %#v", badDigest)
	}

	wrongActor := actor
	wrongActor.AgentID = "agent-intruder"
	wrongActor.Role = ExecutionActorCoordinator
	wrongAccess, err := service.MaterializePlanExecution(context.Background(), wrongActor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if wrongAccess.Outcome != MutationRejected || wrongAccess.ReasonCode != ErrorCodeWrongOwner || createCalls != 0 {
		t.Fatalf("wrong access=%#v create_calls=%d", wrongAccess, createCalls)
	}
}

func TestReconcilePlanProposalsRecoversCommitAfterLostResponse(t *testing.T) {
	lostResponse := true
	createCalls := 0
	main := &fakeRepository{}
	repository := &planProposalTestRepository{fakeRepository: main}
	main.createWithPlan = func(
		_ context.Context,
		command orchestrationstore.CreateWithPlanCommand,
	) (*protocol.ExecutionSnapshot, error) {
		createCalls++
		if main.snapshot == nil {
			main.snapshot = snapshotFromInitialPlan(command.Execution, command.Plan)
			main.snapshot.OutputClaims = nil
			for _, work := range command.Plan.WorkItems {
				for _, claim := range work.OutputClaims {
					claim.PlanID = command.Plan.Plan.ID
					claim.ExecutionID = command.Execution.ID
					claim.WorkItemID = work.WorkItem.ID
					claim.SpecID = work.Spec.ID
					main.snapshot.OutputClaims = append(main.snapshot.OutputClaims, claim)
				}
			}
			repository.receiptCommandID = strings.TrimSuffix(command.Plan.Meta.CommandID, ":plan")
			repository.receiptPlanID = command.Plan.Plan.ID
		}
		if lostResponse {
			lostResponse = false
			return nil, errors.New("simulated response loss after commit")
		}
		return main.snapshot, nil
	}
	service := testService(repository)
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	actor := coordinatorActor()
	actor.RootRoundID = "round-recovery"
	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-recovery",
		PlanDocument: createPlanProposalDocument,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err == nil || repository.proposal.Status != protocol.ExecutionPlanProposalStatusMaterializing ||
		main.snapshot == nil {
		t.Fatalf("lost response err=%v proposal=%#v snapshot=%#v", err, repository.proposal, main.snapshot)
	}
	matches, matchErr := proposalMatchesSnapshot(repository.proposal, main.snapshot)
	if matchErr != nil || !matches {
		t.Fatalf("durable snapshot did not match sealed proposal before recovery: matches=%t err=%v snapshot=%#v", matches, matchErr, main.snapshot)
	}
	now = now.Add(time.Minute)
	recovery, err := service.ReconcilePlanProposals(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Scanned != 1 || recovery.Materialized != 1 || recovery.Failed != 0 ||
		repository.proposal.Status != protocol.ExecutionPlanProposalStatusMaterialized || createCalls != 1 {
		t.Fatalf("recovery=%#v proposal=%#v create_calls=%d", recovery, repository.proposal, createCalls)
	}
}

func TestReconcilePlanProposalsConvergesBlockedRaceThroughExactReceipt(t *testing.T) {
	main := &fakeRepository{}
	repository := &planProposalTestRepository{fakeRepository: main}
	main.createWithPlan = func(
		_ context.Context,
		command orchestrationstore.CreateWithPlanCommand,
	) (*protocol.ExecutionSnapshot, error) {
		main.snapshot = snapshotFromInitialPlan(command.Execution, command.Plan)
		repository.receiptCommandID = strings.TrimSuffix(command.Plan.Meta.CommandID, ":plan")
		repository.receiptPlanID = command.Plan.Plan.ID
		return nil, errors.New("simulated crash after authoritative commit")
	}
	service := testService(repository)
	now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	actor := coordinatorActor()
	actor.RootRoundID = "round-blocked-receipt-race"

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-blocked-receipt-race",
		PlanDocument: createPlanProposalDocument,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.MaterializePlanExecution(
		context.Background(),
		actor,
		MaterializePlanExecutionInput{
			ProposalID:     proposal.ID,
			ProposalDigest: proposal.ContentDigest,
		},
	); err == nil || main.snapshot == nil ||
		repository.proposal.Status != protocol.ExecutionPlanProposalStatusMaterializing {
		t.Fatalf("lost commit response err=%v snapshot=%#v proposal=%#v", err, main.snapshot, repository.proposal)
	}

	// A racing worker observed a stale target after the authoritative command
	// committed, then won the proposal CAS before the first worker could save
	// its aggregate receipt.
	repository.proposal.Status = protocol.ExecutionPlanProposalStatusBlocked
	repository.proposal.Version++
	repository.proposal.LastError = "concurrent target fence appeared stale"
	repository.proposal.NextAttemptAt = nil

	recovery, err := service.ReconcilePlanProposals(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Scanned != 1 || recovery.Materialized != 1 || recovery.Failed != 0 ||
		repository.proposal.Status != protocol.ExecutionPlanProposalStatusMaterialized ||
		repository.proposal.MaterializedExecutionID != main.snapshot.Execution.ID ||
		repository.proposal.MaterializedPlanID != main.snapshot.Plan.ID {
		t.Fatalf("recovery=%#v proposal=%#v", recovery, repository.proposal)
	}
}

type planProposalTestRepository struct {
	*fakeRepository
	proposal         *protocol.ExecutionPlanProposal
	receiptCommandID string
	receiptPlanID    string
}

func (r *planProposalTestRepository) FindPlanMaterializationReceipt(
	_ context.Context,
	_ string,
	commandID string,
) (string, error) {
	if strings.TrimSpace(commandID) != strings.TrimSpace(r.receiptCommandID) {
		return "", nil
	}
	return strings.TrimSpace(r.receiptPlanID), nil
}

func (r *planProposalTestRepository) CreateOrGetPlanProposal(
	_ context.Context,
	command orchestrationstore.CreateOrGetPlanProposalCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if r.proposal == nil {
		item := command.Proposal
		r.proposal = &item
	}
	return r.proposal, nil
}

func (r *planProposalTestRepository) GetPlanProposal(
	_ context.Context,
	query orchestrationstore.GetPlanProposalQuery,
) (*protocol.ExecutionPlanProposal, error) {
	if r.proposal == nil || query.Access.ProposalID != r.proposal.ID {
		return nil, nil
	}
	if !planProposalTestAccessMatches(*r.proposal, query.Access) {
		return nil, orchestrationstore.ErrPlanProposalAccess
	}
	return r.proposal, nil
}

func (r *planProposalTestRepository) GetBoundPlanProposal(
	_ context.Context,
	query orchestrationstore.GetBoundPlanProposalQuery,
) (*protocol.ExecutionPlanProposal, error) {
	if r.proposal == nil || !planProposalTestBindingAccessMatches(*r.proposal, query.Access) {
		return nil, nil
	}
	return r.proposal, nil
}

func (r *planProposalTestRepository) MarkPlanProposalMaterializing(
	_ context.Context,
	command orchestrationstore.MarkPlanProposalMaterializingCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if err := r.checkProposalMutation(command.Access, command.ExpectedVersion); err != nil {
		return nil, err
	}
	r.proposal.Status = protocol.ExecutionPlanProposalStatusMaterializing
	r.proposal.Version++
	r.proposal.ReservedExecutionID = command.ReservedExecutionID
	r.proposal.MaterializationCommandID = command.MaterializationCommandID
	r.proposal.GoalActivationOrigin = command.GoalActivationOrigin
	r.proposal.GoalActivationReason = command.GoalActivationReason
	r.proposal.ReplacesExecutionID = command.ReplacesExecutionID
	r.proposal.NextAttemptAt = command.NextAttemptAt
	r.proposal.AttemptCount++
	if r.proposal.GoalID != "" {
		r.proposal.ConfirmationState = protocol.ExecutionPlanProposalConfirmationPending
	}
	return r.proposal, nil
}

func (r *planProposalTestRepository) ClaimPlanProposalMaterializing(
	_ context.Context,
	command orchestrationstore.ClaimPlanProposalMaterializingCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if err := r.checkProposalMutation(command.Access, command.ExpectedVersion); err != nil {
		return nil, err
	}
	if r.proposal.NextAttemptAt != nil && r.proposal.NextAttemptAt.After(command.ClaimAt) {
		return nil, orchestrationstore.ErrPlanProposalNotDue
	}
	r.proposal.Version++
	r.proposal.AttemptCount++
	r.proposal.NextAttemptAt = &command.LeaseUntil
	r.proposal.LastError = ""
	return r.proposal, nil
}

func (r *planProposalTestRepository) MarkPlanProposalMaterialized(
	_ context.Context,
	command orchestrationstore.MarkPlanProposalMaterializedCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if r.proposal.Status == protocol.ExecutionPlanProposalStatusMaterialized &&
		r.proposal.MaterializedExecutionID == command.MaterializedExecutionID &&
		r.proposal.MaterializedPlanID == command.MaterializedPlanID {
		return r.proposal, nil
	}
	if err := r.checkProposalMutation(command.Access, command.ExpectedVersion); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r.proposal.Status = protocol.ExecutionPlanProposalStatusMaterialized
	r.proposal.Version++
	r.proposal.MaterializedExecutionID = command.MaterializedExecutionID
	r.proposal.MaterializedPlanID = command.MaterializedPlanID
	r.proposal.MaterializedAt = &now
	r.proposal.NextAttemptAt = command.NextAttemptAt
	return r.proposal, nil
}

func (r *planProposalTestRepository) MarkPlanProposalConfirmation(
	_ context.Context,
	command orchestrationstore.MarkPlanProposalConfirmationCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if err := r.checkProposalMutation(command.Access, command.ExpectedVersion); err != nil {
		return nil, err
	}
	r.proposal.Version++
	r.proposal.ConfirmationState = command.ConfirmationState
	r.proposal.LastError = command.LastError
	r.proposal.NextAttemptAt = command.NextAttemptAt
	r.proposal.AttemptCount++
	return r.proposal, nil
}

func (r *planProposalTestRepository) MarkPlanProposalBlocked(
	_ context.Context,
	command orchestrationstore.MarkPlanProposalBlockedCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if err := r.checkProposalMutation(command.Access, command.ExpectedVersion); err != nil {
		return nil, err
	}
	r.proposal.Status = protocol.ExecutionPlanProposalStatusBlocked
	r.proposal.Version++
	r.proposal.LastError = command.LastError
	return r.proposal, nil
}

func (r *planProposalTestRepository) SchedulePlanProposalRetry(
	_ context.Context,
	command orchestrationstore.SchedulePlanProposalRetryCommand,
) (*protocol.ExecutionPlanProposal, error) {
	if err := r.checkProposalMutation(command.Access, command.ExpectedVersion); err != nil {
		return nil, err
	}
	r.proposal.Version++
	r.proposal.LastError = command.LastError
	r.proposal.NextAttemptAt = command.NextAttemptAt
	r.proposal.AttemptCount++
	return r.proposal, nil
}

func (r *planProposalTestRepository) ListRecoverablePlanProposals(
	_ context.Context,
	query orchestrationstore.ListRecoverablePlanProposalsQuery,
) ([]protocol.ExecutionPlanProposal, error) {
	if r.proposal == nil || query.Limit <= 0 {
		return nil, nil
	}
	recoverable := r.proposal.Status == protocol.ExecutionPlanProposalStatusMaterializing ||
		(r.proposal.Status == protocol.ExecutionPlanProposalStatusMaterialized &&
			r.proposal.ConfirmationState == protocol.ExecutionPlanProposalConfirmationPending) ||
		(r.proposal.Status == protocol.ExecutionPlanProposalStatusBlocked &&
			strings.TrimSpace(r.receiptCommandID) == strings.TrimSpace(r.proposal.MaterializationCommandID) &&
			strings.TrimSpace(r.receiptPlanID) != "")
	if !recoverable || (r.proposal.NextAttemptAt != nil && r.proposal.NextAttemptAt.After(query.Now)) {
		return nil, nil
	}
	return []protocol.ExecutionPlanProposal{*r.proposal}, nil
}

func (r *planProposalTestRepository) checkProposalMutation(
	access orchestrationstore.PlanProposalAccess,
	expectedVersion int64,
) error {
	if r.proposal == nil || !planProposalTestAccessMatches(*r.proposal, access) {
		return orchestrationstore.ErrPlanProposalAccess
	}
	if r.proposal.Version != expectedVersion {
		return orchestrationstore.ErrVersionConflict
	}
	return nil
}

func planProposalTestAccessMatches(
	proposal protocol.ExecutionPlanProposal,
	access orchestrationstore.PlanProposalAccess,
) bool {
	return proposal.ID == access.ProposalID &&
		proposal.OwnerUserID == access.OwnerUserID &&
		proposal.SessionKey == access.SessionKey &&
		proposal.ScopeKind == access.ScopeKind &&
		proposal.RoomID == access.RoomID &&
		proposal.ConversationID == access.ConversationID &&
		proposal.CoordinatorAgentID == access.CoordinatorAgentID
}

func planProposalTestBindingAccessMatches(
	proposal protocol.ExecutionPlanProposal,
	access orchestrationstore.PlanProposalBindingAccess,
) bool {
	return proposal.OwnerUserID == access.OwnerUserID &&
		proposal.SessionKey == access.SessionKey &&
		proposal.ScopeKind == access.ScopeKind &&
		proposal.RoomID == access.RoomID &&
		proposal.ConversationID == access.ConversationID &&
		proposal.CoordinatorAgentID == access.CoordinatorAgentID
}

type proposalGoalGateway struct {
	activation            ExplicitGoalActivation
	confirmationErr       error
	resolveCalls          int
	lastActivationRequest ExplicitGoalActivationRequest
}

type switchingProposalGoalGateway struct {
	active       *ExplicitGoalActivation
	resolveCalls int
	prepareCalls int
}

func (g *switchingProposalGoalGateway) ResolveExplicitGoalActivation(
	_ context.Context,
	_ ExplicitGoalActivationRequest,
) (*ExplicitGoalActivation, error) {
	g.resolveCalls++
	if g.active == nil {
		return nil, nil
	}
	activation := *g.active
	return &activation, nil
}

func (g *switchingProposalGoalGateway) PrepareExplicitGoalBinding(
	_ context.Context,
	_ ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error) {
	g.prepareCalls++
	return nil, errors.New("mutable Goal binding must not run after the read-only fence rejects")
}

func (g *proposalGoalGateway) ResolveExplicitGoalActivation(
	_ context.Context,
	request ExplicitGoalActivationRequest,
) (*ExplicitGoalActivation, error) {
	g.resolveCalls++
	g.lastActivationRequest = request
	activation := g.activation
	return &activation, nil
}

func (g *proposalGoalGateway) PrepareExplicitGoalBinding(
	_ context.Context,
	request ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error) {
	return &ExplicitGoalBinding{
		ExecutionID:           g.activation.ReservedExecutionID,
		GoalID:                g.activation.GoalID,
		GoalObjectiveRevision: g.activation.GoalObjectiveRevision,
		ActivationOrigin:      g.activation.ActivationOrigin,
		ActivationReason:      g.activation.ActivationReason,
		ReplacesExecutionID:   g.activation.ReplacesExecutionID,
	}, nil
}

func (g *proposalGoalGateway) ConfirmGoalExecutionBinding(
	context.Context,
	GoalExecutionBindingConfirmation,
) error {
	return g.confirmationErr
}

const validMinimalPlanDocument = `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria:
  - Report is verified
items:
  - logical_key: make
    kind: produce
    subject: Make report
    objective: Produce the report
    deliverable: report.md
`

func TestParseExecutionPlanDocumentDigestIgnoresYAMLPresentation(t *testing.T) {
	t.Parallel()

	left := `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria: [Report is verified]
items:
  - logical_key: source_a
    kind: produce
    subject: Source A
    objective: Gather A
    deliverable: A notes
  - logical_key: source_b
    kind: produce
    subject: Source B
    objective: Gather B
    deliverable: B notes
  - logical_key: final
    kind: integrate
    subject: Final
    objective: Integrate and verify
    deliverable: report.md
    acceptance_criteria: []
    depends_on: [source_b, source_a]
    input_refs: [brief.md]
    output_scopes: [semantic:z, semantic:b]
    shared_output_scopes: [semantic:a]
`
	right := `
# Presentation and mapping order do not participate in proposal identity.
operation: create
nexus_plan: 1
completion_criteria:
  - "Report is verified"
objective: " Ship report "
revision_reason: "   "
items:
  - deliverable: A notes
    objective: Gather A
    subject: Source A
    kind: produce
    logical_key: source_a
    acceptance_criteria: []
  - subject: Source B
    objective: Gather B
    deliverable: B notes
    logical_key: source_b
    kind: produce
  - input_refs:
      - brief.md
    logical_key: final
    deliverable: report.md
    subject: Final
    objective: Integrate and verify
    kind: integrate
    depends_on: [source_a, source_b]
    shared_output_scopes: [semantic:a]
    output_scopes:
      - semantic:b
      - semantic:z
`

	leftDocument, _, err := ParseExecutionPlanDocument(left)
	if err != nil {
		t.Fatalf("parse left: %v", err)
	}
	rightDocument, _, err := ParseExecutionPlanDocument(right)
	if err != nil {
		t.Fatalf("parse right: %v", err)
	}
	if !reflect.DeepEqual(leftDocument, rightDocument) {
		t.Fatalf("equivalent YAML produced different typed documents:\nleft  = %#v\nright = %#v", leftDocument, rightDocument)
	}
	changed := strings.Replace(right, "subject: Final", "subject: Final report", 1)
	changedDocument, _, err := ParseExecutionPlanDocument(changed)
	if err != nil {
		t.Fatalf("parse changed: %v", err)
	}
	if reflect.DeepEqual(changedDocument, leftDocument) {
		t.Fatal("semantic change did not change the typed document")
	}
}

func TestParseExecutionPlanDocumentRejectsUnsupportedYAMLAndSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      string
		wantPath    string
		wantMessage string
	}{
		{name: "empty", source: " \n\t", wantPath: "$", wantMessage: "must not be empty"},
		{
			name:        "unsupported version",
			source:      strings.Replace(validMinimalPlanDocument, "nexus_plan: 1", "nexus_plan: 2", 1),
			wantPath:    "$.nexus_plan",
			wantMessage: "unsupported version",
		},
		{
			name:        "unsupported operation",
			source:      strings.Replace(validMinimalPlanDocument, "operation: create", "operation: append", 1),
			wantPath:    "$.operation",
			wantMessage: "operation must be create, replan, or replace",
		},
		{
			name:        "unknown root field",
			source:      validMinimalPlanDocument + "mystery: true\n",
			wantPath:    "$.mystery",
			wantMessage: "unknown field",
		},
		{
			name: "unknown item field",
			source: strings.Replace(
				validMinimalPlanDocument,
				"    kind: produce",
				"    mystery: value\n    kind: produce",
				1,
			),
			wantPath:    "$.items[0].mystery",
			wantMessage: "unknown field",
		},
		{
			name: "duplicate key",
			source: strings.Replace(
				validMinimalPlanDocument,
				"operation: create",
				"operation: create\noperation: create",
				1,
			),
			wantPath:    "$.operation",
			wantMessage: "duplicate mapping key",
		},
		{
			name:        "multiple documents",
			source:      validMinimalPlanDocument + "---\nnexus_plan: 1\n",
			wantPath:    "$",
			wantMessage: "multiple YAML documents",
		},
		{
			name: "anchor and alias",
			source: strings.Replace(
				strings.Replace(validMinimalPlanDocument, "objective: Ship report", "objective: &objective Ship report", 1),
				"subject: Make report",
				"subject: *objective",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "anchors are not allowed",
		},
		{
			name: "explicit tag",
			source: strings.Replace(
				validMinimalPlanDocument,
				"objective: Ship report",
				"objective: !!str Ship report",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "explicit YAML tags",
		},
		{
			name: "merge key",
			source: `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria: [verified]
items:
  - <<: {logical_key: make, kind: produce, subject: Make, objective: Ship, deliverable: report.md}
`,
			wantPath:    "$.items[0].<<",
			wantMessage: "merge keys are not allowed",
		},
		{
			name: "null",
			source: strings.Replace(
				validMinimalPlanDocument,
				"objective: Ship report",
				"objective: null",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "null values are not allowed",
		},
		{
			name: "timestamp",
			source: strings.Replace(
				validMinimalPlanDocument,
				"objective: Ship report",
				"objective: 2026-08-05",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "timestamp values are not allowed",
		},
		{
			name:        "float",
			source:      strings.Replace(validMinimalPlanDocument, "nexus_plan: 1", "nexus_plan: 1.0", 1),
			wantPath:    "$.nexus_plan",
			wantMessage: "scalar type !!float is not allowed",
		},
		{
			name: "wrong collection type",
			source: `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria: [verified]
items: {}
`,
			wantPath:    "$.items",
			wantMessage: "must be a sequence",
		},
		{
			name:        "syntax error",
			source:      "nexus_plan: [\n",
			wantPath:    "$",
			wantMessage: "invalid YAML",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseExecutionPlanDocument(test.source)
			assertPlanDocumentError(t, err, test.wantPath, test.wantMessage)
		})
	}
}

func TestParseExecutionPlanDocumentRejectsResourceAndCombinedCollectionLimits(t *testing.T) {
	t.Parallel()

	t.Run("bytes", func(t *testing.T) {
		t.Parallel()
		_, _, err := ParseExecutionPlanDocument(strings.Repeat("x", maxExecutionPlanDocumentBytes+1))
		assertPlanDocumentError(t, err, "$", "byte limit")
	})
	t.Run("items", func(t *testing.T) {
		t.Parallel()
		var source strings.Builder
		source.WriteString("nexus_plan: 1\noperation: create\nobjective: Ship\ncompletion_criteria: [done]\nitems:\n")
		for index := 0; index <= protocol.ExecutionProjectionCollectionLimit; index++ {
			fmt.Fprintf(
				&source,
				"  - logical_key: item_%d\n    kind: produce\n    subject: Item\n    objective: Work\n    deliverable: out-%d\n",
				index,
				index,
			)
		}
		_, _, err := ParseExecutionPlanDocument(source.String())
		assertPlanDocumentError(t, err, "$.items", "maximum is 32")
	})
	t.Run("combined dependencies", func(t *testing.T) {
		t.Parallel()
		hard := strings.TrimSuffix(strings.Repeat("source,", 17), ",")
		soft := strings.TrimSuffix(strings.Repeat("source,", 16), ",")
		source := fmt.Sprintf(`nexus_plan: 1
operation: create
objective: Ship
completion_criteria: [done]
items:
  - logical_key: source
    kind: produce
    subject: Source
    objective: Gather
    deliverable: notes
  - logical_key: final
    kind: integrate
    subject: Final
    objective: Integrate
    deliverable: report
    depends_on: [%s]
    soft_depends_on: [%s]
`, hard, soft)
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$.items[1].depends_on", "combined hard and soft dependencies")
	})
	t.Run("combined output scopes", func(t *testing.T) {
		t.Parallel()
		exclusive := make([]string, 17)
		shared := make([]string, 16)
		for index := range exclusive {
			exclusive[index] = fmt.Sprintf("semantic:exclusive-%d", index)
		}
		for index := range shared {
			shared[index] = fmt.Sprintf("semantic:shared-%d", index)
		}
		source := fmt.Sprintf(`nexus_plan: 1
operation: create
objective: Ship
completion_criteria: [done]
items:
  - logical_key: final
    kind: integrate
    subject: Final
    objective: Integrate
    deliverable: report
    output_scopes: [%s]
    shared_output_scopes: [%s]
`, strings.Join(exclusive, ","), strings.Join(shared, ","))
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$.items[0].output_scopes", "combined exclusive and shared output scopes")
	})
	t.Run("depth", func(t *testing.T) {
		t.Parallel()
		source := strings.Repeat("[", maxExecutionPlanYAMLDepth+2) + "x" +
			strings.Repeat("]", maxExecutionPlanYAMLDepth+2)
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$*", "nesting depth")
	})
	t.Run("node count", func(t *testing.T) {
		t.Parallel()
		source := "mystery: " + nestedPlanFlowSequence(22, 3) + "\n"
		if len(source) >= maxExecutionPlanDocumentBytes {
			t.Fatalf("node-count fixture is unexpectedly too large: %d bytes", len(source))
		}
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$.mystery*", "node count")
	})
}

func TestParseExecutionPlanDocumentWrapsDomainErrorWithDocumentLocation(t *testing.T) {
	t.Parallel()

	source := strings.Replace(
		validMinimalPlanDocument,
		"    deliverable: report.md",
		"    deliverable: report.md\n    depends_on: [missing]",
		1,
	)
	_, _, err := ParseExecutionPlanDocument(source)
	typed := assertPlanDocumentError(t, err, "$.items[0].depends_on", "unknown_dependency")
	var domainErr *DomainError
	if !errors.As(typed, &domainErr) || domainErr.Code != ErrorCodeUnknownDependency {
		t.Fatalf("wrapped domain error = %#v, want %s", domainErr, ErrorCodeUnknownDependency)
	}
	result := RejectedResult(nil, typed, nil)
	if result.ReasonCode != ErrorCodeUnknownDependency ||
		!strings.Contains(result.Message, "$.items[0].depends_on") ||
		!strings.Contains(result.Message, "line ") ||
		!strings.Contains(result.Message, "column ") {
		t.Fatalf("projected domain error lost location: %#v", result)
	}
}

func TestValidatePlanYAMLNodeRejectsAliasNodes(t *testing.T) {
	t.Parallel()

	node := &yaml.Node{Kind: yaml.AliasNode, Value: "source", Line: 4, Column: 7}
	count := 0
	err := validatePlanYAMLNode(node, "$.items[0].subject", 0, &count)
	typed := assertPlanDocumentError(t, err, "$.items[0].subject", "aliases are not allowed")
	if typed.Line != 4 || typed.Column != 7 {
		t.Fatalf("alias location = %d:%d, want 4:7", typed.Line, typed.Column)
	}
}

func assertPlanDocumentError(
	t *testing.T,
	err error,
	wantPath string,
	wantMessage string,
) *PlanDocumentError {
	t.Helper()
	if err == nil {
		t.Fatal("ParseExecutionPlanDocument() error = nil")
	}
	var typed *PlanDocumentError
	if !errors.As(err, &typed) {
		t.Fatalf("error type = %T, want *PlanDocumentError: %v", err, err)
	}
	pathMatches := typed.Path == wantPath
	if strings.HasSuffix(wantPath, "*") {
		pathMatches = strings.HasPrefix(typed.Path, strings.TrimSuffix(wantPath, "*"))
	}
	if !pathMatches {
		t.Fatalf("error path = %q, want %q: %v", typed.Path, wantPath, typed)
	}
	if typed.Line <= 0 || typed.Column <= 0 {
		t.Fatalf("error location = %d:%d, want positive one-based coordinates", typed.Line, typed.Column)
	}
	if !strings.Contains(typed.Message, wantMessage) {
		t.Fatalf("error message = %q, want substring %q", typed.Message, wantMessage)
	}
	return typed
}

func nestedPlanFlowSequence(width, depth int) string {
	value := "x"
	for range depth {
		entries := make([]string, width)
		for index := range entries {
			entries[index] = value
		}
		value = "[" + strings.Join(entries, ",") + "]"
	}
	return value
}

func TestPlanExecutionReturnsActionableRecoveryForEmptyWorkGraph(t *testing.T) {
	for _, test := range []struct {
		name  string
		draft PlanDraft
	}{
		{name: "empty array", draft: PlanDraft{}},
		{name: "placeholder object", draft: PlanDraft{Items: []PlanWorkItemDraft{{}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := testService(&fakeRepository{})
			result, err := service.PlanExecution(
				context.Background(),
				coordinatorActor(),
				PlanExecutionInput{
					CommandID:          "empty-workgraph",
					Objective:          "Deliver a verified report",
					CompletionCriteria: []string{"report accepted"},
					Draft:              test.draft,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != MutationRejected ||
				result.ReasonCode != ErrorCodePlanItemsEmpty ||
				len(result.NextActions) != 1 ||
				result.NextActions[0].Operation != "prepare_plan_execution" ||
				!strings.Contains(result.NextActions[0].Reason, "Plan Document") ||
				!strings.Contains(result.NextActions[0].Reason, "Work Item") {
				t.Fatalf("empty WorkGraph recovery = %#v", result)
			}
		})
	}
}

func TestValidatePlanDraftRejectsCycleAndUnknownDependency(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].DependsOn = []PlanDependencyDraft{{LogicalKey: "W2"}}
		err := ValidatePlanDraft(draft)
		assertDomainErrorCode(t, err, ErrorCodeDependencyCycle)
	})
	t.Run("unknown", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[1].DependsOn = []PlanDependencyDraft{{LogicalKey: "Missing"}}
		err := ValidatePlanDraft(draft)
		assertDomainErrorCode(t, err, ErrorCodeUnknownDependency)
	})
}

func TestValidatePlanDraftReportsSupportedWorkItemKinds(t *testing.T) {
	draft := validPlanDraft()
	draft.Items[0].Kind = protocol.WorkItemKind("task")
	err := ValidatePlanDraft(draft)
	assertDomainErrorCode(t, err, ErrorCodeInvalidInput)
	if !strings.Contains(err.Error(), "produce, review, verify, or integrate") {
		t.Fatalf("work item kind error is not actionable: %v", err)
	}
}

func TestValidatePlanDraftEnforcesTypedOutputScopeConflicts(t *testing.T) {
	t.Run("missing produce scope", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes = nil
		if err := ValidatePlanDraft(draft); err != nil {
			t.Fatalf("optional produce scope rejected: %v", err)
		}
	})
	t.Run("duplicate produce", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[1].DependsOn = nil
		draft.Items[1].OutputScopes = []protocol.WorkOutputScope{{
			Scope: "file:report/sources/facts.md",
			Mode:  protocol.WorkOutputScopeShared,
		}}
		err := ValidatePlanDraft(draft)
		assertDomainErrorCode(t, err, ErrorCodeOutputScopeConflict)
	})
	t.Run("shared review", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes[0].Mode = protocol.WorkOutputScopeShared
		draft.Items[1].Kind = protocol.WorkItemKindReview
		draft.Items[1].OutputScopes = []protocol.WorkOutputScope{{
			Scope: "file:report/sources/facts.md",
			Mode:  protocol.WorkOutputScopeShared,
		}}
		if err := ValidatePlanDraft(draft); err != nil {
			t.Fatalf("shared review output should be valid: %v", err)
		}
	})
	t.Run("untyped", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes[0].Scope = "report/sources"
		assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeInvalidInput)
	})
	t.Run("case-folded file", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[1].DependsOn = nil
		draft.Items[0].OutputScopes[0].Scope = "file:Report/Résumé.md"
		draft.Items[1].OutputScopes[0].Scope = "file:report/re\u0301sume\u0301.MD"
		assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeOutputScopeConflict)
	})
	t.Run("same item case-folded duplicate", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes = []protocol.WorkOutputScope{
			{Scope: "file:Report/Final.md"},
			{Scope: "file:report/final.MD"},
		}
		assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeInvalidInput)
	})
}

func TestValidatePlanDraftProjectionCollectionLimit(t *testing.T) {
	values := make([]string, protocol.ExecutionProjectionCollectionLimit)
	scopes := make([]protocol.WorkOutputScope, protocol.ExecutionProjectionCollectionLimit)
	for index := range values {
		values[index] = "value-" + string(rune('A'+index))
		scopes[index] = protocol.WorkOutputScope{
			Scope: "semantic:scope-" + string(rune('A'+index)),
			Mode:  protocol.WorkOutputScopeShared,
		}
	}

	atLimit := validPlanDraft()
	atLimit.Items[0].AcceptanceCriteria = append([]string(nil), values...)
	atLimit.Items[0].InputRefs = append([]string(nil), values...)
	atLimit.Items[0].OutputScopes = append([]protocol.WorkOutputScope(nil), scopes...)
	if err := ValidatePlanDraft(atLimit); err != nil {
		t.Fatalf("32-item collections rejected: %v", err)
	}

	overItemLimit := validPlanDraft()
	for len(overItemLimit.Items) <= protocol.ExecutionProjectionCollectionLimit {
		overItemLimit.Items = append(overItemLimit.Items, PlanWorkItemDraft{})
	}
	assertDomainErrorCode(
		t,
		ValidatePlanDraft(overItemLimit),
		ErrorCodeProjectionLimitExceeded,
	)

	tests := []struct {
		name   string
		mutate func(*PlanDraft)
	}{
		{name: "acceptance criteria", mutate: func(draft *PlanDraft) {
			draft.Items[0].AcceptanceCriteria = append(values, "overflow")
		}},
		{name: "input refs", mutate: func(draft *PlanDraft) {
			draft.Items[0].InputRefs = append(values, "overflow")
		}},
		{name: "output scopes", mutate: func(draft *PlanDraft) {
			draft.Items[0].OutputScopes = append(scopes, protocol.WorkOutputScope{Scope: "semantic:overflow"})
		}},
		{name: "direct dependencies", mutate: func(draft *PlanDraft) {
			draft.Items[2].DependsOn = make([]PlanDependencyDraft, protocol.ExecutionProjectionCollectionLimit+1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			draft := validPlanDraft()
			test.mutate(&draft)
			assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeProjectionLimitExceeded)
		})
	}
}

func validPlanDraft() PlanDraft {
	return PlanDraft{Items: []PlanWorkItemDraft{
		{
			LogicalKey:         "W1",
			Kind:               protocol.WorkItemKindProduce,
			Subject:            "Collect evidence",
			Objective:          "Collect official facts",
			Deliverable:        "Source-backed fact table",
			AcceptanceCriteria: []string{"Every claim has a source"},
			Required:           true,
			OutputScopes: []protocol.WorkOutputScope{{
				Scope: "dir:report/sources",
				Mode:  protocol.WorkOutputScopeExclusive,
			}},
		},
		{
			LogicalKey:         "W2",
			Kind:               protocol.WorkItemKindProduce,
			Subject:            "Analyze",
			Objective:          "Compare accepted facts",
			Deliverable:        "Difference matrix",
			AcceptanceCriteria: []string{"Uses W1"},
			Required:           true,
			DependsOn:          []PlanDependencyDraft{{LogicalKey: "W1"}},
			OutputScopes: []protocol.WorkOutputScope{{
				Scope: "dir:report/analysis",
				Mode:  protocol.WorkOutputScopeExclusive,
			}},
		},
		{
			LogicalKey:         "W3",
			Kind:               protocol.WorkItemKindIntegrate,
			Subject:            "Integrate",
			Objective:          "Deliver the verified result",
			Deliverable:        "Final report",
			AcceptanceCriteria: []string{"Covers all requested dimensions"},
			Required:           true,
			Terminal:           true,
			DependsOn:          []PlanDependencyDraft{{LogicalKey: "W2"}},
		},
	}}
}

func assertDomainErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	var domainErr *DomainError
	if !errors.As(err, &domainErr) {
		t.Fatalf("error = %v, want DomainError(%s)", err, want)
	}
	if domainErr.Code != want {
		t.Fatalf("error code = %s, want %s: %v", domainErr.Code, want, err)
	}
}

// ValidatePlanDraft 检查结构完整性、DAG 与已声明 output scope 冲突。
func ValidatePlanDraft(draft PlanDraft) error {
	_, err := NormalizeAndValidatePlanDraft(draft)
	return err
}

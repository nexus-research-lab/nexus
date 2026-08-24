// INPUT: strict Plan Document、in-memory proposal saga 与可控 authoritative Repository failures。
// OUTPUT: Plan Mode sealing、exact-fence commit、idempotent replay、stale blocking 与 crash recovery 证明。
// POS: service 层 ExecutionPlanProposal 两阶段协议的端到端行为测试。
package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
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

func TestPreparePlanExecutionSealsProposalInPlanModeWithoutAuthoritativeWrite(t *testing.T) {
	main := &fakeRepository{
		createWithPlan: func(context.Context, orchestrationstore.CreateWithPlanCommand) (*protocol.ExecutionSnapshot, error) {
			t.Fatal("Plan Mode preparation wrote authoritative Execution state")
			return nil, nil
		},
	}
	repository := &planProposalTestRepository{fakeRepository: main}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-prepare"
	actor.PlanMode = true

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-1",
		PlanDocument: createPlanProposalDocument,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal == nil || proposal.Status != protocol.ExecutionPlanProposalStatusSealed ||
		proposal.ContentDigest == "" || proposal.Document.Operation != protocol.ExecutionPlanProposalCreate ||
		len(proposal.Document.Items) != 2 || proposal.RootRoundID != actor.RootRoundID {
		t.Fatalf("sealed proposal = %#v", proposal)
	}
	wantDigest, err := protocol.DigestExecutionPlanProposalImmutable(*proposal)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.ContentDigest != wantDigest || main.snapshot != nil {
		t.Fatalf("proposal digest/state = %q / %#v", proposal.ContentDigest, main.snapshot)
	}
}

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

func TestGoalFreeProposalIgnoresAmbientGoalWithoutExactAuthority(t *testing.T) {
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
	gateway := &switchingProposalGoalGateway{active: &ExplicitGoalActivation{
		GoalID:                "goal-ambient",
		GoalObjectiveRevision: 1,
		Objective:             "Ambient Goal objective",
		ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
		ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		ReservedExecutionID:   "execution-ambient",
	}}
	service.SetExplicitGoalBindingGateway(gateway)
	actor := coordinatorActor()
	actor.RootRoundID = "round-goal-free-fence"

	proposal, err := service.PreparePlanExecution(context.Background(), actor, PreparePlanExecutionInput{
		CommandID:    "tool-prepare-goal-free",
		PlanDocument: createPlanProposalDocument,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.GoalID != "" {
		t.Fatalf("proposal unexpectedly bound Goal %q", proposal.GoalID)
	}
	if gateway.resolveCalls != 0 {
		t.Fatalf("Goal-free prepare resolved ambient Goal %d times", gateway.resolveCalls)
	}
	result, err := service.MaterializePlanExecution(context.Background(), actor, MaterializePlanExecutionInput{
		ProposalID:     proposal.ID,
		ProposalDigest: proposal.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.Snapshot == nil ||
		result.Snapshot.Execution.GoalID != "" || createCalls != 1 ||
		result.GoalAuthority != nil ||
		gateway.resolveCalls != 0 || gateway.prepareCalls != 0 ||
		repository.proposal.Status != protocol.ExecutionPlanProposalStatusMaterialized {
		t.Fatalf("result=%#v proposal=%#v create_calls=%d prepare_calls=%d",
			result, repository.proposal, createCalls, gateway.prepareCalls)
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

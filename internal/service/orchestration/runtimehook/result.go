// INPUT: A bound host round and explicit JSON declaration in its Stop payload.
// OUTPUT: Strict durable result intake or a bounded correction; plain text never writes work state.
// POS: Result-hook transport adapter, separate from failure-tolerant runtime observation.
package runtimehook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

const resultMarker = "<!-- nexus-workgraph-result\n"

type ResultProvider interface {
	AcceptRuntimeResult(context.Context, orchestration.ActorContext, protocol.ExecutionResultDeclaration) (orchestration.RuntimeResultOutcome, error)
}

// ResultContract is advertised only for a complete binding present at round start.
// HTML comments keep the transport declaration out of the rendered conversation.
func ResultContract(actor orchestration.ActorContext) string {
	if actor.PlanMode || actor.ObservationOnly || actor.Role == orchestration.ExecutionActorSubagent || actor.AgentRoundID == "" || actor.RuntimeRoundID == "" {
		return ""
	}
	example := protocol.ExecutionResultDeclaration{Version: 1, RoundID: actor.AgentRoundID}
	if actor.WorkBinding != nil && actor.WorkBinding.Complete() && actor.ReviewBinding == nil {
		example.Delivery = &protocol.ExecutionDeliveryResult{ResultSummary: "Describe the delivered result", ResultRefs: []string{"actual artifact reference"}, Evidence: []string{"observed evidence"}}
	} else if actor.ReviewBinding != nil && actor.ReviewBinding.Complete() && actor.WorkBinding == nil {
		example.Review = &protocol.ExecutionReviewResult{Decision: protocol.WorkAcceptanceAccepted, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "exact criterion", Passed: true, Evidence: []string{"observed evidence"}}}}
	} else {
		return ""
	}
	encoded, _ := json.Marshal(example)
	return "This round supports durable WorkGraph result writeback. When you explicitly decide to deliver or review your bound responsibility, append exactly one JSON declaration as the final HTML comment in your last reply. Use actual summary, references, evidence and all exact criteria; the example is not a decision. Review decision must be accepted, rejected or changes_requested. Omit the comment when unfinished, blocked, interrupted or already reported through an explicit tool. Do not also call submit_work/review_work for the same result. The host persists intake and applies the existing domain checks; runtime completion alone never means accepted. Ordinary tools remain available when no declaration is appropriate. No extra fields, duplicate keys or other result targets.\n" + resultMarker + string(encoded) + "\n-->"
}

func ResultCallback(provider ResultProvider, actor orchestration.ActorContext, logger *slog.Logger) sdkhook.Callback {
	// Freeze authority at registration: a late Stop cannot use a successor binding.
	actor.WorkBinding = actor.WorkBinding.Clone()
	actor.ReviewBinding = actor.ReviewBinding.Clone()
	return func(ctx context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if input.IsInterrupt || input.Error != "" || input.AgentID != "" {
			return sdkhook.Output{}, nil
		}
		declaration, err := parseResultDeclaration(input.LastAssistantMessage)
		if err == nil && declaration == nil {
			return sdkhook.Output{}, nil
		}
		if err == nil && declaration.RoundID != actor.AgentRoundID {
			err = errors.New("result belongs to a different round")
		}
		var outcome orchestration.RuntimeResultOutcome
		if err == nil {
			outcome, err = provider.AcceptRuntimeResult(ctx, actor, *declaration)
		}
		if err == nil && outcome.State != "rejected" {
			if logger != nil {
				logger.Info("WorkGraph result hook", "state", outcome.State)
			}
			return sdkhook.Output{}, nil
		}
		if logger != nil {
			logger.Warn("WorkGraph result hook not applied", "state", outcome.State)
		}
		reason := "WorkGraph result was not confirmed. Inspect the current work state before proceeding; correct the explicit result declaration or use the existing result tool only after reconciliation. Do not assume delivery or acceptance."
		if input.StopHookActive != nil && *input.StopHookActive {
			stop := false
			return sdkhook.Output{Continue: &stop, StopReason: reason}, nil
		}
		return sdkhook.Output{Decision: "block", Reason: reason}, nil
	}
}

func parseResultDeclaration(text string) (*protocol.ExecutionResultDeclaration, error) {
	if !strings.Contains(text, "<!-- nexus-workgraph-result") {
		return nil, nil
	}
	if strings.Count(text, "<!-- nexus-workgraph-result") != 1 {
		return nil, errors.New("exactly one result declaration is required")
	}
	prefix, suffix, ok := strings.Cut(text, resultMarker)
	if !ok || (prefix != "" && !strings.HasSuffix(prefix, "\n")) || resultInsideFence(prefix) {
		return nil, errors.New("invalid result marker")
	}
	suffix = strings.TrimSpace(suffix)
	if !strings.HasSuffix(suffix, "-->") {
		return nil, errors.New("result declaration must end the reply")
	}
	payload := strings.TrimSpace(strings.TrimSuffix(suffix, "-->"))
	if len(payload) == 0 || len(payload) > 64*1024 {
		return nil, errors.New("result declaration size is invalid")
	}
	if err := rejectDuplicateJSONKeys([]byte(payload)); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	var result protocol.ExecutionResultDeclaration
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if result.Version != 1 || result.RoundID == "" || (result.Delivery == nil) == (result.Review == nil) {
		return nil, errors.New("result requires version, round_id and exactly one result kind")
	}
	if delivery := result.Delivery; delivery != nil {
		delivery.ResultSummary = strings.TrimSpace(delivery.ResultSummary)
		delivery.ResultRefs = normalizedResultStrings(delivery.ResultRefs)
		delivery.Evidence = normalizedResultStrings(delivery.Evidence)
	}
	if review := result.Review; review != nil {
		review.Feedback = strings.TrimSpace(review.Feedback)
		for i := range review.CriteriaResults {
			criterion := &review.CriteriaResults[i]
			criterion.Criterion = strings.TrimSpace(criterion.Criterion)
			criterion.Note = strings.TrimSpace(criterion.Note)
			criterion.Evidence = normalizedResultStrings(criterion.Evidence)
		}
	}
	return &result, nil
}

// Declarations must be top-level comments, not quoted code/examples. The wire
// requires a column-zero marker; this scanner only guards fenced prefixes.
func resultInsideFence(prefix string) bool {
	var marker byte
	length := 0
	for _, line := range strings.Split(prefix, "\n") {
		line = strings.TrimLeft(line, " ")
		if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
			continue
		}
		n := 1
		for n < len(line) && line[n] == line[0] {
			n++
		}
		if n < 3 {
			continue
		}
		if marker == 0 {
			marker, length = line[0], n
		} else if line[0] == marker && n >= length && strings.TrimSpace(line[n:]) == "" {
			marker, length = 0, 0
		}
	}
	return marker != 0
}

func normalizedResultStrings(values []string) []string {
	var result []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func rejectDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 16 {
			return errors.New("result nesting limit exceeded")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			keys := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || name != strings.ToLower(name) || keys[name] {
					return errors.New("duplicate or invalid result field")
				}
				keys[name] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid result delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing result data")
	}
	return nil
}

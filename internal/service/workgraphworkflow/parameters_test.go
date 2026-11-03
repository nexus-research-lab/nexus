package workgraphworkflow

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"strings"
	"testing"
)

func TestTypedParametersAndInvocation(t *testing.T) {
	minimum, maximum := 1.0, 5.0
	workflow := protocol.WorkGraphWorkflow{ID: "template", Version: 2, SlashName: "report", Parameters: []protocol.WorkGraphParameter{
		{Name: "topic", Type: "string", Required: true}, {Name: "count", Type: "integer", Default: 3.0, Minimum: &minimum, Maximum: &maximum},
		{Name: "format", Type: "string", Enum: []string{"md", "txt"}, Default: "md"}, {Name: "links", Type: "string_list"}, {Name: "verbose", Type: "boolean"},
	}}
	request, source, err := parameterInvocation(workflow, `{"parameters":{"topic":"$(do not execute)","links":["fixture:a"]},"request":"prepare"}`)
	if err != nil || request != "prepare" || source.Parameters["topic"] != "$(do not execute)" || source.Parameters["count"] != 3.0 || source.Version != 2 {
		t.Fatalf("valid invocation: %+v %s %v", source, request, err)
	}
	for _, raw := range []string{`null`, `{}`, `{"parameters":{"topic":"x","count":null}}`, `{"parameters":{"topic":"x","count":"3"}}`, `{"parameters":{"topic":"x","count":1.2}}`, `{"parameters":{"topic":"x","count":6}}`, `{"parameters":{"topic":"x","format":"pdf"}}`, `{"parameters":{"topic":"x","unknown":true}}`, `{"parameters":{"topic":"x","verbose":1}}`, `{"parameters":{"topic":"x","links":[1]}}`, `{"parameters":{"topic":"x","topic":"y"}}`, `{"parameters":{"topic":"x"},"extra":true}`, `{"parameters":{"topic":"x"}} {}`} {
		if _, _, err = parameterInvocation(workflow, raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	repository := &workflowMemoryRepository{items: map[string]protocol.WorkGraphWorkflow{"template": workflow}}
	workflow.OwnerUserID = "owner"
	repository.items[workflow.ID] = workflow
	service := NewService(repository, nil)
	if _, err = service.ValidateWorkflowSource(t.Context(), "other", *source); err == nil {
		t.Fatal("cross-owner template")
	}
	source.Version = 1
	if _, err = service.ValidateWorkflowSource(t.Context(), "owner", *source); err == nil {
		t.Fatal("stale template version")
	}
	expanded, err := service.ExpandRuntimePrompt(t.Context(), "owner", `/report {"parameters":{"topic":"test"}}`)
	if err != nil || !strings.Contains(expanded, `"version":2`) || !strings.Contains(expanded, `"count":3`) {
		t.Fatalf("expansion: %s %v", expanded, err)
	}
}

func TestIntegerParameterRejectsEmptyRange(t *testing.T) {
	minimum, maximum := 1.2, 1.3
	if err := validateParameters([]protocol.WorkGraphParameter{{Name: "count", Type: "integer", Minimum: &minimum, Maximum: &maximum}}); err == nil {
		t.Fatal("integer parameter accepted an interval with no valid value")
	}
}

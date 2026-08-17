package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type runtimeAutomationRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn runtimeAutomationRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestRuntimeAutomationControllerUsesCapabilityAndStrictEnvelope(t *testing.T) {
	controller := remoteRuntimeAutomationController{
		endpoint: "http://127.0.0.1:8010/internal/runtime/automation",
		token:    "capability",
		client: &http.Client{Transport: runtimeAutomationRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get(protocol.NexusCommandCapabilityHeader) != "capability" {
				t.Fatalf("capability header = %q", request.Header.Get(protocol.NexusCommandCapabilityHeader))
			}
			payload, err := io.ReadAll(request.Body)
			if err != nil || !bytes.Contains(payload, []byte(`"action":"contract"`)) {
				t.Fatalf("request payload = %s err=%v", payload, err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body: io.NopCloser(strings.NewReader(
					`{"success":true,"data":{"query_operations":["list"],"mutation_operations":[],"mutation_allowed":false,"cross_agent_allowed":false,"operations":{}}}`,
				)),
			}, nil
		})},
	}
	contract, err := controller.Contract(context.Background())
	if err != nil {
		t.Fatalf("Contract: %v", err)
	}
	if len(contract.QueryOperations) != 1 || contract.QueryOperations[0] != "list" || contract.MutationAllowed {
		t.Fatalf("contract = %+v", contract)
	}
}

func TestDecodeRuntimeAutomationInputRejectsUnknownIdentityFields(t *testing.T) {
	if _, err := decodeRuntimeAutomationInput(`{"owner_user_id":"forged"}`); err == nil {
		t.Fatal("unknown owner identity field was accepted")
	}
	input, err := decodeRuntimeAutomationInput(`{"name":"task","deliver_result":true}`)
	if err != nil || input.Name != "task" || input.DeliverResult == nil || !*input.DeliverResult {
		t.Fatalf("decoded input = %+v err=%v", input, err)
	}
}

var _ runtimeAutomationController = remoteRuntimeAutomationController{}

package artifact

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

type deliveryService struct {
	owner, agent string
	calls        int
	err          error
}

func (s *deliveryService) ValidateDeliverables(ctx context.Context, agent string, paths []string) ([]string, error) {
	s.owner, s.agent = authctx.OwnerUserID(ctx), agent
	s.calls++
	return paths, s.err
}

func TestDeliveryToolBindsScopeAndValidatesBeforeService(t *testing.T) {
	svc := &deliveryService{}
	tool := BuildTools(svc, Context{OwnerUserID: "owner", AgentID: "author", AgentRoundID: "round"})[0]
	for _, input := range []map[string]any{
		{"paths": []string{"out.pdf"}, "agent_id": "other"},
		{"paths": []any{42}}, {"paths": []string{}}, {"paths": []string{" "}},
	} {
		result, err := tool.Handler(context.Background(), input)
		if err != nil || !result.IsError || svc.calls != 0 {
			t.Fatalf("invalid input reached service: %+v %v", result, err)
		}
	}
	result, err := tool.Handler(context.Background(), map[string]any{"paths": []string{"out.pdf", "out.xlsx"}})
	if err != nil || result.IsError || svc.owner != "owner" || svc.agent != "author" {
		t.Fatalf("wrong scope: %+v %+v", result, svc)
	}
	if result.StructuredContent["agent_round_id"] != "round" {
		t.Fatalf("missing round: %+v", result)
	}
	svc.err = errors.New("file missing")
	result, _ = tool.Handler(context.Background(), map[string]any{"paths": []string{"out.pdf"}})
	if !result.IsError || result.StructuredContent != nil {
		t.Fatal("failed validation yielded receipt")
	}
	tool = BuildTools(svc, Context{OwnerUserID: "owner", AgentID: "author"})[0]
	result, _ = tool.Handler(context.Background(), map[string]any{"paths": []string{"out.pdf"}})
	if !result.IsError {
		t.Fatal("missing round accepted")
	}
}

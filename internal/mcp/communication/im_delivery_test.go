package communication

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	"testing"
)

func TestIMQueryRejectsScopeWideningBeforeService(t *testing.T) {
	for _, args := range []map[string]any{
		{"scope": "delivery_sources", "session_key": "other"},
		{"scope": "address_book", "delivery_id": "d"},
		{"scope": "delivery_sources", "delivery_id": "d", "query": "other"},
		{"scope": "delivery_sources", "limit": 1.5},
		{"scope": "delivery_sources", "limit": 51},
		{"scope": "delivery_sources", "offset": -1},
		{"scope": true},
		{"scope": ""},
		{"scope": "delivery_sources", "delivery_id": ""},
	} {
		if _, _, err := parseDeliveryQuery(args); err == nil {
			t.Fatalf("accepted %+v", args)
		}
	}
	if _, lookup, err := parseDeliveryQuery(nil); err != nil || lookup {
		t.Fatalf("default directory changed %v %v", lookup, err)
	}
	q, lookup, err := parseDeliveryQuery(map[string]any{"scope": "delivery_sources", "query": "草案", "limit": float64(5)})
	if err != nil || !lookup || q.Limit != 5 || q.Query != "草案" {
		t.Fatalf("query %+v %v", q, err)
	}
}

func TestIMDeliveryCallIdentitySupportsMissingSDKMetadata(t *testing.T) {
	ctx := RuntimeContext{Actor: communicationsvc.Actor{OwnerUserID: "owner", AgentID: "agent", SessionKey: "source", RoundID: "round"}}
	input := map[string]any{"destination": "external_session", "target_id": "im", "content": "草案"}
	first, err := imDeliveryCallID(ctx, nil, input)
	if err != nil || first == "" {
		t.Fatalf("missing SDK metadata blocked send: %q %v", first, err)
	}
	retry, err := imDeliveryCallID(ctx, &sdktool.CallContext{}, map[string]any{"content": "草案", "target_id": "im", "destination": "external_session"})
	if err != nil || retry != first {
		t.Fatalf("retry identity changed: %q %v", retry, err)
	}
	for _, change := range []func(*RuntimeContext){func(c *RuntimeContext) { c.Actor.OwnerUserID = "other" }, func(c *RuntimeContext) { c.Actor.AgentID = "other" }, func(c *RuntimeContext) { c.Actor.SessionKey = "other" }, func(c *RuntimeContext) { c.Actor.RoundID = "other" }, func(c *RuntimeContext) { c.CurrentAgentRoundID = "other" }} {
		other := ctx
		change(&other)
		id, err := imDeliveryCallID(other, nil, input)
		if err != nil || id == first {
			t.Fatalf("host identities collapsed %q %v", id, err)
		}
	}
	for _, key := range []string{"content", "target_id"} {
		changed := map[string]any{"destination": "external_session", "target_id": "im", "content": "草案"}
		changed[key] = "changed"
		id, err := imDeliveryCallID(ctx, nil, changed)
		if err != nil || id == first {
			t.Fatalf("different intent collapsed %q %v", id, err)
		}
	}
	exact, err := imDeliveryCallID(ctx, &sdktool.CallContext{ToolUseID: "sdk-call"}, input)
	if err != nil || exact != "sdk-call" {
		t.Fatalf("SDK identity lost %q %v", exact, err)
	}
	ctx.Actor.RoundID = ""
	if _, err = imDeliveryCallID(ctx, nil, input); err == nil {
		t.Fatal("missing host round accepted")
	}
}

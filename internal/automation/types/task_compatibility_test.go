package types

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeScheduledTaskCompatibilityMapsLegacyExternalSessionDelivery(t *testing.T) {
	externalSession := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"account-old",
		"contact-old",
		"",
	)
	task := ScheduledTask{
		ExecutionKind: "",
		Delivery: DeliveryTarget{
			Mode:    DeliveryModeExplicit,
			Channel: protocol.SessionChannelWebSocket,
			To:      externalSession,
		},
		Source: Source{Kind: SourceKindUserPage},
	}

	normalized := NormalizeScheduledTaskCompatibility(task)
	if normalized.Delivery.Mode != DeliveryModeLast ||
		normalized.Delivery.SessionKey != externalSession ||
		normalized.Delivery.Channel != "" ||
		normalized.Delivery.To != "" {
		t.Fatalf("legacy external delivery was not mapped to session route: %+v", normalized.Delivery)
	}
	if normalized.ExecutionKind != ExecutionKindAgent ||
		normalized.PermissionMode != PermissionModeDefault {
		t.Fatalf("legacy execution defaults were not retained: %+v", normalized)
	}
	if normalized.DeliveryGrant != normalized.Source {
		t.Fatalf("legacy delivery grant should copy source exactly: %+v", normalized)
	}
}

func TestNormalizeScheduledTaskCompatibilityPreservesIndependentDeliveryGrant(t *testing.T) {
	task := ScheduledTask{
		Source: Source{
			Kind:           SourceKindAgent,
			CreatorAgentID: "agent-1",
			ContextType:    "agent",
			ContextID:      "agent-1",
		},
		DeliveryGrant: Source{Kind: SourceKindUserPage},
	}

	normalized := NormalizeScheduledTaskCompatibility(task)
	if normalized.Source.Kind != SourceKindAgent ||
		normalized.Source.CreatorAgentID != "agent-1" {
		t.Fatalf("creation provenance was rewritten: %+v", normalized.Source)
	}
	if normalized.DeliveryGrant.Kind != SourceKindUserPage ||
		normalized.DeliveryGrant.CreatorAgentID != "" {
		t.Fatalf("independent delivery grant was not preserved: %+v", normalized.DeliveryGrant)
	}
}

func TestNormalizeScheduledTaskCompatibilityDoesNotGuessLegacyPlatformTarget(t *testing.T) {
	task := ScheduledTask{
		Delivery: DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   protocol.SessionChannelFeishu,
			To:        "opaque-chat-id",
			AccountID: "account-1",
		},
	}

	normalized := NormalizeScheduledTaskCompatibility(task)
	if normalized.Delivery != task.Delivery.Normalized() {
		t.Fatalf("opaque legacy target must retain its exact semantics: %+v", normalized.Delivery)
	}
}

func TestNormalizeScheduledTaskCompatibilityRecoversLastSessionFromSource(t *testing.T) {
	sourceSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	task := ScheduledTask{
		Delivery: DeliveryTarget{Mode: DeliveryModeLast},
		Source:   Source{Kind: SourceKindAgent, SessionKey: sourceSession},
	}

	normalized := NormalizeScheduledTaskCompatibility(task)
	if normalized.Delivery.SessionKey != sourceSession {
		t.Fatalf("last delivery session = %q, want %q", normalized.Delivery.SessionKey, sourceSession)
	}
}

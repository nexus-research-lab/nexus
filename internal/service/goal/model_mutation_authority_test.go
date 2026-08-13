// INPUT: DM/Room current Goal、持久 owner 与负责人身份。
// OUTPUT: 跨 round Goal-only authority 的负责人准入与协作者拒绝回归。
// POS: model_mutation_authority.go 的领域权限测试。
package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestCurrentModelMutationAuthorityUsesRoomLeadAndExactRevision(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  protocol.BuildRoomSharedSessionKey("owner-authority"),
		Objective:   "finish the Room work",
		CreatedBy:   "model",
		OwnerUserID: "owner-1",
		AgentID:     "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	created.Metadata[protocol.GoalMetadataObjectiveRevision] = int64(3)
	repo.goals[created.ID] = *created

	granted, err := service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-lead",
	)
	if err != nil {
		t.Fatal(err)
	}
	if granted.ID != created.ID || granted.ObjectiveRevision() != 3 {
		t.Fatalf("granted Goal = %#v, want exact current revision", granted)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-peer",
	); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("peer authority error = %v, want ErrGoalForbidden", err)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-2",
		"agent-lead",
	); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("foreign owner authority error = %v, want ErrGoalForbidden", err)
	}
}

func TestCurrentModelMutationAuthorityUsesDMAgentIdentity(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  protocol.BuildAgentSessionKey("agent-owner", protocol.SessionChannelWebSocketSegment, "dm", "chat-1", ""),
		Objective:   "finish the DM work",
		CreatedBy:   "model",
		OwnerUserID: "owner-1",
		AgentID:     "agent-owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-owner",
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-other",
	); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("other DM Agent error = %v, want ErrGoalForbidden", err)
	}
}

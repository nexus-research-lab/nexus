package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
)

type runtimeCommandInputAgentResolver struct {
	agent *protocol.Agent
}

func (r runtimeCommandInputAgentResolver) GetAgent(
	context.Context,
	string,
) (*protocol.Agent, error) {
	return r.agent, nil
}

func TestPrepareRuntimeCommandInputIsPrivateStableAndRoundScoped(t *testing.T) {
	t.Setenv(appfs.NexusStateRootEnvName, t.TempDir())
	firstPath, cleanup, err := prepareRuntimeCommandInput("owner-a", "round-a", "capability-a")
	if err != nil {
		t.Fatalf("prepareRuntimeCommandInput() error = %v", err)
	}
	defer cleanup()
	info, err := os.Stat(firstPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("input staging info = %+v err=%v", info, err)
	}
	if content, readErr := os.ReadFile(firstPath); readErr != nil || string(content) != "{}\n" {
		t.Fatalf("initial input staging = %q err=%v", content, readErr)
	}
	custom := []byte(`{"instruction":"date '+%Y-%m-%d'"}`)
	if err = os.WriteFile(firstPath, custom, 0o600); err != nil {
		t.Fatal(err)
	}
	reusedPath, reusedCleanup, err := prepareRuntimeCommandInput("owner-a", "round-a", "capability-a")
	if err != nil {
		t.Fatal(err)
	}
	defer reusedCleanup()
	if reusedPath != firstPath {
		t.Fatalf("same round path changed: first=%s reused=%s", firstPath, reusedPath)
	}
	if content, readErr := os.ReadFile(reusedPath); readErr != nil || string(content) != string(custom) {
		t.Fatalf("same round input was overwritten: %q err=%v", content, readErr)
	}
	secondPath, secondCleanup, err := prepareRuntimeCommandInput("owner-a", "round-b", "capability-a")
	if err != nil {
		t.Fatal(err)
	}
	defer secondCleanup()
	if secondPath == firstPath || filepath.Dir(secondPath) == filepath.Dir(firstPath) {
		t.Fatalf("different rounds reused staging directory: %s", firstPath)
	}
	staleDirectory := filepath.Join(filepath.Dir(filepath.Dir(firstPath)), "stale-round")
	if err = os.Mkdir(staleDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	staleAt := time.Now().Add(-runtimeCommandInputRetention - time.Hour)
	if err = os.Chtimes(staleDirectory, staleAt, staleAt); err != nil {
		t.Fatal(err)
	}
	_, thirdCleanup, err := prepareRuntimeCommandInput("owner-a", "round-c", "capability-a")
	if err != nil {
		t.Fatal(err)
	}
	defer thirdCleanup()
	if _, err = os.Stat(staleDirectory); !os.IsNotExist(err) {
		t.Fatalf("stale round staging was not reaped: %v", err)
	}
	cleanup()
	if _, err = os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("round cleanup left input staging: %v", err)
	}
}

func TestRuntimeCommandInputFollowsPhysicalRoundInsteadOfPreparationContext(t *testing.T) {
	t.Setenv(appfs.NexusStateRootEnvName, t.TempDir())
	const (
		agentID = "agent-input-lifecycle"
		ownerID = "owner-input-lifecycle"
		roundID = "round-input-lifecycle"
	)
	sessionKey := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"input-lifecycle",
		"",
	)
	registry := runtimecommand.NewRegistry(runtimeAutomationRoundResolver{
		sessionKey: sessionKey,
		roundID: roundID,
	})
	resources := runtimecommand.NewRoundResources()
	builder := newRuntimeCommandEnvironmentBuilder(
		config.Config{},
		registry,
		runtimeCommandInputAgentResolver{agent: &protocol.Agent{
			AgentID: agentID,
			OwnerUserID: ownerID,
		}},
		nil,
	)
	preparationContext, cancelPreparation := context.WithCancel(
		runtimectx.WithRuntimeRoundLease(context.Background(), sessionKey, roundID),
	)
	environment, err := builder(preparationContext, runtimecommand.RoundContext{
		SessionKey: sessionKey,
		RoundID: roundID,
		SourceContextType: "agent",
		SourceContextID: agentID,
		Receipts: runtimecommand.NewReceiptState(),
		Resources: resources,
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent: &protocol.Agent{AgentID: agentID, OwnerUserID: ownerID},
			ScopeSessionKey: sessionKey,
			RuntimeSessionKey: sessionKey,
			RootRoundID: roundID,
			SourceContextType: "agent",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inputPath := environment[protocol.NexusCommandInputPathEnvName]
	if inputPath == "" {
		t.Fatalf("runtime command environment = %#v", environment)
	}
	cancelPreparation()
	if _, err = os.Stat(inputPath); err != nil {
		t.Fatalf("preparation context cancellation removed live round input: %v", err)
	}
	resources.Close()
	if _, err = os.Stat(inputPath); !os.IsNotExist(err) {
		t.Fatalf("physical round cleanup left command input: %v", err)
	}
}

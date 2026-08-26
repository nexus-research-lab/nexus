// INPUT: host-verified runtime Actor identity and one sidecar epoch's opaque refs.
// OUTPUT: isolated per-physical-round target, observation, mutation, and artifact lifecycle.
// POS: cross-round reference and replay fence for Nexus Computer Use.
package computeruse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
)

type roundState struct {
	mu sync.Mutex

	key           string
	roundSegment  string
	ownerUserID   string
	workspacePath string
	client        RuntimeClient
	epoch         uint64
	sessionID     nexuscua.SessionID
	application   *nexuscua.DiscoveredApplication
	window        *nexuscua.WindowSummary
	observation   *nexuscua.WindowObservation
	actions       map[string]*roundAction
}

type roundAction struct {
	digest    string
	epoch     uint64
	handle    ActionHandle
	wait      time.Duration
	result    runtimecommand.Result
	completed bool
}

func (service *Service) getOrCreateRound(actor runtimecommand.Actor) (*roundState, error) {
	if actor.Round.Resources == nil {
		return nil, errors.New("Computer Use physical round has no resource owner")
	}
	key := computerRoundKey(actor)
	if key == "" {
		return nil, errors.New("Computer Use physical round identity is incomplete")
	}
	service.roundsMu.Lock()
	if state := service.rounds[key]; state != nil {
		service.roundsMu.Unlock()
		return state, nil
	}
	state := &roundState{
		key: key, roundSegment: computerRoundSegment(key), ownerUserID: actor.OwnerUserID,
		workspacePath: actor.WorkspacePath, actions: make(map[string]*roundAction),
	}
	service.rounds[key] = state
	service.roundsMu.Unlock()
	actor.Round.Resources.Add(func() { service.closeRound(key) })
	return state, nil
}

func (service *Service) findRound(actor runtimecommand.Actor) *roundState {
	key := computerRoundKey(actor)
	service.roundsMu.Lock()
	defer service.roundsMu.Unlock()
	return service.rounds[key]
}

func (service *Service) closeRound(key string) {
	service.roundsMu.Lock()
	state := service.rounds[key]
	delete(service.rounds, key)
	service.roundsMu.Unlock()
	if state == nil {
		return
	}
	state.mu.Lock()
	client := state.client
	sessionID := state.sessionID
	state.sessionID = ""
	state.client = nil
	state.application = nil
	state.window = nil
	state.observation = nil
	state.actions = nil
	state.mu.Unlock()
	if client != nil && sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = client.CloseSession(ctx, sessionID, 2*time.Second)
		cancel()
	}
	removeRoundArtifacts(state.workspacePath, state.roundSegment)
}

func (service *Service) closeAllRounds() {
	service.roundsMu.Lock()
	keys := make([]string, 0, len(service.rounds))
	for key := range service.rounds {
		keys = append(keys, key)
	}
	service.roundsMu.Unlock()
	for _, key := range keys {
		service.closeRound(key)
	}
}

func computerRoundKey(actor runtimecommand.Actor) string {
	parts := []string{
		strings.TrimSpace(actor.OwnerUserID), strings.TrimSpace(actor.AgentID),
		strings.TrimSpace(actor.LeaseSessionKey), strings.TrimSpace(actor.LeaseRoundID),
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, "\x00")
}

func computerRoundSegment(key string) string {
	digest := sha256.Sum256([]byte(key))
	return "round-" + hex.EncodeToString(digest[:12])
}

func actionDigest(operation string, input map[string]any) (string, error) {
	payload, err := json.Marshal(struct {
		Operation string         `json:"operation"`
		Input     map[string]any `json:"input"`
	}{operation, input})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

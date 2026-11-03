// INPUT: Exact current Execution and a declared Room target.
// OUTPUT: Read-only pause admission for declarative scheduling.
// POS: Room queue gate adapter; assignment membership is still checked by orchestration.
package realtime

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) AutomaticDispatchAllowed(ctx context.Context, e protocol.Execution, target string) (bool, error) {
	value, err := s.rooms.GetConversationContextForSystem(ctx, e.ConversationID)
	if err != nil {
		return false, err
	}
	if value.Room.OwnerUserID != e.OwnerUserID || value.Room.ID != e.RoomID {
		return false, errors.New("dispatch scope changed")
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, value)
	if err != nil {
		return false, err
	}
	location, ok := locations[target]
	if !ok {
		return false, errors.New("dispatch target left Room")
	}
	paused, err := s.InputQueue.DispatchPaused(location.Location)
	return !paused, err
}

func (s *Service) ContinuationPauseRevision(ctx context.Context, e protocol.Execution, target string) (int64, error) {
	value, err := s.rooms.GetConversationContextForSystem(ctx, e.ConversationID)
	if err != nil {
		return 0, err
	}
	if value == nil || value.Room.OwnerUserID != e.OwnerUserID || value.Room.ID != e.RoomID {
		return 0, errors.New("continuation scope changed")
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, value)
	if err != nil {
		return 0, err
	}
	location, ok := locations[target]
	if !ok {
		return 0, errors.New("continuation target left Room")
	}
	return s.InputQueue.ContinuationPauseRevision(location.Location)
}

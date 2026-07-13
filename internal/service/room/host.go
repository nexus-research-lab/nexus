package room

import (
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) normalizeRoomHostSettings(
	roomType string,
	memberAgentIDs []string,
	hostAgentID string,
	hostAutoReplyEnabled bool,
) (string, bool, error) {
	hostAgentID = strings.TrimSpace(hostAgentID)
	if roomType == protocol.RoomTypeDM {
		if hostAgentID != "" || hostAutoReplyEnabled {
			return "", false, errors.New("DM room 不支持设置群主")
		}
		return "", false, nil
	}
	if hostAgentID == "" {
		if hostAutoReplyEnabled {
			return "", false, errors.New("启用群主接管时必须设置群主")
		}
		return "", false, nil
	}
	if !slices.Contains(memberAgentIDs, hostAgentID) {
		return "", false, errors.New("群主必须是当前 room 成员")
	}
	return hostAgentID, hostAutoReplyEnabled, nil
}

func (s *Service) normalizeRoomHostSettingsPatch(
	existing *protocol.RoomAggregate,
	hostAgentIDPatch *string,
	hostAutoReplyEnabledPatch *bool,
) (string, bool, error) {
	if existing == nil {
		return "", false, ErrRoomNotFound
	}
	memberAgentIDs := roomAgentMemberIDs(existing.Members)
	hostAgentID := strings.TrimSpace(existing.Room.HostAgentID)
	if hostAgentIDPatch != nil {
		hostAgentID = strings.TrimSpace(*hostAgentIDPatch)
	}
	hostAutoReplyEnabled := existing.Room.HostAutoReplyEnabled
	if hostAutoReplyEnabledPatch != nil {
		hostAutoReplyEnabled = *hostAutoReplyEnabledPatch
	}
	if hostAgentID == "" {
		hostAutoReplyEnabled = false
	}
	return s.normalizeRoomHostSettings(existing.Room.RoomType, memberAgentIDs, hostAgentID, hostAutoReplyEnabled)
}

func roomAgentMemberIDs(members []protocol.MemberRecord) []string {
	result := make([]string, 0, len(members))
	for _, member := range members {
		if member.MemberType != protocol.MemberTypeAgent {
			continue
		}
		agentID := strings.TrimSpace(member.MemberAgentID)
		if agentID == "" {
			continue
		}
		result = append(result, agentID)
	}
	return result
}

func (s *Service) normalizeRoomType(roomType string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(roomType))
	if normalized == "" {
		normalized = protocol.RoomTypeGroup
	}
	switch normalized {
	case protocol.RoomTypeDM, protocol.RoomTypeGroup:
		return normalized, nil
	default:
		return "", errors.New("room_type 仅支持 room 或 dm")
	}
}

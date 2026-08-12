// INPUT: owner 作用域、发起 Agent、目标 Agent、别名与直聊 Room id。
// OUTPUT: 仅普通同 owner Agent 可用的双向好友关系与通讯录查询。
// POS: Agent 联系人身份校验和业务不变量边界。
package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

const agentContactAliasMaxRunes = 128

var (
	// ErrAgentContactNotFound 表示目标不在当前 Agent 通讯录中。
	ErrAgentContactNotFound = errors.New("agent contact not found")
	// ErrAgentContactUnsupported 表示主智能体或跨 owner 关系不属于首版通讯录。
	ErrAgentContactUnsupported = errors.New("agent contact unsupported")
)

// ListAgentContacts 返回指定普通 Agent 的通讯录。
func (s *Service) ListAgentContacts(ctx context.Context, ownerAgentID string) ([]protocol.AgentContact, error) {
	ownerAgent, err := s.requireContactAgent(ctx, ownerAgentID)
	if err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	items, err := s.repository.ListAgentContacts(ctx, ownerUserID, ownerAgent.AgentID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Avatar = resolveAgentAvatar(
			items[index].Avatar, items[index].ContactAgentID, false,
		)
	}
	return items, nil
}

// GetAgentContact 返回指定普通 Agent 的一个联系人。
func (s *Service) GetAgentContact(
	ctx context.Context,
	ownerAgentID string,
	contactAgentID string,
) (*protocol.AgentContact, error) {
	ownerAgent, err := s.requireContactAgent(ctx, ownerAgentID)
	if err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	item, err := s.repository.GetAgentContact(
		ctx, ownerUserID, ownerAgent.AgentID, strings.TrimSpace(contactAgentID),
	)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrAgentContactNotFound
	}
	item.Avatar = resolveAgentAvatar(item.Avatar, item.ContactAgentID, false)
	return item, nil
}

// AddAgentContact 创建双向好友关系；alias 只属于发起方视图。
func (s *Service) AddAgentContact(
	ctx context.Context,
	ownerAgentID string,
	request protocol.CreateAgentContactRequest,
) (*protocol.AgentContact, error) {
	ownerAgentID = strings.TrimSpace(ownerAgentID)
	contactAgentID := strings.TrimSpace(request.ContactAgentID)
	if ownerAgentID == "" || contactAgentID == "" {
		return nil, errors.New("owner_agent_id 和 contact_agent_id 不能为空")
	}
	if ownerAgentID == contactAgentID {
		return nil, errors.New("Agent 不能把自己添加为联系人")
	}
	alias := strings.TrimSpace(request.Alias)
	if utf8.RuneCountInString(alias) > agentContactAliasMaxRunes {
		return nil, fmt.Errorf("联系人别名不能超过 %d 个字符", agentContactAliasMaxRunes)
	}
	agents, err := s.GetAgentsByIDs(ctx, []string{ownerAgentID, contactAgentID})
	if err != nil {
		return nil, err
	}
	if len(agents) != 2 {
		return nil, ErrAgentNotFound
	}
	for _, item := range agents {
		if item.IsMain {
			return nil, fmt.Errorf("%w: 主智能体只属于控制面", ErrAgentContactUnsupported)
		}
	}
	if err = s.repository.UpsertAgentContactPair(ctx, agentrepo.ContactPairRecord{
		OwnerContactID: protocol.NewContactID(),
		PeerContactID:  protocol.NewContactID(),
		OwnerAgentID:   ownerAgentID,
		ContactAgentID: contactAgentID,
		Alias:          alias,
	}); err != nil {
		return nil, err
	}
	return s.GetAgentContact(ctx, ownerAgentID, contactAgentID)
}

// DeleteAgentContact 删除双向好友关系，保留历史消息与 Room。
func (s *Service) DeleteAgentContact(ctx context.Context, ownerAgentID string, contactAgentID string) error {
	if _, err := s.GetAgentContact(ctx, ownerAgentID, contactAgentID); err != nil {
		return err
	}
	return s.repository.DeleteAgentContactPair(
		ctx, strings.TrimSpace(ownerAgentID), strings.TrimSpace(contactAgentID),
	)
}

// SetAgentContactDirectRoom 绑定联系人复用的双人私域 Room。
func (s *Service) SetAgentContactDirectRoom(
	ctx context.Context,
	ownerAgentID string,
	contactAgentID string,
	roomID string,
) error {
	if _, err := s.GetAgentContact(ctx, ownerAgentID, contactAgentID); err != nil {
		return err
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return errors.New("direct room_id 不能为空")
	}
	if err := s.repository.SetAgentContactDirectRoom(
		ctx, strings.TrimSpace(ownerAgentID), strings.TrimSpace(contactAgentID), roomID,
	); errors.Is(err, sql.ErrNoRows) {
		return ErrAgentContactNotFound
	} else {
		return err
	}
}

func (s *Service) requireContactAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	item, err := s.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if item.IsMain {
		return nil, fmt.Errorf("%w: 主智能体只属于控制面", ErrAgentContactUnsupported)
	}
	return item, nil
}

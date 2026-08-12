// INPUT: HTTP/配置控制面或平台通讯发起的 Room 创建、资料、成员与对话变更意图。
// OUTPUT: 隔离内部联系人通道标记、带可选 configuration_version CAS 的 Room 请求协议。
// POS: Room 写入命令、内部用途与乐观并发令牌的跨边界真相源。
package protocol

// CreateRoomRequest 表示创建房间请求。
type CreateRoomRequest struct {
	AgentIDs               []string `json:"agent_ids"`
	Name                   string   `json:"name,omitempty"`
	Description            string   `json:"description,omitempty"`
	Title                  string   `json:"title,omitempty"`
	Avatar                 string   `json:"avatar,omitempty"`
	SkillNames             []string `json:"skill_names,omitempty"`
	HostAgentID            string   `json:"host_agent_id,omitempty"`
	HostAutoReplyEnabled   bool     `json:"host_auto_reply_enabled,omitempty"`
	PrivateMessagesEnabled bool     `json:"private_messages_enabled,omitempty"`
	// IsContactChannel 只允许平台通讯服务设置，HTTP JSON 不能创建内部通道。
	IsContactChannel bool `json:"-"`
}

// UpdateRoomRequest 表示更新房间请求。
type UpdateRoomRequest struct {
	Name                   *string   `json:"name,omitempty"`
	Description            *string   `json:"description,omitempty"`
	Title                  *string   `json:"title,omitempty"`
	Avatar                 *string   `json:"avatar,omitempty"`
	SkillNames             *[]string `json:"skill_names,omitempty"`
	HostAgentID            *string   `json:"host_agent_id,omitempty"`
	HostAutoReplyEnabled   *bool     `json:"host_auto_reply_enabled,omitempty"`
	PrivateMessagesEnabled *bool     `json:"private_messages_enabled,omitempty"`
	// ExpectedConfigurationVersion 可选；设置后 Room 更新使用资源级 CAS。
	ExpectedConfigurationVersion *int64 `json:"expected_configuration_version,omitempty"`
}

// AddRoomMemberRequest 表示追加成员请求。
type AddRoomMemberRequest struct {
	AgentID string `json:"agent_id"`
}

// SetRoomMemberParticipationRequest 表示暂停或恢复 Room 成员参与。
type SetRoomMemberParticipationRequest struct {
	Paused bool `json:"paused"`
}

// CreateConversationRequest 表示创建话题请求。
type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
}

// UpdateConversationRequest 表示更新话题请求。
type UpdateConversationRequest struct {
	Title string `json:"title,omitempty"`
}

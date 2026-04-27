package room

// AgentRuntimeRef 表示 Room 创建成员会话时所需的 Agent 运行时信息。
type AgentRuntimeRef struct {
	AgentID     string
	Name        string
	DisplayName string
	RuntimeID   string
	Status      string
}

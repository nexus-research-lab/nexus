package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// randomIDSuffix 生成 12 字节随机 hex；随机源不可用时退化为纳秒时间戳。
func randomIDSuffix() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

// NewRoundID 生成根业务轮次 id，只允许 Nexus 后端调用。
func NewRoundID() string {
	return "round_" + randomIDSuffix()
}

// NewUserMessageID 生成 durable 用户消息 id。
func NewUserMessageID() string {
	return "msg_user_" + randomIDSuffix()
}

// NewAgentRoundID 生成 agent slot 执行轮次 id，与 round_id 相互独立。
func NewAgentRoundID() string {
	return "agent_round_" + randomIDSuffix()
}

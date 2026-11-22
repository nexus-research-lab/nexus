package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// MaxClientMessageIDBytes 限制客户端关联 ID，避免历史和队列索引被异常输入撑大。
const MaxClientMessageIDBytes = 512

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

// InputSeqField 是用户输入行上的受理游标字段：chat_ack、实时 durable 行与历史读取对同一输入给出同一个值。
const InputSeqField = "input_seq"

// lastInputSeq 是本进程最近一次发出的受理游标。
var lastInputSeq atomic.Int64

// NewInputSeq 生成用户输入行的受理游标，只允许 Nexus 后端在受理输入时调用一次。
//
// 取值以 UnixMicro 为基准并在进程内严格递增：同一微秒内的并发受理依次加一，
// 宿主时钟回拨时从上一个值继续，不会发出重复或倒退的游标；数值始终是 JS 安全整数。
// 进程重启后分配器从当前时钟重新起步，跨进程的先后因此取决于宿主时钟，不读取已落盘的游标。
func NewInputSeq() int64 {
	for {
		last := lastInputSeq.Load()
		next := max(time.Now().UnixMicro(), last+1)
		if lastInputSeq.CompareAndSwap(last, next) {
			return next
		}
	}
}

// NewAssistantMessageID 生成 assistant 消息 id。
func NewAssistantMessageID() string {
	return "msg_assistant_" + randomIDSuffix()
}

// NewAgentRoundID 生成 agent slot 执行轮次 id，与 round_id 相互独立。
func NewAgentRoundID() string {
	return "agent_round_" + randomIDSuffix()
}

// NewContactID 生成 Agent 通讯录关系 id。
func NewContactID() string {
	return "contact_" + randomIDSuffix()
}

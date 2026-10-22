// INPUT: 一轮内按序到达的 SDK 消息。
// OUTPUT: 本轮从提交输入到模型首包、首个思考块、首个回答块的耗时日志。
// POS: 宿主侧启动只占亚秒级，用户感知的“启动慢”主要落在模型首包与思考阶段；这里把这几段量出来。
package runtimehost

import (
	"log/slog"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
)

// roundLatency 只看主线程（非子智能体）第一个模型请求；同一 round 内只记一次。
type roundLatency struct {
	start    time.Time
	logger   *slog.Logger
	marks    map[string]time.Duration
	reported bool
}

func newRoundLatency(logger *slog.Logger) *roundLatency {
	return &roundLatency{start: time.Now(), logger: logger, marks: make(map[string]time.Duration, 4)}
}

// Observe 记录里程碑：system init、message_start、首个 thinking 块与首个回答（text/tool_use）块。
func (l *roundLatency) Observe(incoming sdkprotocol.ReceivedMessage) {
	if l == nil || l.reported || incoming.ParentToolUseID != nil {
		return
	}
	mark := roundLatencyMark(incoming)
	if mark == "" {
		return
	}
	if _, seen := l.marks[mark]; !seen {
		l.marks[mark] = time.Since(l.start)
	}
	if mark == "first_answer_ms" {
		l.Report()
	}
}

// Report 输出已观察到的里程碑；round 未产出回答就结束时也会带出已有阶段。
func (l *roundLatency) Report() {
	if l == nil || l.reported || l.logger == nil || len(l.marks) == 0 {
		return
	}
	l.reported = true
	fields := make([]any, 0, 2*len(l.marks)+2)
	for _, key := range []string{"system_init_ms", "message_start_ms", "first_thinking_ms", "first_answer_ms"} {
		if elapsed, ok := l.marks[key]; ok {
			fields = append(fields, key, elapsed.Milliseconds())
		}
	}
	thinking, thought := l.marks["first_thinking_ms"]
	answer, answered := l.marks["first_answer_ms"]
	if thought && answered {
		fields = append(fields, "thinking_ms", (answer - thinking).Milliseconds())
	}
	l.logger.Info("Agent round 首包耗时", fields...)
}

func roundLatencyMark(incoming sdkprotocol.ReceivedMessage) string {
	if incoming.Type == sdkprotocol.MessageTypeSystem && incoming.Subtype == "init" {
		return "system_init_ms"
	}
	if incoming.Type != sdkprotocol.MessageTypeStreamEvent || incoming.Stream == nil {
		return ""
	}
	payload, ok := incoming.Stream.Event.(map[string]any)
	if !ok {
		payload = incoming.Stream.Data
	}
	switch textutil.AnyString(payload["type"]) {
	case "message_start":
		return "message_start_ms"
	case "content_block_start":
		block, _ := payload["content_block"].(map[string]any)
		switch textutil.AnyString(block["type"]) {
		case "thinking", "redacted_thinking":
			return "first_thinking_ms"
		case "":
			return ""
		}
		return "first_answer_ms"
	}
	return ""
}

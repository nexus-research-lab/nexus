// INPUT: 用户级 Echo 开关与产品内部策略。
// OUTPUT: 带 Preferences revision 的 Echo 设置与主窗口 Echo 的时段策略。
// POS: Echo 的领域合同；不依赖 HTTP、数据库或 runtime 实现。
package echo

import (
	"strings"
	"time"
)

// Settings 是用户可见的 Echo 全局开关。
type Settings struct {
	Enabled bool  `json:"enabled"`
	Version int64 `json:"version"`
}

// Policy 是 Echo 的内部策略：只在活跃时段内、对话安静一段时间后出现。
// 不设冷却与每日上限：每条 Echo 都锚定一轮新的用户输入，用户开口即取消，节奏天然有界。
type Policy struct {
	Enabled          bool
	Timezone         string
	ActiveStart      string
	ActiveEnd        string
	IdleDelaySeconds int
}

// DefaultPolicy 返回关闭状态下的产品默认值。
func DefaultPolicy(timezone string) Policy {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	return Policy{
		Timezone:         timezone,
		ActiveStart:      "09:00",
		ActiveEnd:        "22:00",
		IdleDelaySeconds: 6 * 60 * 60,
	}
}

// ParseClock 把 HH:mm 转成当天分钟数。
func ParseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

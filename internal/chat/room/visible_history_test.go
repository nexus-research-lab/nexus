// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package room

import "github.com/nexus-research-lab/nexus/internal/protocol"

func buildHistoryLines(history []protocol.Message, agentNameByID map[string]string) []string {
	if len(history) == 0 {
		return nil
	}
	formatted := make([]string, 0, len(history))
	for _, message := range history {
		line := formatHistoryLine(message, agentNameByID)
		if line != "" {
			formatted = append(formatted, line)
		}
	}
	return formatted
}

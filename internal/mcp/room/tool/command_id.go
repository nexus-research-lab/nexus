// INPUT: Room MCP CallContext、server-bound round identity 与规范化工具参数。
// OUTPUT: transport/model retry 间稳定、跨 source round 隔离的宿主命令 ID。
// POS: Room 副作用工具共享的幂等身份生成边界。
package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func roomCommandID(
	sctx contract.ServerContext,
	callContext *sdktool.CallContext,
	toolName string,
	input map[string]any,
) (string, error) {
	if callContext != nil {
		if toolUseID := strings.TrimSpace(callContext.ToolUseID); toolUseID != "" {
			return toolUseID, nil
		}
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("canonicalize Room tool input: %w", err)
	}
	parts := []string{
		strings.TrimSpace(sctx.CurrentSessionKey),
		strings.TrimSpace(sctx.ConversationID),
		strings.TrimSpace(sctx.CurrentAgentID),
		strings.TrimSpace(sctx.CurrentRoundID),
		strings.TrimSpace(toolName),
		string(canonical),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "room-" + hex.EncodeToString(digest[:]), nil
}

// INPUT: Completed file paths and immutable host runtime identity.
// OUTPUT: A verified delivery receipt, projected into the calling Agent's durable reply.
// POS: nexus.deliver_files transport; never accepts model-supplied owner, Agent or round identity.
package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

type Service interface {
	ValidateDeliverables(context.Context, string, []string) ([]string, error)
}

type Context struct {
	OwnerUserID  string
	AgentID      string
	AgentRoundID string
}

func BuildTools(service Service, scope Context) []sdktool.Tool {
	return []sdktool.Tool{{
		Name:        "deliver_files",
		Description: "完成文件交付后调用，供 Nexus 在当前智能体回复下显示文件卡片。适用于 Write、Skill、Python、Shell 或任意脚本生成的最终产物；传入当前 workspace 的实际文件路径，一次最多 32 个。只登记自己本轮完成的交付，不登记参考资料、缓存、临时脚本或其他智能体的文件。正文引用不能替代此调用。先完成生成和验证，再调用；失败时修正路径后重试。",
		SearchHint:  "deliver generated files artifacts skill script pdf pptx xlsx 文件 产物 交付",
		AlwaysLoad:  true,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"paths": map[string]any{"type": "array", "minItems": 1, "maxItems": 32, "items": map[string]any{"type": "string", "minLength": 1}}},
			"required":   []string{"paths"},
		},
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			paths, err := parsePaths(input)
			if err != nil {
				return failure(err), nil
			}
			if service == nil || scope.OwnerUserID == "" || scope.AgentID == "" || scope.AgentRoundID == "" {
				return failure(errors.New("file delivery requires an active Agent round")), nil
			}
			ctx = authctx.WithPrincipal(ctx, &authctx.Principal{UserID: scope.OwnerUserID, Role: authctx.RoleOwner, AuthMethod: "artifact_mcp_runtime"})
			paths, err = service.ValidateDeliverables(ctx, scope.AgentID, paths)
			if err != nil {
				return failure(err), nil
			}
			payload := map[string]any{"kind": "file_delivery", "version": 1, "agent_id": scope.AgentID, "agent_round_id": scope.AgentRoundID, "paths": paths}
			data, _ := json.Marshal(payload)
			return sdktool.ToolResult{Content: []map[string]any{{"type": "text", "text": string(data)}}, StructuredContent: payload}, nil
		},
	}}
}

func parsePaths(input map[string]any) ([]string, error) {
	if len(input) != 1 {
		return nil, errors.New("only paths may be supplied")
	}
	data, err := json.Marshal(input["paths"])
	if err != nil {
		return nil, errors.New("paths must be strings")
	}
	var paths []string
	if json.Unmarshal(data, &paths) != nil || len(paths) == 0 || len(paths) > 32 {
		return nil, errors.New("supply between 1 and 32 file paths")
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" || len(path) > 4096 {
			return nil, errors.New("invalid file path")
		}
	}
	return paths, nil
}

func failure(err error) sdktool.ToolResult {
	return sdktool.ToolResult{IsError: true, Content: []map[string]any{{"type": "text", "text": err.Error()}}}
}

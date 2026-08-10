// INPUT: runtime permission 固化的人工 allow 与高风险工具叶名称。
// OUTPUT: 按工具域路由到 configuration 或 Connector durable approval recorder。
// POS: 应用层人工批准组合器；runtime 不依赖任何业务 service。
package server

import (
	"context"
	"errors"
	"strings"

	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

type humanToolApprovalRouter struct {
	configuration permissionctx.HumanToolApprovalRecorder
	connector     permissionctx.HumanToolApprovalRecorder
}

func (r humanToolApprovalRouter) RecordHumanToolApproval(
	ctx context.Context,
	approval permissionctx.HumanToolApproval,
) error {
	switch {
	case connectorsvc.IsConnectorAuthorizationStartTool(approval.ToolName):
		if r.connector == nil {
			return errors.New("Connector 人工授权记录器未装配")
		}
		return r.connector.RecordHumanToolApproval(ctx, approval)
	case matchesApprovalToolLeaf(approval.ToolName, "apply_nexus_configuration_change"):
		if r.configuration == nil {
			return errors.New("配置人工授权记录器未装配")
		}
		return r.configuration.RecordHumanToolApproval(ctx, approval)
	default:
		return errors.New("不支持记录该工具的人工批准")
	}
}

func matchesApprovalToolLeaf(toolName string, leaf string) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == leaf {
		return true
	}
	for _, separator := range []string{"__", ".", "/"} {
		if strings.HasSuffix(toolName, separator+leaf) {
			return true
		}
	}
	return false
}

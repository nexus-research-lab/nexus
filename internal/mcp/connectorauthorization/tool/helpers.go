// INPUT: MCP 参数和 service 公开投影。
// OUTPUT: Authorization request/ref 投影。
// POS: Nexus Connector 授权工具的无业务逻辑适配层。
package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

func startRequest(args map[string]any) connectorsvc.AuthorizationStartRequest {
	return connectorsvc.AuthorizationStartRequest{
		RequestID:   sdktool.StringArg(args, "request_id"),
		ConnectorID: sdktool.StringArg(args, "connector_id"),
		Method:      sdktool.StringArg(args, "method"),
		DeviceMode: connectorsvc.DeviceAuthStartMode(
			sdktool.StringArg(args, "device_mode"),
		),
		Extras: sdktool.StringMapArg(args, "extras"),
	}
}

func flowRef(args map[string]any) connectorsvc.AuthorizationFlowRef {
	return connectorsvc.AuthorizationFlowRef{
		FlowID:      sdktool.StringArg(args, "flow_id"),
		ConnectorID: sdktool.StringArg(args, "connector_id"),
	}
}

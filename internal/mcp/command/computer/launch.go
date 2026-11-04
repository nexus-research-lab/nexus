// INPUT: optional application-name query, model launch reference and host permission channel.
// OUTPUT: filtered launch catalog and approved, reconciled launch operations.
// POS: model-facing lifecycle adapter; application identity comes from runtime discovery.
package computer

import (
	"context"
	"errors"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

func (r registry) listLaunchableApplications() command.Operation {
	return command.Operation{Name: "list_launchable_applications", Description: "Find installed launchable applications and short-lived launch_ref values. When the app is known, pass query (name or application id) to avoid paging through the full catalog. This does not launch or grant authority.",
		InputSchema: applicationQuerySchema(), ReadOnly: true,
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			query, err := parseApplicationQuery(input)
			if err != nil {
				return rejected(err), nil
			}
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			result, err := round.ListLaunchableApplications(ctx, query)
			if err != nil {
				return rejected(err), nil
			}
			return structured(result), nil
		},
	}
}

func (r registry) launchApplication() command.Operation {
	return command.Operation{Name: "launch_application", Description: "Approve and launch one catalog application. Reuse the original request_id to reconcile unknown delivery; select_target separately grants input authority.",
		InputSchema: objectSchema(map[string]any{"launch_ref": stringProperty("Fresh reference from list_launchable_applications.", 1, 256), "allow_foreground_activation": map[string]any{"type": "boolean"}}, "launch_ref"),
		Idempotent:  true,
		ContextHandler: func(ctx context.Context, input map[string]any, call *command.CallContext) (command.Result, error) {
			if call == nil || call.RequestID == "" {
				return rejected(errors.New("launch_application 需要宿主 request_id")), nil
			}
			var parsed nexuscua.LaunchApplicationInput
			if err := decodeInput(input, &parsed); err != nil {
				return rejected(err), nil
			}
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			result, err := round.LaunchApplication(ctx, call.RequestID, parsed, func(app nexuscua.LaunchableApplication) error {
				if r.permissions == nil || r.sessionKey == "" {
					return errors.New("launch_application 缺少宿主授权通道")
				}
				decision, _, err := r.permissions.RequestPermissionWithID(ctx, r.sessionKey, sdkpermission.Request{
					ToolName: "nexus_computer_use_launch", Title: "启动桌面应用", DisplayName: "Nexus Computer Use",
					Input:       map[string]any{"application": app, "allow_foreground_activation": parsed.AllowForegroundActivation},
					Description: "仅启动所列应用；后续操作需要 select_target 授权。",
				})
				if err != nil {
					return err
				}
				if decision.Behavior != sdkpermission.BehaviorAllow {
					return errors.New("用户未批准启动应用")
				}
				return nil
			})
			if err != nil {
				return rejected(err), nil
			}
			return applied(result), nil
		},
	}
}

// INPUT: trusted round owner/identity and the host-owned Computer Use service.
// OUTPUT: bounded computer operations with screenshot defaults and normalized key input for nexus.command.
// POS: the only model-facing adapter for Nexus Computer Use.
package computer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/service/computeruse"
)

// Service is the narrow host lifecycle surface needed by this adapter.
type Service interface {
	Round(context.Context, string, string) (*computeruse.Round, error)
	CloseRound(context.Context, string, string) error
}

// PermissionRequester is the host-owned runtime permission boundary. The
// Computer Use driver never decides whether a mutation is authorized.
type PermissionRequester interface {
	RequestPermissionWithID(context.Context, string, sdkpermission.Request) (sdkpermission.Decision, string, error)
}

type registry struct {
	service     Service
	permissions PermissionRequester
	owner       string
	roundID     string
	sessionKey  string
}

// BuildAll returns the stable Computer Use operation directory for one
// physical Nexus round. It is retained as a narrow compatibility helper for
// callers that only need read operations; mutations fail closed without the
// host permission channel.
func BuildAll(service Service, owner, roundID string) []command.Operation {
	return buildAll(service, owner, roundID, "", nil)
}

// BuildAllWithPermissions binds the current runtime session to the host-owned
// permission context before exposing desktop mutations to the model.
func BuildAllWithPermissions(service Service, owner, roundID, sessionKey string, permissions PermissionRequester) []command.Operation {
	return buildAll(service, owner, roundID, sessionKey, permissions)
}

func buildAll(service Service, owner, roundID, sessionKey string, permissions PermissionRequester) []command.Operation {
	r := registry{
		service: service, permissions: permissions,
		owner: strings.TrimSpace(owner), roundID: strings.TrimSpace(roundID),
		sessionKey: strings.TrimSpace(sessionKey),
	}
	return []command.Operation{
		r.getComputer(),
		r.listApplications(),
		r.listLaunchableApplications(),
		r.launchApplication(),
		r.selectTarget(),
		r.listWindows(),
		r.observe(),
		r.performAction(),
		r.verifyState(),
		r.closeTarget(),
	}
}

func (r registry) listWindows() command.Operation {
	return command.Operation{
		Name:        "list_windows",
		Description: "List the selected application's exact top-level windows and stable titles before observe; use window_ref or title as observe.window_selector when more than one window exists.",
		SearchHint:  "computer list windows target window title 窗口 标题",
		InputSchema: objectSchema(nil),
		ReadOnly:    true,
		ContextHandler: func(ctx context.Context, _ map[string]any, _ *command.CallContext) (command.Result, error) {
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.ListWindows(ctx)
			if err != nil {
				return rejected(err), nil
			}
			return structured(value), nil
		},
	}
}

func (r registry) round(ctx context.Context) (*computeruse.Round, error) {
	if r.service == nil {
		return nil, errors.New("Computer Use service 尚未装配")
	}
	if r.owner == "" || r.roundID == "" {
		return nil, errors.New("Computer Use round identity 无效")
	}
	return r.service.Round(ctx, r.owner, r.roundID)
}

func (r registry) getComputer() command.Operation {
	return command.Operation{
		Name:        "get_computer",
		Description: "Read Computer Use protocol capabilities and current host permission state. The sidecar is started only when this round first uses Computer Use.",
		SearchHint:  "computer use capabilities permissions macOS desktop 电脑 屏幕 辅助功能 权限",
		InputSchema: objectSchema(nil),
		ReadOnly:    true,
		ContextHandler: func(ctx context.Context, _ map[string]any, _ *command.CallContext) (command.Result, error) {
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.GetComputer(ctx)
			if err != nil {
				return rejected(err), nil
			}
			if limits, ok := value["action_limits"].(nexuscua.ActionLimits); ok {
				value["action_schema"] = boundedActionSchema(limits)
			}
			if actions, ok := value["actions"].([]nexuscua.ActionKind); ok {
				for _, action := range actions {
					if action == nexuscua.ActionSetWindowMinimized {
						value["action_aliases"] = map[string]string{"restore_window": "set_window_minimized"}
						value["action_names_hint"] = "actions lists authorization kinds. perform_action uses action_schema names: restore_window restores a window using the set_window_minimized grant."
					}
				}
			}
			return structured(value), nil
		},
	}
}

func (r registry) listApplications() command.Operation {
	return command.Operation{
		Name:        "list_applications",
		Description: "Discover currently available desktop applications and their opaque discovery references. Discovery references expire and must not be persisted by the Agent.",
		SearchHint:  "computer applications desktop app list 应用 列表 macOS",
		InputSchema: objectSchema(nil),
		ReadOnly:    true,
		ContextHandler: func(ctx context.Context, _ map[string]any, _ *command.CallContext) (command.Result, error) {
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.ListApplications(ctx)
			if err != nil {
				return rejected(err), nil
			}
			return structured(value), nil
		},
	}
}

func (r registry) selectTarget() command.Operation {
	return command.Operation{
		Name:        "select_target",
		Description: "Select one discovered application by exact name or stable application id and open a bounded CUA session for this round.",
		SearchHint:  "computer select target app application session 选择目标 应用 会话",
		InputSchema: objectSchema(map[string]any{
			"selector":               stringProperty("Exact application name or stable application id returned by list_applications.", 1, 256),
			"allowed_actions":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Requested authorization kinds from get_computer.actions; restore_window requires the set_window_minimized grant. Omitted means all exposed actions shown for approval."},
			"allow_foreground_input": map[string]any{"type": "boolean", "description": "Explicit foreground-input authority; defaults to false."},
			"ttl_seconds":            map[string]any{"type": "integer", "minimum": 1, "description": "Upper bound, capped by the round and runtime limits."},
		}, "selector"),
		Idempotent: true,
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			var parsed struct {
				Selector string `json:"selector"`
				computeruse.TargetOptions
			}
			if err := decodeInput(input, &parsed); err != nil {
				return rejected(err), nil
			}
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.SelectTarget(ctx, parsed.Selector, parsed.TargetOptions, func(grant computeruse.TargetGrant) error { return r.requestTargetApproval(ctx, grant) })
			if err != nil {
				return rejected(err), nil
			}
			return applied(value), nil
		},
	}
}

func (r registry) observe() command.Operation {
	return command.Operation{
		Name:        "observe",
		Description: "Capture a fresh observation for the selected window; screenshots default to true. Metadata-only recovery requires BOTH include_screenshot=false and accessibility=disabled. Use fresh observation_id and element_ref values only before the next mutation.",
		SearchHint:  "computer observe screenshot accessibility window observation 观察 截图 无障碍窗口",
		InputSchema: observeSchema(),
		ReadOnly:    true,
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			parsed := observeInput{IncludeScreenshot: true}
			if err := decodeInput(input, &parsed); err != nil {
				return rejected(err), nil
			}
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.Observe(ctx, computeruse.ObserveInput{
				WindowSelector:    parsed.WindowSelector,
				IncludeScreenshot: parsed.IncludeScreenshot,
				Accessibility:     parsed.Accessibility,
				ApplicationScene:  parsed.ApplicationScene,
			})
			if err != nil {
				return rejected(err), nil
			}
			data := cloneMap(value.Data)
			if value.ImageData != "" {
				data["image_data"] = value.ImageData
				data["image_mime"] = value.ImageMIME
			}
			return structured(data), nil
		},
	}
}

func (r registry) performAction() command.Operation {
	return command.Operation{
		Name:        "perform_action",
		Description: "Perform one bounded action against the latest observation. When supported, returns a fresh observation using the previous observe options. An observation_error does not undo the action. Retry the same request_id only to reconcile; a cached result may require a new observe.",
		SearchHint:  "computer action click type key scroll drag invoke desktop 操作 点击 输入 键盘 滚动 拖拽",
		InputSchema: actionSchema(),
		Idempotent:  true,
		Annotations: &command.OperationAnnotations{DestructiveHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, call *command.CallContext) (command.Result, error) {
			if call == nil || strings.TrimSpace(call.RequestID) == "" {
				return rejected(errors.New("Computer Use perform_action 需要宿主 request_id")), nil
			}
			var parsed actionInput
			if err := decodeInput(input, &parsed); err != nil {
				return rejected(err), nil
			}
			if err := parsed.validate(); err != nil {
				return rejected(err), nil
			}
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.PerformActionWithApproval(ctx, call.RequestID, parsed.action(), func() error {
				if r.requiresActionApproval(parsed) {
					return r.requestActionApproval(ctx, call.RequestID, parsed)
				}
				return nil
			})
			if err != nil {
				return rejected(err), nil
			}
			data := cloneMap(value.Data)
			if value.ImageData != "" {
				data["image_data"], data["image_mime"] = value.ImageData, value.ImageMIME
			}
			return applied(data), nil
		},
	}
}

// ActionPermissionPolicy lets the host mark additional operations as sensitive.
// Routine operations use the explicit target-session grant.
type ActionPermissionPolicy interface {
	RequiresComputerActionApproval(kind string) bool
}

func (r registry) requiresActionApproval(input actionInput) bool {
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind == "terminate_application" || kind == "clipboard_write" {
		return true
	}
	policy, ok := r.permissions.(ActionPermissionPolicy)
	return ok && policy.RequiresComputerActionApproval(kind)
}

func (r registry) requestTargetApproval(ctx context.Context, grant computeruse.TargetGrant) error {
	if r.permissions == nil || r.sessionKey == "" {
		return errors.New("Computer Use select_target 缺少宿主授权通道或 session identity")
	}
	decision, _, err := r.permissions.RequestPermissionWithID(ctx, r.sessionKey, sdkpermission.Request{
		ToolName: "nexus_computer_use_session", Title: "授权桌面应用操作", DisplayName: "Nexus Computer Use",
		Description: "授权当前回合在指定应用中执行所列动作；前台输入和有效期以本次请求为限。",
		Input: map[string]any{"application": grant.Application, "allowed_actions": grant.Actions,
			"allow_foreground_input": grant.AllowForegroundInput, "ttl_seconds": grant.TTLSeconds, "expires_at": grant.ExpiresAt},
	})
	if err != nil {
		return err
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		return errors.New("用户未批准 Computer Use 应用授权")
	}
	return nil
}

func (r registry) requestActionApproval(ctx context.Context, requestID string, input actionInput) error {
	if r.permissions == nil {
		return errors.New("Computer Use 缺少宿主真人审批通道")
	}
	if r.sessionKey == "" {
		return errors.New("Computer Use 缺少宿主 runtime session identity")
	}
	decision, _, err := r.permissions.RequestPermissionWithID(ctx, r.sessionKey, sdkpermission.Request{
		ToolName:    "nexus_computer_use_action",
		Input:       computerActionApprovalInput(requestID, input),
		Title:       "确认桌面操作",
		DisplayName: "Nexus Computer Use",
		Description: "Nexus 将在当前选定的桌面应用中执行一次 Computer Use 操作。",
	})
	if err != nil {
		return err
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		message := strings.TrimSpace(decision.Message)
		if message == "" {
			message = "用户未批准 Computer Use 桌面操作"
		}
		return errors.New(message)
	}
	return nil
}

func computerActionApprovalInput(requestID string, input actionInput) map[string]any {
	result := map[string]any{
		"request_id": requestID,
		"kind":       input.Kind,
	}
	if input.Bounds != nil {
		result["bounds"] = *input.Bounds
	}
	if input.RangeValue != nil {
		result["range_value"] = *input.RangeValue
	}
	if input.ElementRef != "" {
		result["element_ref"] = input.ElementRef
	}
	if input.X != nil {
		result["x"] = *input.X
	}
	if input.Y != nil {
		result["y"] = *input.Y
	}
	if input.ToX != nil {
		result["to_x"] = *input.ToX
	}
	if input.ToY != nil {
		result["to_y"] = *input.ToY
	}
	if input.Button != "" {
		result["button"] = input.Button
	}
	if input.Count != 0 {
		result["count"] = input.Count
	}
	if input.DurationMS != 0 {
		result["duration_ms"] = input.DurationMS
	}
	if input.DeltaX != nil {
		result["delta_x"] = *input.DeltaX
	}
	if input.DeltaY != nil {
		result["delta_y"] = *input.DeltaY
	}
	if input.Expanded != nil {
		result["expanded"] = *input.Expanded
	}
	if len(input.Keys) != 0 {
		result["keys"] = append([]string(nil), input.Keys...)
	}
	// Text and value can contain secrets. The permission card only needs to
	// show that input is present; the raw bytes must not enter the approval
	// event or its normal UI projection.
	if input.Text != "" {
		result["text_redacted"] = true
	}
	if input.Value != "" {
		result["value_redacted"] = true
	}
	return result
}

func (r registry) verifyState() command.Operation {
	return command.Operation{
		Name:        "verify_state",
		Description: "Verify a bounded state predicate against the selected target window after an action or observation.",
		SearchHint:  "computer verify state predicate window title element exists 校验 状态 标题 元素",
		InputSchema: predicateSchema(),
		ReadOnly:    true,
		ContextHandler: func(ctx context.Context, input map[string]any, _ *command.CallContext) (command.Result, error) {
			round, err := r.round(ctx)
			if err != nil {
				return rejected(err), nil
			}
			value, err := round.VerifyState(ctx, input)
			if err != nil {
				return rejected(err), nil
			}
			return structured(value), nil
		},
	}
}

func (r registry) closeTarget() command.Operation {
	return command.Operation{
		Name:        "close_target",
		Description: "Close this round's CUA session and discard its target, observation, pending mutation, and opaque reference state.",
		SearchHint:  "computer close target session cleanup 关闭目标 会话 清理",
		InputSchema: objectSchema(nil),
		Idempotent:  true,
		Annotations: &command.OperationAnnotations{DestructiveHint: true},
		ContextHandler: func(ctx context.Context, _ map[string]any, _ *command.CallContext) (command.Result, error) {
			if r.service == nil {
				return rejected(errors.New("Computer Use service 尚未装配")), nil
			}
			if err := r.service.CloseRound(ctx, r.owner, r.roundID); err != nil {
				return rejected(err), nil
			}
			return applied(map[string]any{"closed": true}), nil
		},
	}
}

type observeInput struct {
	ApplicationScene  bool   `json:"application_scene,omitempty"`
	WindowSelector    string `json:"window_selector,omitempty"`
	IncludeScreenshot bool   `json:"include_screenshot,omitempty"`
	Accessibility     string `json:"accessibility,omitempty"`
}

type actionInput struct {
	Bounds     *nexuscua.ScreenRect `json:"bounds,omitempty"`
	RangeValue *float64             `json:"range_value,omitempty"`
	Kind       string               `json:"kind"`
	ElementRef string               `json:"element_ref,omitempty"`
	Value      string               `json:"value,omitempty"`
	Text       string               `json:"text,omitempty"`
	Expanded   *bool                `json:"expanded,omitempty"`
	X          *uint32              `json:"x,omitempty"`
	Y          *uint32              `json:"y,omitempty"`
	ToX        *uint32              `json:"to_x,omitempty"`
	ToY        *uint32              `json:"to_y,omitempty"`
	Button     string               `json:"button,omitempty"`
	Count      uint8                `json:"count,omitempty"`
	DurationMS uint32               `json:"duration_ms,omitempty"`
	DeltaX     *int32               `json:"delta_x,omitempty"`
	DeltaY     *int32               `json:"delta_y,omitempty"`
	Keys       []string             `json:"keys,omitempty"`
}

func (i actionInput) action() computeruse.ActionInput {
	expanded := false
	if i.Expanded != nil {
		expanded = *i.Expanded
	}
	return computeruse.ActionInput{
		Bounds: i.Bounds, RangeValue: i.RangeValue, Kind: i.Kind, ElementRef: i.ElementRef, Value: i.Value, Text: i.Text,
		Expanded: expanded, X: uint32Value(i.X), Y: uint32Value(i.Y), ToX: uint32Value(i.ToX), ToY: uint32Value(i.ToY),
		Button: i.Button, Count: i.Count, DurationMS: i.DurationMS,
		DeltaX: int32Value(i.DeltaX), DeltaY: int32Value(i.DeltaY), Keys: i.Keys,
	}
}

func (i *actionInput) validate() error {
	kind := strings.ToLower(strings.TrimSpace(i.Kind))
	switch kind {
	case "focus_element", "invoke_element", "set_value", "toggle_element", "select_element", "set_expanded":
		if strings.TrimSpace(i.ElementRef) == "" {
			return errors.New("Computer Use element action 需要 element_ref")
		}
		if kind == "set_expanded" && i.Expanded == nil {
			return errors.New("Computer Use set_expanded 需要 expanded")
		}
	case "click_point", "press_point", "move_pointer":
		if i.X == nil || i.Y == nil {
			return errors.New("Computer Use point action 需要 x 和 y")
		}
	case "drag":
		if i.X == nil || i.Y == nil || i.ToX == nil || i.ToY == nil {
			return errors.New("Computer Use drag 需要 x、y、to_x 和 to_y")
		}
	case "scroll":
		if i.X == nil || i.Y == nil {
			return errors.New("Computer Use scroll 需要目标 x 和 y")
		}
		if i.DeltaX == nil && i.DeltaY == nil {
			return errors.New("Computer Use scroll 需要 delta_x 或 delta_y")
		}
	case "press_keys":
		if len(i.Keys) == 0 {
			return errors.New("Computer Use press_keys 需要 keys")
		}
		for index, key := range i.Keys {
			key = strings.ToLower(strings.TrimSpace(key))
			switch key {
			case "return":
				key = "enter"
			case "esc":
				key = "escape"
			}
			i.Keys[index] = key
		}
	}
	return nil
}

func uint32Value(value *uint32) uint32 {
	if value == nil {
		return 0
	}
	return *value
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"additionalProperties": false, "required": append([]string{}, required...),
	}
}

func stringProperty(description string, min, max int) map[string]any {
	return map[string]any{"type": "string", "description": description, "minLength": min, "maxLength": max}
}

func enumProperty(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func observeSchema() map[string]any {
	return objectSchema(map[string]any{
		"window_selector":    stringProperty("Optional exact window ref or title. Omit to reuse the selected window if it still exists, or select the only available window.", 0, 256),
		"include_screenshot": map[string]any{"type": "boolean", "default": true, "description": "Include the screenshot needed for pixel actions. Set false only for metadata recovery."},
		"application_scene":  map[string]any{"type": "boolean", "description": "Capture this application and its menus on the anchor display when advertised."},
		"accessibility":      enumProperty("Accessibility payload size mode. disabled does not disable the screenshot; metadata-only reads also require include_screenshot=false.", "interactive", "disabled", "full"),
	})
}

func actionSchema() map[string]any {
	return objectSchema(map[string]any{
		"kind":        enumProperty("Closed action kind.", "focus_window", "focus_element", "invoke_element", "click_point", "set_value", "toggle_element", "select_element", "set_expanded", "move_pointer", "type_text", "press_keys", "scroll", "drag", "restore_window", "set_window_bounds", "set_range_value", "press_point", "terminate_application"),
		"element_ref": stringProperty("Opaque element_ref from the latest observation.", 1, 256),
		"bounds":      objectSchema(map[string]any{"x": map[string]any{"type": "number"}, "y": map[string]any{"type": "number"}, "width": map[string]any{"type": "number", "minimum": 1}, "height": map[string]any{"type": "number", "minimum": 1}}, "x", "y", "width", "height"),
		"range_value": map[string]any{"type": "number", "description": "Numeric value bounded by get_computer.action_limits."},
		"value":       map[string]any{"type": "string", "description": "UTF-8 byte limit from get_computer.action_limits.max_text_bytes"},
		"text":        map[string]any{"type": "string", "description": "UTF-8 byte limit from get_computer.action_limits.max_text_bytes"},
		"expanded":    map[string]any{"type": "boolean"},
		"x":           map[string]any{"type": "integer", "minimum": 0, "maximum": 1000, "description": "Normalized coordinate in 0..1000."},
		"y":           map[string]any{"type": "integer", "minimum": 0, "maximum": 1000, "description": "Normalized coordinate in 0..1000."},
		"to_x":        map[string]any{"type": "integer", "minimum": 0, "maximum": 1000, "description": "Normalized coordinate in 0..1000."},
		"to_y":        map[string]any{"type": "integer", "minimum": 0, "maximum": 1000, "description": "Normalized coordinate in 0..1000."},
		"button":      enumProperty("Pointer button.", "left", "middle", "right"),
		"count":       map[string]any{"type": "integer", "minimum": 1},
		"duration_ms": map[string]any{"type": "integer", "minimum": 0, "description": "Maximum from get_computer.action_limits.max_pointer_duration_ms"},
		"delta_x":     map[string]any{"type": "integer", "description": "Absolute bound from get_computer.action_limits.max_scroll_delta"},
		"delta_y":     map[string]any{"type": "integer", "description": "Absolute bound from get_computer.action_limits.max_scroll_delta"},
		"keys":        map[string]any{"type": "array", "items": stringProperty("Key name, e.g. mod, enter, escape, tab. Case-insensitive; Return means enter.", 1, 128)},
	}, "kind")
}

// Runtime capability bounds produce the model's detailed action contract. The
// static registry schema only describes shape, so stale hardcoded maxima cannot
// reject a runtime-supported action or suggest an unsupported limit.
func boundedActionSchema(limits nexuscua.ActionLimits) map[string]any {
	schema := actionSchema()
	props := schema["properties"].(map[string]any)
	props["duration_ms"].(map[string]any)["maximum"] = limits.MaxPointerDurationMS
	props["keys"].(map[string]any)["maxItems"] = limits.MaxKeys
	props["count"].(map[string]any)["maximum"] = limits.MaxClickCount
	for _, key := range []string{"delta_x", "delta_y"} {
		prop := props[key].(map[string]any)
		prop["minimum"] = -int64(limits.MaxScrollDelta)
		prop["maximum"] = limits.MaxScrollDelta
	}
	props["range_value"].(map[string]any)["minimum"] = -float64(limits.MaxRangeValue)
	props["range_value"].(map[string]any)["maximum"] = limits.MaxRangeValue
	bounds := props["bounds"].(map[string]any)["properties"].(map[string]any)
	for _, key := range []string{"x", "y"} {
		bounds[key].(map[string]any)["minimum"] = -int64(limits.MaxWindowCoordinate)
		bounds[key].(map[string]any)["maximum"] = limits.MaxWindowCoordinate
	}
	for _, key := range []string{"width", "height"} {
		bounds[key].(map[string]any)["maximum"] = limits.MaxWindowDimension
	}
	return schema
}

func predicateSchema() map[string]any {
	return objectSchema(map[string]any{
		"kind": enumProperty("Closed predicate kind.", "window_title_contains", "element_exists"),
		"text": stringProperty("Text for window_title_contains.", 0, 4096),
		"role": stringProperty("Optional accessibility role for element_exists.", 0, 256),
		"name": stringProperty("Optional accessibility name for element_exists.", 0, 4096),
	}, "kind")
}

func decodeInput(input map[string]any, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("Computer Use input 编码失败: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("Computer Use input 无效: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("Computer Use input 必须是单个 JSON 对象")
	}
	return nil
}

func structured(data map[string]any) command.Result {
	data = cloneMap(data)
	imageData, _ := data["image_data"].(string)
	imageMIME, _ := data["image_mime"].(string)
	delete(data, "image_data")
	delete(data, "image_mime")
	payload, _ := json.Marshal(data)
	content := []map[string]any{{"type": "text", "text": string(payload)}}
	if imageData != "" && imageMIME != "" {
		content = append(content, map[string]any{
			"type": "image", "data": imageData, "mimeType": imageMIME,
		})
	}
	return command.Result{
		Content:           content,
		StructuredContent: data,
	}
}

func applied(data map[string]any) command.Result {
	data = cloneMap(data)
	data["outcome"] = "applied"
	return structured(data)
}

func rejected(err error) command.Result {
	message := "Computer Use command failed"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	data := map[string]any{"outcome": "rejected", "message": message}
	var cuaErr *nexuscua.CUAError
	if errors.As(err, &cuaErr) {
		data["error_code"] = cuaErr.Code
		data["mutation_status"] = cuaErr.MutationStatus
		data["retryable"] = cuaErr.Retryable
		if cuaErr.RecoveryAction != nil {
			data["recovery_action"] = *cuaErr.RecoveryAction
			if *cuaErr.RecoveryAction == "focus_window" && cuaErr.MutationStatus != nexuscua.MutationIndeterminate {
				data["recovery_hint"] = "For the same selected window, call observe with include_screenshot=false AND accessibility=disabled. Then perform one authorized focus_window and observe again. Do not reopen the target session. If recovery fails again, report the error."
			}
		}
	}
	// Runtimes may expose only MCP Content to the model. Recovery and mutation
	// evidence must be present there as well as in the host's structured receipt.
	result := structured(data)
	result.IsError = true
	return result
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+1)
	for key, value := range input {
		result[key] = value
	}
	return result
}

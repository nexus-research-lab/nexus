// INPUT: round-scoped runtimecommand Actor, fresh operation input, and typed nexus-cua client.
// OUTPUT: closed Computer Use contracts and safe application/observation/action projections.
// POS: Agent -> Nexus policy -> nexus-cua semantic adapter; no raw transport or manifest passthrough.
package computeruse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
)

const (
	computerReadTimeout       = 10 * time.Second
	computerActionFirstWait   = 5 * time.Second
	computerActionMaximumWait = 60 * time.Second
	computerSessionTTLSeconds = 15 * 60
)

func (service *Service) Operations(actor runtimecommand.Actor) []runtimecommand.Operation {
	return []runtimecommand.Operation{
		{
			Name: "get_computer", Description: "Read enablement, package, sidecar, permission, and current round target state.",
			InputSchema: emptyObjectSchema(), ReadOnly: true,
			Handler: func(ctx context.Context, _ map[string]any) (runtimecommand.Result, error) {
				return service.inspectComputer(ctx, actor), nil
			},
		},
		{
			Name: "list_applications", Description: "Discover host-approved desktop application candidates without opening a session.",
			InputSchema: emptyObjectSchema(), ReadOnly: true,
			ContextHandler: func(ctx context.Context, input map[string]any, _ *runtimecommand.CallContext) (runtimecommand.Result, error) {
				return service.listApplications(ctx, actor), nil
			},
		},
		{
			Name: "select_target", Description: "Select one unambiguous application/window and open a host-generated bounded session.",
			InputSchema: selectTargetSchema(), Idempotent: true,
			ContextHandler: func(ctx context.Context, input map[string]any, _ *runtimecommand.CallContext) (runtimecommand.Result, error) {
				return service.selectTarget(ctx, actor, input), nil
			},
		},
		{
			Name: "observe", Description: "Refresh screenshot/accessibility state for the current round target.",
			InputSchema: observeSchema(), ReadOnly: true,
			ContextHandler: func(ctx context.Context, input map[string]any, _ *runtimecommand.CallContext) (runtimecommand.Result, error) {
				return service.observe(ctx, actor, input), nil
			},
		},
		{
			Name: "perform_action", Description: "Perform one closed desktop action against the latest observation; retries reconcile the exact same request.",
			InputSchema: actionSchema(), Idempotent: true,
			ContextHandler: func(ctx context.Context, input map[string]any, call *runtimecommand.CallContext) (runtimecommand.Result, error) {
				return service.performAction(ctx, actor, input, call), nil
			},
		},
		{
			Name: "verify_state", Description: "Evaluate one closed state predicate against the current target.",
			InputSchema: predicateSchema(), ReadOnly: true,
			ContextHandler: func(ctx context.Context, input map[string]any, _ *runtimecommand.CallContext) (runtimecommand.Result, error) {
				return service.verifyState(ctx, actor, input), nil
			},
		},
		{
			Name: "close_target", Description: "Close and forget the current round target; round teardown remains the final cleanup.",
			InputSchema: emptyObjectSchema(), Idempotent: true,
			ContextHandler: func(ctx context.Context, input map[string]any, _ *runtimecommand.CallContext) (runtimecommand.Result, error) {
				return service.closeTarget(ctx, actor), nil
			},
		},
	}
}

func (service *Service) inspectComputer(ctx context.Context, actor runtimecommand.Actor) runtimecommand.Result {
	status, err := service.Status(ctx, actor.OwnerUserID)
	if err != nil {
		return computerErrorResult(err)
	}
	data := map[string]any{"outcome": "observed", "status": status}
	if service != nil && service.supervisor != nil {
		client, _, ready := service.supervisor.Current()
		if ready {
			permissionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			permissions, permissionErr := client.GetPermissionStatus(permissionCtx, 2*time.Second)
			cancel()
			if permissionErr == nil {
				data["permissions"] = permissions
			}
		}
	}
	if state := service.findRound(actor); state != nil {
		state.mu.Lock()
		if state.sessionID != "" && state.application != nil && state.window != nil {
			data["target"] = map[string]any{
				"application": applicationProjection(*state.application),
				"window":      windowProjection(*state.window),
				"epoch":       state.epoch,
			}
		}
		state.mu.Unlock()
	}
	return computerResult(data, "Computer Use state inspected")
}

func (service *Service) listApplications(ctx context.Context, actor runtimecommand.Actor) runtimecommand.Result {
	if err := service.authorizeActor(ctx, actor); err != nil {
		return computerErrorResult(err)
	}
	release := service.admitOperation()
	defer release()
	client, epoch, _, err := service.supervisor.EnsureReady(ctx)
	if err != nil {
		return computerErrorResult(err)
	}
	discovery, err := client.DiscoverApplications(ctx, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	applications := make([]map[string]any, 0, len(discovery.Applications))
	for _, application := range discovery.Applications {
		if deniedApplication(application) {
			continue
		}
		applications = append(applications, applicationProjection(application))
	}
	return computerResult(map[string]any{
		"outcome": "observed", "epoch": epoch, "applications": applications, "complete": discovery.Complete,
	}, "Desktop applications discovered")
}

func (service *Service) selectTarget(ctx context.Context, actor runtimecommand.Actor, input map[string]any) runtimecommand.Result {
	if err := service.authorizeActor(ctx, actor); err != nil {
		return computerErrorResult(err)
	}
	release := service.admitOperation()
	defer release()
	state, err := service.getOrCreateRound(actor)
	if err != nil {
		return computerErrorResult(err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.client != nil && state.sessionID != "" {
		_ = state.client.CloseSession(ctx, state.sessionID, 2*time.Second)
		clearTargetLocked(state)
	}
	client, epoch, capabilities, err := service.supervisor.EnsureReady(ctx)
	if err != nil {
		return computerErrorResult(err)
	}
	discovery, err := client.DiscoverApplications(ctx, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	application, err := nexuscua.SelectApplication(discovery.Applications, stringInput(input, "application_selector"))
	if err != nil {
		return computerErrorResult(newCommandError("target_selection_failed", err.Error(), "use an exact application name or stable application ID from list_applications"))
	}
	if deniedApplication(*application) {
		return computerErrorResult(newCommandError("target_denied", "Computer Use host policy denies this application surface", "choose a non-system application surface"))
	}
	mode := nexuscua.PermissionMode(stringInputDefault(input, "mode", string(nexuscua.PermissionBounded)))
	if mode != nexuscua.PermissionReadOnly && mode != nexuscua.PermissionBounded {
		return computerErrorResult(errors.New("Computer Use session mode is invalid"))
	}
	manifest := nexuscua.CapabilityManifest{
		Mode: mode, ApplicationRefs: []nexuscua.DiscoveryRef{application.DiscoveryRef},
		AllowedActions:       allowedActions(capabilities.Actions, mode),
		AllowForegroundInput: mode == nexuscua.PermissionBounded,
		TTLSeconds:           computerSessionTTLSeconds,
	}
	opened, err := client.OpenSession(ctx, nexuscua.OpenSessionInput{Manifest: manifest}, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = client.CloseSession(context.Background(), opened.SessionID, 2*time.Second)
		}
	}()
	apps, err := client.ListApps(ctx, opened.SessionID, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	appRef, err := selectedAppRef(apps, *application)
	if err != nil {
		return computerErrorResult(newCommandError("target_unavailable", err.Error(), "run list_applications and select the target again"))
	}
	windows, err := client.ListWindows(ctx, opened.SessionID, &appRef, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	window, err := selectTargetWindow(windows, stringInput(input, "window_selector"))
	if err != nil {
		return computerErrorResult(newCommandError("window_selection_failed", err.Error(), "use an exact visible window title"))
	}
	observation, err := client.ObserveWindow(ctx, nexuscua.ObserveWindowInput{
		SessionID: opened.SessionID, WindowRef: window.WindowRef,
		IncludeScreenshot: boolInputDefault(input, "include_screenshot", true),
		Accessibility:     accessibilityInput(input),
	}, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	projected, err := service.projectObservation(actor, state, epoch, observation)
	if err != nil {
		return computerErrorResult(err)
	}
	state.client = client
	state.epoch = epoch
	state.sessionID = opened.SessionID
	applicationCopy := *application
	windowCopy := *window
	state.application = &applicationCopy
	state.window = &windowCopy
	state.observation = &observation
	state.actions = make(map[string]*roundAction)
	closeOnError = false
	return computerResult(map[string]any{
		"outcome": "selected", "epoch": epoch,
		"application": applicationProjection(*application), "window": windowProjection(*window),
		"observation": projected,
	}, "Desktop target selected and observed")
}

func (service *Service) observe(ctx context.Context, actor runtimecommand.Actor, input map[string]any) runtimecommand.Result {
	if err := service.authorizeActor(ctx, actor); err != nil {
		return computerErrorResult(err)
	}
	release := service.admitOperation()
	defer release()
	state := service.findRound(actor)
	if state == nil {
		return computerErrorResult(targetRequiredError())
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := service.ensureTargetEpochLocked(state); err != nil {
		return computerErrorResult(err)
	}
	observation, err := state.client.ObserveWindow(ctx, nexuscua.ObserveWindowInput{
		SessionID: state.sessionID, WindowRef: state.window.WindowRef,
		IncludeScreenshot: boolInputDefault(input, "include_screenshot", true),
		Accessibility:     accessibilityInput(input),
	}, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	projected, err := service.projectObservation(actor, state, state.epoch, observation)
	if err != nil {
		return computerErrorResult(err)
	}
	state.observation = &observation
	return computerResult(map[string]any{"outcome": "observed", "observation": projected}, "Desktop target observed")
}

func (service *Service) verifyState(ctx context.Context, actor runtimecommand.Actor, input map[string]any) runtimecommand.Result {
	if err := service.authorizeActor(ctx, actor); err != nil {
		return computerErrorResult(err)
	}
	release := service.admitOperation()
	defer release()
	predicate, err := parsePredicate(input)
	if err != nil {
		return computerErrorResult(newCommandError("invalid_input", err.Error(), "read the fresh verify_state contract and submit one closed predicate"))
	}
	state := service.findRound(actor)
	if state == nil {
		return computerErrorResult(targetRequiredError())
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if err = service.ensureTargetEpochLocked(state); err != nil {
		return computerErrorResult(err)
	}
	verification, err := state.client.VerifyState(ctx, nexuscua.VerifyStateInput{
		SessionID: state.sessionID, WindowRef: state.window.WindowRef, Predicate: predicate,
	}, computerReadTimeout)
	if err != nil {
		return computerErrorResult(err)
	}
	return computerResult(map[string]any{
		"outcome": "verified", "matched": verification.Matched, "evidence": verification.Evidence,
	}, "Desktop state verified")
}

func (service *Service) performAction(
	ctx context.Context,
	actor runtimecommand.Actor,
	input map[string]any,
	call *runtimecommand.CallContext,
) runtimecommand.Result {
	if err := service.authorizeActor(ctx, actor); err != nil {
		return computerErrorResult(err)
	}
	release := service.admitOperation()
	defer release()
	if call == nil || !runtimecommand.ValidRequestID(call.RequestID) {
		return computerErrorResult(newCommandError("invalid_request_id", "Computer Use action requires a valid stable request ID", "follow the fresh contract request_id rule"))
	}
	digest, err := actionDigest("perform_action", input)
	if err != nil {
		return computerErrorResult(err)
	}
	state := service.findRound(actor)
	if state == nil {
		return computerErrorResult(targetRequiredError())
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if err = service.ensureTargetEpochLocked(state); err != nil {
		return computerErrorResult(err)
	}
	if existing := state.actions[call.RequestID]; existing != nil {
		if existing.digest != digest {
			return computerErrorResult(newCommandError("request_conflict", "Computer Use request ID conflicts with a different action input", "use a new request ID when the operation target or input changes"))
		}
		if existing.epoch != state.epoch {
			return computerErrorResult(ErrEpochChanged)
		}
		if existing.completed {
			return existing.result
		}
		return service.reconcileActionLocked(ctx, actor, state, existing)
	}
	action, err := parseAction(input)
	if err != nil {
		return computerErrorResult(newCommandError("invalid_input", err.Error(), "read the fresh perform_action contract and submit one closed action"))
	}
	record := &roundAction{digest: digest, epoch: state.epoch, wait: computerActionFirstWait}
	state.actions[call.RequestID] = record
	output, handle, err := state.client.PerformAction(ctx, nexuscua.PerformActionInput{
		SessionID: state.sessionID, WindowRef: state.window.WindowRef,
		ObservationID: state.observation.ObservationID, Action: action,
	}, record.wait)
	record.handle = handle
	if err != nil {
		result := computerErrorResult(err)
		if !isMutationIndeterminate(err) {
			record.completed = true
			record.result = result
		}
		return result
	}
	return service.completeActionLocked(ctx, actor, state, record, output)
}

func (service *Service) reconcileActionLocked(
	ctx context.Context,
	actor runtimecommand.Actor,
	state *roundState,
	record *roundAction,
) runtimecommand.Result {
	if record.handle == nil {
		record.completed = true
		record.result = computerErrorResult(errors.New("Computer Use mutation cannot be reconciled safely"))
		return record.result
	}
	nextWait := record.wait * 2
	if nextWait > computerActionMaximumWait {
		nextWait = computerActionMaximumWait
	}
	if nextWait <= record.wait {
		return computerErrorResult(newCommandError("mutation_indeterminate", "Computer Use mutation remains indeterminate at the maximum reconciliation wait", "do not replay the action; inspect fresh state and ask the user before any new intent"))
	}
	record.wait = nextWait
	output, err := state.client.ReconcileAction(ctx, record.handle, nextWait)
	if err != nil {
		return computerErrorResult(err)
	}
	return service.completeActionLocked(ctx, actor, state, record, output)
}

func (service *Service) completeActionLocked(
	ctx context.Context,
	actor runtimecommand.Actor,
	state *roundState,
	record *roundAction,
	output nexuscua.ActionOutput,
) runtimecommand.Result {
	data := map[string]any{
		"outcome": "applied", "action": output,
	}
	observation, err := state.client.ObserveWindow(ctx, nexuscua.ObserveWindowInput{
		SessionID: state.sessionID, WindowRef: state.window.WindowRef,
		IncludeScreenshot: true, Accessibility: nexuscua.AccessibilityInteractive,
	}, computerReadTimeout)
	if err != nil {
		data["observation_refresh_required"] = true
		data["observation_error"] = safeErrorCode(err)
	} else if projected, projectionErr := service.projectObservation(actor, state, state.epoch, observation); projectionErr != nil {
		data["observation_refresh_required"] = true
		data["observation_error"] = safeErrorCode(projectionErr)
	} else {
		state.observation = &observation
		data["observation"] = projected
	}
	record.completed = true
	record.result = computerResult(data, "Desktop action dispatched; post-action state is reported separately")
	return record.result
}

func (service *Service) closeTarget(ctx context.Context, actor runtimecommand.Actor) runtimecommand.Result {
	if err := service.authorizeActor(ctx, actor); err != nil {
		return computerErrorResult(err)
	}
	release := service.admitOperation()
	defer release()
	state := service.findRound(actor)
	if state == nil {
		return computerResult(map[string]any{"outcome": "unchanged"}, "No Computer Use target is open")
	}
	state.mu.Lock()
	client := state.client
	sessionID := state.sessionID
	clearTargetLocked(state)
	state.mu.Unlock()
	if client != nil && sessionID != "" {
		if err := client.CloseSession(ctx, sessionID, 2*time.Second); err != nil {
			return computerErrorResult(err)
		}
	}
	removeRoundArtifacts(state.workspacePath, state.roundSegment)
	return computerResult(map[string]any{"outcome": "closed"}, "Computer Use target closed")
}

func (service *Service) ensureTargetEpochLocked(state *roundState) error {
	if state == nil || state.client == nil || state.sessionID == "" || state.window == nil || state.observation == nil {
		return targetRequiredError()
	}
	client, epoch, ready := service.supervisor.Current()
	if !ready || client != state.client || epoch != state.epoch {
		return ErrEpochChanged
	}
	return nil
}

func targetRequiredError() error {
	return newCommandError("target_required", "Select a Computer Use target in this physical round first", "run list_applications, then select_target")
}

func clearTargetLocked(state *roundState) {
	state.client = nil
	state.epoch = 0
	state.sessionID = ""
	state.application = nil
	state.window = nil
	state.observation = nil
	state.actions = make(map[string]*roundAction)
}

func (service *Service) projectObservation(
	actor runtimecommand.Actor,
	state *roundState,
	epoch uint64,
	observation nexuscua.WindowObservation,
) (map[string]any, error) {
	result := map[string]any{
		"observation_id": observation.ObservationID, "captured_at": observation.CapturedAt,
		"window_screen_bounds": observation.WindowScreenBounds,
		"elements":             observation.Elements, "elements_complete": observation.ElementsComplete,
		"elements_truncation": observation.ElementsTruncation,
	}
	if observation.Screenshot != nil {
		path, err := projectScreenshot(
			service.supervisor, actor.WorkspacePath, state.roundSegment, epoch, *observation.Screenshot,
		)
		if err != nil {
			return nil, err
		}
		result["screenshot"] = map[string]any{
			"artifact_ref": observation.Screenshot.ArtifactRef, "path": path,
			"mime_type": observation.Screenshot.MIMEType, "mapping": observation.Screenshot.Mapping,
			"byte_length": observation.Screenshot.ByteLength, "sha256": observation.Screenshot.SHA256,
		}
	}
	return result, nil
}

func applicationProjection(application nexuscua.DiscoveredApplication) map[string]any {
	result := map[string]any{
		"name": application.Name, "application_id": application.ApplicationID,
		"foreground": application.Foreground, "platform": application.Provenance.Platform,
		"expires_at": application.ExpiresAt,
	}
	if application.Provenance.SignatureStatus != nil {
		result["signature_status"] = *application.Provenance.SignatureStatus
	}
	if application.Provenance.Publisher != nil {
		result["publisher"] = *application.Provenance.Publisher
	}
	return result
}

func windowProjection(window nexuscua.WindowSummary) map[string]any {
	return map[string]any{
		"title": window.Title, "screen_bounds": window.ScreenBounds,
		"minimized": window.Minimized, "visible": window.Visible, "foreground": window.Foreground,
	}
}

func selectedAppRef(apps []nexuscua.ApplicationSummary, selected nexuscua.DiscoveredApplication) (nexuscua.AppRef, error) {
	var matches []nexuscua.AppRef
	for _, application := range apps {
		if application.ApplicationID == selected.ApplicationID {
			matches = append(matches, application.AppRef)
		}
	}
	if len(matches) != 1 {
		return "", errors.New("selected application did not resolve to one bounded session application")
	}
	return matches[0], nil
}

func selectTargetWindow(windows []nexuscua.WindowSummary, selector string) (*nexuscua.WindowSummary, error) {
	candidates := make([]nexuscua.WindowSummary, 0, len(windows))
	for _, window := range windows {
		if window.Visible && !window.Minimized {
			candidates = append(candidates, window)
		}
	}
	if strings.TrimSpace(selector) != "" {
		return nexuscua.SelectWindow(candidates, selector)
	}
	if len(candidates) != 1 {
		return nil, fmt.Errorf("application has %d visible windows; provide an exact window selector", len(candidates))
	}
	return &candidates[0], nil
}

func allowedActions(capabilities []nexuscua.ActionKind, mode nexuscua.PermissionMode) []nexuscua.ActionKind {
	if mode == nexuscua.PermissionReadOnly {
		return []nexuscua.ActionKind{}
	}
	hostAllowed := map[nexuscua.ActionKind]struct{}{
		nexuscua.ActionFocusWindow: {}, nexuscua.ActionFocusElement: {}, nexuscua.ActionInvokeElement: {},
		nexuscua.ActionClickPoint: {}, nexuscua.ActionSetValue: {}, nexuscua.ActionToggleElement: {},
		nexuscua.ActionSelectElement: {}, nexuscua.ActionSetExpanded: {}, nexuscua.ActionMovePointer: {},
		nexuscua.ActionTypeText: {}, nexuscua.ActionPressKeys: {}, nexuscua.ActionScroll: {}, nexuscua.ActionDrag: {},
	}
	result := make([]nexuscua.ActionKind, 0, len(capabilities))
	for _, action := range capabilities {
		if _, ok := hostAllowed[action]; ok {
			result = append(result, action)
		}
	}
	return result
}

func deniedApplication(application nexuscua.DiscoveredApplication) bool {
	if application.Provenance.SignatureStatus != nil && *application.Provenance.SignatureStatus == nexuscua.SignatureInvalid {
		return true
	}
	identity := strings.ToLower(application.Name + "\x00" + application.ApplicationID)
	for _, denied := range []string{"loginwindow", "securityagent", "credentialui", "lockapp", "consent.exe", "useraccountcontrol"} {
		if strings.Contains(identity, denied) {
			return true
		}
	}
	return false
}

func boolInputDefault(input map[string]any, key string, fallback bool) bool {
	value, ok := input[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func accessibilityInput(input map[string]any) nexuscua.AccessibilityMode {
	value := nexuscua.AccessibilityMode(stringInputDefault(input, "accessibility", string(nexuscua.AccessibilityInteractive)))
	if value != nexuscua.AccessibilityDisabled && value != nexuscua.AccessibilityInteractive && value != nexuscua.AccessibilityFull {
		return nexuscua.AccessibilityInteractive
	}
	return value
}

func safeErrorCode(err error) string {
	var cuaError *nexuscua.CUAError
	if errors.As(err, &cuaError) {
		return string(cuaError.Code)
	}
	if errors.Is(err, ErrEpochChanged) {
		return "epoch_changed"
	}
	return "observation_failed"
}

func isMutationIndeterminate(err error) bool {
	var indeterminate *nexuscua.MutationIndeterminateError
	if errors.As(err, &indeterminate) {
		return true
	}
	var cuaError *nexuscua.CUAError
	return errors.As(err, &cuaError) && cuaError.MutationStatus == nexuscua.MutationIndeterminate
}

func emptyObjectSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}

func selectTargetSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"application_selector"},
		"properties": map[string]any{
			"application_selector": map[string]any{"type": "string"},
			"window_selector":      map[string]any{"type": "string"},
			"mode":                 map[string]any{"type": "string", "enum": []string{"read_only", "bounded"}},
			"include_screenshot":   map[string]any{"type": "boolean"},
			"accessibility":        accessibilitySchema(),
		},
	}
}

func observeSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"include_screenshot": map[string]any{"type": "boolean"}, "accessibility": accessibilitySchema(),
		},
	}
}

func accessibilitySchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"disabled", "interactive", "full"}}
}

func actionSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"kind"},
		"properties": map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{
				"focus_window", "focus_element", "invoke_element", "click_point", "set_value", "toggle_element",
				"select_element", "set_expanded", "move_pointer", "type_text", "press_keys", "scroll", "drag",
			}},
			"element_ref": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"},
			"text": map[string]any{"type": "string"}, "expanded": map[string]any{"type": "boolean"},
			"x": map[string]any{"type": "integer"}, "y": map[string]any{"type": "integer"},
			"from_x": map[string]any{"type": "integer"}, "from_y": map[string]any{"type": "integer"},
			"to_x": map[string]any{"type": "integer"}, "to_y": map[string]any{"type": "integer"},
			"button": map[string]any{"type": "string", "enum": []string{"left", "middle", "right"}},
			"count":  map[string]any{"type": "integer"}, "duration_ms": map[string]any{"type": "integer"},
			"keys":    map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "string"}},
			"delta_x": map[string]any{}, "delta_y": map[string]any{},
		},
	}
}

func predicateSchema() map[string]any {
	rect := map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"x", "y", "width", "height"},
		"properties": map[string]any{"x": map[string]any{}, "y": map[string]any{}, "width": map[string]any{}, "height": map[string]any{}},
	}
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"kind"},
		"properties": map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{"window_title_contains", "element_exists", "bounds_contained"}},
			"text": map[string]any{"type": "string"}, "role": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
			"inner": rect, "outer": rect,
		},
	}
}

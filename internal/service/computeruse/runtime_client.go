// INPUT: private sidecar endpoint/token identity and typed nexus-cua SDK requests.
// OUTPUT: a narrow client interface plus immutable mutation handles for Nexus policy code.
// POS: official nexus-cua Go SDK adapter; raw protocol and transport credentials stop here.
package computeruse

import (
	"context"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

// RuntimeClient is the typed sidecar surface used by Nexus. It deliberately
// excludes generic request transport and token access.
type RuntimeClient interface {
	GetCapabilities(context.Context, time.Duration) (nexuscua.DriverCapabilities, error)
	GetPermissionStatus(context.Context, time.Duration) (nexuscua.PermissionStatus, error)
	DiscoverApplications(context.Context, time.Duration) (nexuscua.DiscoverApplicationsOutput, error)
	OpenSession(context.Context, nexuscua.OpenSessionInput, time.Duration) (nexuscua.OpenSessionOutput, error)
	CloseSession(context.Context, nexuscua.SessionID, time.Duration) error
	ListApps(context.Context, nexuscua.SessionID, time.Duration) ([]nexuscua.ApplicationSummary, error)
	ListWindows(context.Context, nexuscua.SessionID, *nexuscua.AppRef, time.Duration) ([]nexuscua.WindowSummary, error)
	ObserveWindow(context.Context, nexuscua.ObserveWindowInput, time.Duration) (nexuscua.WindowObservation, error)
	VerifyState(context.Context, nexuscua.VerifyStateInput, time.Duration) (nexuscua.VerificationOutput, error)
	PerformAction(context.Context, nexuscua.PerformActionInput, time.Duration) (nexuscua.ActionOutput, ActionHandle, error)
	ReconcileAction(context.Context, ActionHandle, time.Duration) (nexuscua.ActionOutput, error)
}

// ActionHandle identifies the exact immutable SDK mutation that may be
// reconciled. Implementations must not allow its target or payload to change.
type ActionHandle interface {
	RequestID() string
	privateActionHandle()
}

type sdkActionHandle struct {
	request *nexuscua.ActionRequest
}

func (handle *sdkActionHandle) RequestID() string {
	if handle == nil || handle.request == nil {
		return ""
	}
	return string(handle.request.RequestID())
}

func (*sdkActionHandle) privateActionHandle() {}

type sdkRuntimeClient struct {
	client *nexuscua.Client
}

func newSDKRuntimeClient(endpoint, tokenFile string) (RuntimeClient, error) {
	client, err := nexuscua.NewClient(nexuscua.Config{Endpoint: endpoint, TokenFile: tokenFile})
	if err != nil {
		return nil, err
	}
	return &sdkRuntimeClient{client: client}, nil
}

func (client *sdkRuntimeClient) GetCapabilities(ctx context.Context, timeout time.Duration) (nexuscua.DriverCapabilities, error) {
	return client.client.GetCapabilities(ctx, timeout)
}

func (client *sdkRuntimeClient) GetPermissionStatus(ctx context.Context, timeout time.Duration) (nexuscua.PermissionStatus, error) {
	return client.client.GetPermissionStatus(ctx, timeout)
}

func (client *sdkRuntimeClient) DiscoverApplications(ctx context.Context, timeout time.Duration) (nexuscua.DiscoverApplicationsOutput, error) {
	return client.client.DiscoverApplications(ctx, timeout)
}

func (client *sdkRuntimeClient) OpenSession(ctx context.Context, input nexuscua.OpenSessionInput, timeout time.Duration) (nexuscua.OpenSessionOutput, error) {
	return client.client.OpenSession(ctx, input, timeout)
}

func (client *sdkRuntimeClient) CloseSession(ctx context.Context, sessionID nexuscua.SessionID, timeout time.Duration) error {
	return client.client.CloseSession(ctx, sessionID, timeout)
}

func (client *sdkRuntimeClient) ListApps(ctx context.Context, sessionID nexuscua.SessionID, timeout time.Duration) ([]nexuscua.ApplicationSummary, error) {
	return client.client.ListApps(ctx, sessionID, timeout)
}

func (client *sdkRuntimeClient) ListWindows(ctx context.Context, sessionID nexuscua.SessionID, appRef *nexuscua.AppRef, timeout time.Duration) ([]nexuscua.WindowSummary, error) {
	return client.client.ListWindows(ctx, sessionID, appRef, timeout)
}

func (client *sdkRuntimeClient) ObserveWindow(ctx context.Context, input nexuscua.ObserveWindowInput, timeout time.Duration) (nexuscua.WindowObservation, error) {
	return client.client.ObserveWindow(ctx, input, timeout)
}

func (client *sdkRuntimeClient) VerifyState(ctx context.Context, input nexuscua.VerifyStateInput, timeout time.Duration) (nexuscua.VerificationOutput, error) {
	return client.client.VerifyState(ctx, input, timeout)
}

func (client *sdkRuntimeClient) PerformAction(ctx context.Context, input nexuscua.PerformActionInput, timeout time.Duration) (nexuscua.ActionOutput, ActionHandle, error) {
	result, request, err := client.client.PerformAction(ctx, input, timeout)
	if request == nil {
		return result, nil, err
	}
	return result, &sdkActionHandle{request: request}, err
}

func (client *sdkRuntimeClient) ReconcileAction(ctx context.Context, handle ActionHandle, timeout time.Duration) (nexuscua.ActionOutput, error) {
	typed, ok := handle.(*sdkActionHandle)
	if !ok || typed == nil || typed.request == nil {
		return nexuscua.ActionOutput{}, ErrEpochChanged
	}
	return client.client.ReconcileAction(ctx, typed.request, timeout)
}

type runtimeClientFactory func(endpoint string, tokenFile string) (RuntimeClient, error)

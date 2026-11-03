// INPUT: fresh runtime launch catalog, optional name/id query, and exact application approval.
// OUTPUT: filtered discovery and bounded launch reconciliation under the original request ID.
// POS: round-owned application lifecycle; never retries an unknown launch as new.
package computeruse

import (
	"context"
	"errors"
	"reflect"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

type launchClient interface {
	DiscoverLaunchableApplications(context.Context, time.Duration) (nexuscua.DiscoverLaunchableApplicationsOutput, error)
	LaunchApplication(context.Context, nexuscua.LaunchApplicationInput, time.Duration) (nexuscua.ApplicationLaunchOutput, *nexuscua.LaunchRequest, error)
	ReconcileLaunch(context.Context, *nexuscua.LaunchRequest, time.Duration) (nexuscua.ApplicationLaunchOutput, error)
}

func (r *Round) ListLaunchableApplications(ctx context.Context, query string) (map[string]any, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	err := r.ensureOpenLocked()
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}
	client, ok := r.client.(launchClient)
	if !ok {
		return nil, errors.New("Computer Use runtime 不支持启动应用")
	}
	catalog, err := client.DiscoverLaunchableApplications(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	if len(catalog.Applications) > 1024 {
		return nil, errors.New("Computer Use launch catalog 超出上限")
	}
	applications := make([]nexuscua.LaunchableApplication, 0, len(catalog.Applications))
	for _, app := range catalog.Applications {
		if matchesApplicationQuery(app.Name, app.ApplicationID, query) {
			applications = append(applications, app)
		}
	}
	r.mu.Lock()
	r.launchables = make(map[nexuscua.LaunchRef]nexuscua.LaunchableApplication, len(applications))
	for _, app := range applications {
		r.launchables[app.LaunchRef] = app
	}
	r.mu.Unlock()
	return map[string]any{"applications": applications, "complete": catalog.Complete, "query": query}, nil
}

func (r *Round) LaunchApplication(ctx context.Context, requestID string, input nexuscua.LaunchApplicationInput, approve func(nexuscua.LaunchableApplication) error) (map[string]any, error) {
	if requestID == "" {
		return nil, errors.New("launch_application 需要宿主 request_id")
	}
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	if result, ok := r.completed[requestID]; ok {
		r.mu.Unlock()
		return cloneActionResult(result).Data, nil
	}
	pending := r.pendingLaunches[requestID]
	app, known := r.launchables[input.LaunchRef]
	blocked := len(r.pending) > 0 || len(r.pendingLaunches) > 0
	r.mu.Unlock()
	client, ok := r.client.(launchClient)
	if !ok {
		return nil, errors.New("Computer Use runtime 不支持启动应用")
	}
	var output nexuscua.ApplicationLaunchOutput
	var err error
	if pending != nil {
		output, err = client.ReconcileLaunch(ctx, pending, defaultCommandTimeout)
	} else {
		if blocked {
			return nil, ErrMutationPending
		}
		if !known {
			return nil, errors.New("launch_ref 已失效，请重新 list_launchable_applications")
		}
		if approve == nil {
			return nil, errors.New("launch_application 缺少宿主授权通道")
		}
		if err := approve(app); err != nil {
			return nil, err
		}
		// Approval can outlive the launch reference. Refresh only the exact
		// approved identity; a changed/ambiguous installation requires approval again.
		fresh, discoverErr := client.DiscoverLaunchableApplications(ctx, defaultCommandTimeout)
		if discoverErr != nil {
			return nil, discoverErr
		}
		if len(fresh.Applications) > 1024 {
			return nil, errors.New("Computer Use launch catalog 超出上限")
		}
		matches := 0
		for _, candidate := range fresh.Applications {
			if candidate.ApplicationID == app.ApplicationID && reflect.DeepEqual(candidate.Provenance, app.Provenance) {
				input.LaunchRef = candidate.LaunchRef
				matches++
			}
		}
		if matches != 1 {
			return nil, errors.New("启动应用身份已变化，请重新发现并授权")
		}
		var request *nexuscua.LaunchRequest
		output, request, err = client.LaunchApplication(ctx, input, defaultCommandTimeout)
		var uncertain *nexuscua.LaunchIndeterminateError
		if errors.As(err, &uncertain) && request != nil {
			r.mu.Lock()
			if r.pendingLaunches == nil {
				r.pendingLaunches = make(map[string]*nexuscua.LaunchRequest)
			}
			r.pendingLaunches[requestID] = request
			r.mu.Unlock()
		}
	}
	if err != nil {
		return nil, err
	}
	result := ActionResult{Data: map[string]any{"application": output.Application, "delivery_mode": output.DeliveryMode, "dispatched": output.Dispatched, "next_action": "select_target"}}
	r.mu.Lock()
	delete(r.pendingLaunches, requestID)
	delete(r.launchables, input.LaunchRef)
	r.rememberActionResultLocked(requestID, result)
	r.mu.Unlock()
	return cloneActionResult(result).Data, nil
}

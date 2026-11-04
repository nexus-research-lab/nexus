package computeruse

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

type catalogClient struct {
	*fakeClient
	installed []nexuscua.LaunchableApplication
}

func (c *catalogClient) DiscoverLaunchableApplications(context.Context, time.Duration) (nexuscua.DiscoverLaunchableApplicationsOutput, error) {
	return nexuscua.DiscoverLaunchableApplicationsOutput{Applications: c.installed, Complete: false}, nil
}

func (c *catalogClient) DiscoverApplications(context.Context, time.Duration) (nexuscua.DiscoverApplicationsOutput, error) {
	apps := make([]nexuscua.DiscoveredApplication, 0, len(c.installed))
	for _, app := range c.installed {
		apps = append(apps, nexuscua.DiscoveredApplication{Name: app.Name, ApplicationID: app.ApplicationID, Provenance: app.Provenance})
	}
	return nexuscua.DiscoverApplicationsOutput{Applications: apps, Complete: false}, nil
}

func (*catalogClient) LaunchApplication(context.Context, nexuscua.LaunchApplicationInput, time.Duration) (nexuscua.ApplicationLaunchOutput, *nexuscua.LaunchRequest, error) {
	panic("catalog discovery must not launch an application")
}

func (*catalogClient) ReconcileLaunch(context.Context, *nexuscua.LaunchRequest, time.Duration) (nexuscua.ApplicationLaunchOutput, error) {
	panic("catalog discovery must not reconcile a mutation")
}

func TestApplicationQueryAvoidsFullCatalogWithoutSelectingOrWideningAuthority(t *testing.T) {
	service, base, _ := newFakeService(t)
	round, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	client := &catalogClient{fakeClient: base}
	for i := range 144 {
		client.installed = append(client.installed, nexuscua.LaunchableApplication{
			LaunchRef: nexuscua.LaunchRef(fmt.Sprintf("launch-%d", i)),
			Name:      fmt.Sprintf("Application %d", i), ApplicationID: fmt.Sprintf("dev.fixture.%d", i),
		})
	}
	bundleID := "com.google.Chrome"
	chrome := nexuscua.LaunchableApplication{
		LaunchRef: "chrome-exact-ref", Name: "Google Chrome", ApplicationID: bundleID,
		ExpiresAt:  "2026-11-03T10:00:00Z",
		Provenance: nexuscua.ApplicationProvenance{Platform: nexuscua.PlatformMacOS, BundleID: &bundleID},
	}
	client.installed[89] = chrome
	round.client = client
	for _, query := range []string{"CHROME", "com.google.chrome"} {
		result, err := round.ListLaunchableApplications(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		apps := result["applications"].([]nexuscua.LaunchableApplication)
		encoded, _ := json.Marshal(result)
		if len(apps) != 1 || !reflect.DeepEqual(apps[0], chrome) || len(encoded) > 4096 || result["complete"] != false {
			t.Fatal("filtered catalog lost exact identity, exceeded a small page, or hid native incompleteness")
		}
		running, err := round.ListApplications(context.Background(), query)
		if err != nil || len(running["applications"].([]map[string]any)) != 1 || running["complete"] != false {
			t.Fatalf("running catalog differs: %v, %v", running, err)
		}
	}
	approved := false
	_, err = round.LaunchApplication(context.Background(), "hidden-app", nexuscua.LaunchApplicationInput{LaunchRef: "launch-0"}, func(nexuscua.LaunchableApplication) error { approved = true; return nil })
	if err == nil || approved || base.openCalls != 0 || base.performCalls != 0 || len(round.launchables) != 1 {
		t.Fatal("query silently selected/granted authority or admitted an undisclosed launch ref")
	}
	for query, want := range map[string]int{"missing": 0, "": 144} {
		result, err := round.ListLaunchableApplications(context.Background(), query)
		if err != nil || len(result["applications"].([]nexuscua.LaunchableApplication)) != want {
			t.Fatalf("query %q did not preserve empty/full discovery behavior: %v", query, err)
		}
	}
}

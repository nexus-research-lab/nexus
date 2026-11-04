// INPUT: explicit local packaged CUA binary and the owned native fixture identity.
// OUTPUT: bounded live evidence for the Nexus service, SDK, image and action loop.
// POS: opt-in integration test; never runs against arbitrary applications or user state.
package computeruse

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestPackagedCUANativeActionLoop(t *testing.T) {
	binary := os.Getenv("NEXUS_CUA_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_CUA_TEST_BINARY and open the owned native fixture")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("NEXUS_CUA_TEST_BINARY must be absolute")
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("", "nexus-cua-live-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	service := NewService(config.Config{
		DatabaseURL:        filepath.Join(root, "nexus.db"),
		ComputerUseEnabled: true, ComputerUseBinaryPath: binary,
	})
	t.Cleanup(func() { _ = service.Close(ctx) })
	round, err := service.Round(ctx, "native-test-owner", "native-test-round")
	if err != nil {
		t.Fatal(err)
	}
	computer, err := round.GetComputer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if computer["runtime_version"] != "0.2.0-rc.5" || computer["observe_after"] != true {
		t.Fatal("packaged runtime version/capabilities do not match pinned SDK")
	}
	approved := false
	fixtureID := os.Getenv("NEXUS_CUA_TEST_APPLICATION_ID")
	if fixtureID == "" {
		fixtureID = "dev.nexus.cua.native-fixture"
	}
	_, err = round.SelectTarget(ctx, fixtureID, TargetOptions{
		Actions: []nexuscua.ActionKind{nexuscua.ActionInvokeElement}, TTLSeconds: 60,
	}, func(grant TargetGrant) error {
		approved = grant.Application.ApplicationID == fixtureID && !grant.AllowForegroundInput
		return nil
	})
	if err != nil || !approved {
		t.Fatalf("bounded fixture selection: %v", err)
	}
	observed, err := round.Observe(ctx, ObserveInput{
		WindowSelector: "Generation", IncludeScreenshot: true, Accessibility: "full",
	})
	if err != nil {
		t.Fatal(err)
	}
	if observed.ImageData == "" || observed.ImageMIME != "image/png" {
		t.Fatal("initial image missing")
	}
	findButton := func(data map[string]any) string {
		for _, element := range data["elements"].([]map[string]any) {
			if element["name"] == "Increment Counter" {
				return string(element["element_ref"].(nexuscua.ElementRef))
			}
		}
		t.Fatal("owned fixture button missing")
		return ""
	}
	for _, id := range []string{"native-first", "native-second"} {
		result, err := round.PerformAction(ctx, id, ActionInput{Kind: "invoke_element", ElementRef: findButton(observed.Data)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Data["dispatched"] != true || result.Data["observation_error"] != nil || result.ImageData == "" {
			t.Fatal("action did not return successful evidence and a fresh image")
		}
		after, ok := result.Data["observation"].(map[string]any)
		if !ok || after["observation_id"] == observed.Data["observation_id"] {
			t.Fatal("guard did not advance")
		}
		if shot, ok := after["screenshot"].(map[string]any); !ok || shot["path"] != nil {
			t.Fatal("screenshot missing or leaked its private path")
		}
		observed = Observation{Data: after}
	}
	if _, err := round.PerformAction(ctx, "native-first", ActionInput{Kind: "invoke_element", ElementRef: findButton(observed.Data)}); err != nil {
		t.Fatal(err)
	}
	if err := service.CloseRound(ctx, "native-test-owner", "native-test-round"); err != nil {
		t.Fatal(err)
	}
	if round.HasSession() {
		t.Fatal("round close retained native authority")
	}
	if _, err := round.Observe(ctx, ObserveInput{}); err == nil || !strings.Contains(err.Error(), "关闭") {
		t.Fatal("closed round admitted a read")
	}
	t.Log("pinned runtime, bounded host approval, image delivery, two chained actions, replay and close verified")
}

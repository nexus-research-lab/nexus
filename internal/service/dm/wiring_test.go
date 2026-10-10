package dm

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestRequireWiringNamesEveryMissingDependency(t *testing.T) {
	err := NewService(config.Config{WorkspacePath: t.TempDir()}, nil, nil, nil).RequireWiring()
	if err == nil {
		t.Fatal("unwired DM service must fail")
	}
	for _, name := range []string{"agents", "goal context provider", "IM delivery store"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("missing %q in %v", name, err)
		}
	}
}

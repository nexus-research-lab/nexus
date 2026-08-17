package runtime

import (
	"context"
	"errors"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

type cacheSurfaceMCPServer struct {
	description string
}

type cacheSurfacePanicServer struct{}

func (cacheSurfacePanicServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	panic("must not escape cache observability")
}

type cacheSurfaceErrorServer struct{}

func (cacheSurfaceErrorServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return nil, errors.New("unavailable")
}

type cacheSurfaceBlockingServer struct{}

func (cacheSurfaceBlockingServer) HandleMessage(ctx context.Context, _ map[string]any) (map[string]any, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s cacheSurfaceMCPServer) HandleMessage(
	_ context.Context,
	request map[string]any,
) (map[string]any, error) {
	if request["method"] != "tools/list" {
		return map[string]any{}, nil
	}
	return map[string]any{
		"result": map[string]any{
			"tools": []map[string]any{{
				"name":        "update_goal",
				"description": s.description,
				"inputSchema": map[string]any{"type": "object"},
			}},
		},
	}, nil
}

func TestCacheSurfaceFingerprintsModelVisibleSDKToolSchema(t *testing.T) {
	options := agentclient.Options{
		CLIPath: "/test/nxs",
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
		Model:   "model-secret-name",
		Env: map[string]string{
			runtimeProviderEnvName: "provider-secret-name",
			"NEXUS_CONFIG_DIR":     t.TempDir(),
		},
		MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
			managedGoalMCPServerName: sdkmcp.SDKServerConfig{
				Name:     managedGoalMCPServerName,
				Instance: cacheSurfaceMCPServer{description: "first private schema text"},
			},
		}},
	}
	first, err := cacheSurfaceProfileFromOptions(context.Background(), options)
	if err != nil {
		t.Fatalf("cacheSurfaceProfileFromOptions() error = %v", err)
	}
	exportedFingerprint, complete, err := ModelToolSurfaceFingerprint(context.Background(), options)
	if err != nil {
		t.Fatalf("ModelToolSurfaceFingerprint() error = %v", err)
	}
	if !complete {
		t.Fatal("Nexus SDK-only tool surface should be complete through exported helper")
	}
	if exportedFingerprint != first.ToolSurfaceFingerprint {
		t.Fatalf("导出工具面指纹 = %q, want %q", exportedFingerprint, first.ToolSurfaceFingerprint)
	}
	if !first.GoalToolSurfacePresent || first.ExecutionToolSurfacePresent {
		t.Fatalf("tool presence = goal:%v execution:%v", first.GoalToolSurfacePresent, first.ExecutionToolSurfacePresent)
	}
	if !first.HostToolSurfaceComplete {
		t.Fatal("Nexus SDK-only tool surface should be complete")
	}
	for name, value := range map[string]string{
		"provider": first.ProviderFingerprint,
		"model":    first.ModelFingerprint,
		"surface":  first.ToolSurfaceFingerprint,
	} {
		if len(value) != 64 {
			t.Fatalf("%s fingerprint length = %d, want 64", name, len(value))
		}
		if value == "provider-secret-name" || value == "model-secret-name" || value == "first private schema text" {
			t.Fatalf("%s persisted plaintext: %q", name, value)
		}
	}

	options.MCP.Servers[managedGoalMCPServerName] = sdkmcp.SDKServerConfig{
		Name:     managedGoalMCPServerName,
		Instance: cacheSurfaceMCPServer{description: "changed private schema text"},
	}
	second, err := cacheSurfaceProfileFromOptions(context.Background(), options)
	if err != nil {
		t.Fatalf("cacheSurfaceProfileFromOptions() second error = %v", err)
	}
	if first.MCPServersFingerprint != second.MCPServersFingerprint {
		t.Fatal("bridge config fingerprint unexpectedly includes SDK schema")
	}
	if first.ToolSurfaceFingerprint == second.ToolSurfaceFingerprint {
		t.Fatal("model-visible SDK schema change must alter tool surface fingerprint")
	}
	options.MCP.Servers[managedGoalMCPServerName] = sdkmcp.SDKServerConfig{
		Name:     managedGoalMCPServerName,
		Instance: cacheSurfaceMCPServer{description: "changed private schema text"},
	}
	options.Tools.Available = []string{"Read"}
	third, err := cacheSurfaceProfileFromOptions(context.Background(), options)
	if err != nil {
		t.Fatalf("cacheSurfaceProfileFromOptions() third error = %v", err)
	}
	if second.ToolSurfaceFingerprint == third.ToolSurfaceFingerprint {
		t.Fatal("native model-visible tool set change must alter tool surface fingerprint")
	}
}

func TestCacheSurfaceMCPFingerprintExcludesEndpointAndSecrets(t *testing.T) {
	first, err := cacheSurfaceProfileFromOptions(context.Background(), agentclient.Options{
		MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
			"search": sdkmcp.HTTPServerConfig{
				URL:     "https://first.example/mcp",
				Headers: map[string]string{"Authorization": "secret-one"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("first profile error = %v", err)
	}
	second, err := cacheSurfaceProfileFromOptions(context.Background(), agentclient.Options{
		MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
			"search": sdkmcp.HTTPServerConfig{
				URL:     "https://second.example/mcp",
				Headers: map[string]string{"Authorization": "secret-two"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("second profile error = %v", err)
	}
	if first.MCPServersFingerprint != second.MCPServersFingerprint {
		t.Fatal("endpoint or credential rotation must not create high-cardinality MCP buckets")
	}
	if first.ToolSurfaceFingerprint != second.ToolSurfaceFingerprint {
		t.Fatal("uninspectable endpoint details must not affect host tool surface fingerprint")
	}
	if first.HostToolSurfaceComplete || second.HostToolSurfaceComplete {
		t.Fatal("external tool schemas must remain explicitly incomplete")
	}
}

func TestCacheSurfaceMarksUninspectableExternalMCPIncomplete(t *testing.T) {
	profile, err := cacheSurfaceProfileFromOptions(context.Background(), agentclient.Options{
		CLIPath: "/test/nxs",
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
		Env:     map[string]string{"NEXUS_CONFIG_DIR": t.TempDir()},
		MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
			managedExecutionMCPServerName: sdkmcp.HTTPServerConfig{URL: "https://example.invalid/mcp"},
		}},
	})
	if err != nil {
		t.Fatalf("cacheSurfaceProfileFromOptions() error = %v", err)
	}
	if !profile.ExecutionToolSurfacePresent {
		t.Fatal("execution server presence should be host-observable")
	}
	if profile.HostToolSurfaceComplete {
		t.Fatal("external MCP schema cannot be claimed complete before connection")
	}
	if len(profile.ToolSurfaceFingerprint) != 64 {
		t.Fatalf("partial surface fingerprint length = %d, want 64", len(profile.ToolSurfaceFingerprint))
	}
}

func TestCacheSurfaceInspectionFailureIsContained(t *testing.T) {
	for name, server := range map[string]sdkmcp.SDKMCPServer{
		"panic":    cacheSurfacePanicServer{},
		"error":    cacheSurfaceErrorServer{},
		"blocking": cacheSurfaceBlockingServer{},
	} {
		t.Run(name, func(t *testing.T) {
			options := agentclient.Options{
				CLIPath: "/test/nxs",
				Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
				Env:     map[string]string{"NEXUS_CONFIG_DIR": t.TempDir()},
				MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
					managedGoalMCPServerName: sdkmcp.SDKServerConfig{
						Name: managedGoalMCPServerName, Instance: server,
					},
				}},
			}
			profile, err := cacheSurfaceProfileFromOptions(context.Background(), options)
			if err != nil {
				t.Fatalf("cacheSurfaceProfileFromOptions() error = %v", err)
			}
			if profile.HostToolSurfaceComplete {
				t.Fatalf("inspection %s must mark surface incomplete", name)
			}
			if !profile.GoalToolSurfacePresent || profile.ToolSurfaceFingerprint == "" {
				t.Fatalf("partial profile = %+v", profile)
			}
			if _, complete, fingerprintErr := ModelToolSurfaceFingerprint(context.Background(), options); fingerprintErr != nil || complete {
				t.Fatalf("inspection %s exported completeness = %v err=%v, want incomplete without error", name, complete, fingerprintErr)
			}
		})
	}
}

func TestCacheSurfaceDoesNotSwallowCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cacheSurfaceProfileFromOptions(ctx, agentclient.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cacheSurfaceProfileFromOptions() error = %v, want context.Canceled", err)
	}
}

func TestManagerCacheSurfaceTracksSuccessfulConfiguration(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:default:cache-surface"
	options := agentclient.Options{
		CLIPath: "/test/nxs",
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
		Env:     map[string]string{"NEXUS_CONFIG_DIR": t.TempDir()},
		MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
			managedGoalMCPServerName: sdkmcp.SDKServerConfig{
				Name:     managedGoalMCPServerName,
				Instance: cacheSurfaceMCPServer{description: "goal"},
			},
		}},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	profile, ok := manager.CacheSurface(sessionKey)
	if !ok || !profile.GoalToolSurfacePresent || profile.ToolSurfaceFingerprint == "" {
		t.Fatalf("CacheSurface() = %+v, ok=%v", profile, ok)
	}
}

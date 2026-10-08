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

const cacheSurfaceNexusServerName = "nexus"

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
				"name":        "show_widget",
				"description": s.description,
				"inputSchema": map[string]any{"type": "object"},
			}},
		},
	}, nil
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
					cacheSurfaceNexusServerName: sdkmcp.SDKServerConfig{
						Name: cacheSurfaceNexusServerName, Instance: server,
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
			if profile.ToolSurfaceFingerprint == "" {
				t.Fatalf("partial profile = %+v", profile)
			}
			if _, complete, fingerprintErr := ModelToolSurfaceFingerprint(context.Background(), options); fingerprintErr != nil || complete {
				t.Fatalf("inspection %s exported completeness = %v err=%v, want incomplete without error", name, complete, fingerprintErr)
			}
		})
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
			cacheSurfaceNexusServerName: sdkmcp.SDKServerConfig{
				Name:     cacheSurfaceNexusServerName,
				Instance: cacheSurfaceMCPServer{description: "goal"},
			},
		}},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	profile, ok := manager.CacheSurface(sessionKey)
	if !ok || profile.ToolSurfaceFingerprint == "" {
		t.Fatalf("CacheSurface() = %+v, ok=%v", profile, ok)
	}
}

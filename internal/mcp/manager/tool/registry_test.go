package tool

import (
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	managersvc "github.com/nexus-research-lab/nexus/internal/service/nexusmanager"
)

func TestBuildAllCutsToolsAtAuthorityBoundary(t *testing.T) {
	tests := []struct {
		name string
		sctx contract.ServerContext
		want []string
	}{
		{
			name: "owner main private dm",
			sctx: contract.ServerContext{
				ContextKind: managersvc.ContextKindAgent, IsMainAgent: true,
			},
			want: []string{
				"inspect_nexus_manager",
				"list_nexus_agents", "get_nexus_agent",
				"list_nexus_rooms", "get_nexus_room",
				"list_nexus_conversations", "get_nexus_conversation",
				"list_nexus_sessions", "get_nexus_session",
				"list_nexus_workspace", "read_nexus_workspace_file",
			},
		},
		{
			name: "ordinary agent private dm",
			sctx: contract.ServerContext{ContextKind: managersvc.ContextKindAgent},
			want: []string{
				"inspect_nexus_manager", "list_nexus_workspace", "read_nexus_workspace_file",
			},
		},
		{
			name: "room member even when agent record is main",
			sctx: contract.ServerContext{
				ContextKind: managersvc.ContextKindRoom, IsMainAgent: true,
			},
			want: []string{
				"inspect_nexus_manager", "get_nexus_room", "get_nexus_conversation",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tools := BuildAll(nil, test.sctx)
			got := toolNames(tools)
			if !slices.Equal(got, test.want) {
				t.Fatalf("tools = %+v, want %+v", got, test.want)
			}
			for _, item := range tools {
				if strings.TrimSpace(item.SearchHint) == "" {
					t.Fatalf("%s missing search hint", item.Name)
				}
				if item.Annotations != nil && item.Annotations.Destructive {
					t.Fatalf("%s must not expose destructive semantics", item.Name)
				}
			}
		})
	}
}

func TestSelfAndRoomSchemasCannotOverrideServerScope(t *testing.T) {
	selfTools := BuildAll(nil, contract.ServerContext{
		ContextKind: managersvc.ContextKindAgent,
	})
	for _, name := range []string{"list_nexus_workspace", "read_nexus_workspace_file"} {
		properties := schemaProperties(t, toolByName(t, selfTools, name))
		if _, ok := properties["agent_id"]; ok {
			t.Fatalf("%s self schema must not accept agent_id: %+v", name, properties)
		}
	}

	roomTools := BuildAll(nil, contract.ServerContext{
		ContextKind: managersvc.ContextKindRoom,
	})
	roomProperties := schemaProperties(t, toolByName(t, roomTools, "get_nexus_room"))
	if len(roomProperties) != 0 {
		t.Fatalf("current Room schema must not accept a target: %+v", roomProperties)
	}
	conversationProperties := schemaProperties(
		t,
		toolByName(t, roomTools, "get_nexus_conversation"),
	)
	if len(conversationProperties) != 0 {
		t.Fatalf("current conversation schema must not accept a target: %+v", conversationProperties)
	}
}

func TestOwnerManagerDoesNotDuplicateConfigurationOrHighRiskDomains(t *testing.T) {
	tools := BuildAll(nil, contract.ServerContext{
		ContextKind: managersvc.ContextKindAgent, IsMainAgent: true,
	})
	for _, item := range tools {
		name := strings.ToLower(item.Name)
		for _, forbidden := range []string{
			"delete", "auth", "user", "provider", "channel", "connector", "credential",
			"automation", "update_agent", "create_agent", "write_workspace",
		} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("manager exposed forbidden/duplicated capability %q", item.Name)
			}
		}
	}
}

func toolNames(tools []sdktool.Tool) []string {
	result := make([]string, 0, len(tools))
	for _, item := range tools {
		result = append(result, item.Name)
	}
	return result
}

func toolByName(t *testing.T, tools []sdktool.Tool, name string) sdktool.Tool {
	t.Helper()
	for _, item := range tools {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("missing tool %s", name)
	return sdktool.Tool{}
}

func schemaProperties(t *testing.T, item sdktool.Tool) map[string]any {
	t.Helper()
	properties, ok := item.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s properties have unexpected type %T", item.Name, item.InputSchema["properties"])
	}
	additional, ok := item.InputSchema["additionalProperties"].(bool)
	if !ok || additional {
		t.Fatalf("%s must reject additional properties: %+v", item.Name, item.InputSchema)
	}
	return properties
}

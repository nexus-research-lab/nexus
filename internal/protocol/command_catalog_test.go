package protocol

import "testing"

func TestNewCommandCatalogEventKeepsOnlyPublicDescriptorFields(t *testing.T) {
	event := NewCommandCatalogEvent("agent:a:ws:dm:one", CommandCatalogData{
		Revision:    "revision-1",
		RuntimeKind: "nxs",
		Status:      CommandCatalogStatusReady,
		AgentID:     "a",
		Commands: []CommandDescriptor{{
			Name:         "review",
			Description:  "Review code",
			ArgumentHint: "<target>",
			Execution:    CommandExecutionRuntimePrompt,
			Enabled:      true,
		}},
	})

	if event.EventType != EventTypeCommandCatalog ||
		event.SessionKey != "agent:a:ws:dm:one" ||
		event.AgentID != "a" ||
		event.Data["revision"] != "revision-1" ||
		event.Data["runtime_kind"] != "nxs" {
		t.Fatalf("event = %#v, want scoped command catalog", event)
	}
	commands, ok := event.Data["commands"].([]CommandDescriptor)
	if !ok || len(commands) != 1 || commands[0].Name != "review" {
		t.Fatalf("commands = %#v, want one public descriptor", event.Data["commands"])
	}
}

func TestNewCommandCatalogEventNormalizesNilCommands(t *testing.T) {
	event := NewCommandCatalogEvent("agent:a:ws:dm:one", CommandCatalogData{
		Status: CommandCatalogStatusLoading,
	})

	commands, ok := event.Data["commands"].([]CommandDescriptor)
	if !ok || commands == nil || len(commands) != 0 {
		t.Fatalf("commands = %#v, want non-nil empty slice", event.Data["commands"])
	}
}

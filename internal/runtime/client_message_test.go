// INPUT: Production client options, pinned bridge, and a private process emitting synthetic image frames.
// OUTPUT: Large images reach the consumer; rejected frames close the stream without waiting on a live child.
// POS: Runtime transport and cleanup regression without a provider, desktop input, or user content.
package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	nexusruntime "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
)

const imageProcessFixtureEnv = "NEXUS_TEST_IMAGE_PROCESS_BYTES"

// The test binary acts as a CLI before Go's flag parser sees runtime arguments.
func init() {
	encodedBytes := os.Getenv(imageProcessFixtureEnv)
	if encodedBytes == "" {
		return
	}
	size, err := strconv.Atoi(encodedBytes)
	if err != nil || size < 0 || size > 34*1024*1024 {
		os.Exit(2)
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	for {
		var request map[string]any
		if decoder.Decode(&request) != nil {
			os.Exit(0)
		}
		switch request["type"] {
		case "control_request":
			_ = encoder.Encode(map[string]any{"type": "control_response", "response": map[string]any{
				"subtype": "success", "request_id": request["request_id"],
				"response": map[string]any{"session_id": "image-fixture"},
			}})
		case "user":
			_ = encoder.Encode(map[string]any{"type": "user", "session_id": "image-fixture", "message": map[string]any{
				"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "synthetic-observe", "content": []any{
					map[string]any{"type": "image", "data": strings.Repeat("A", size), "mime_type": "image/png"},
				}}},
			}})
			_ = encoder.Encode(map[string]any{"type": "result", "subtype": "success", "session_id": "image-fixture", "result": "complete", "is_error": false})
		}
	}
}

func TestRuntimeImageMessageBudgetAndRejectedFrameCleanup(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                      string
		imageBytes, overrideLimit int
		wantComplete              bool
	}{
		{"old limit rejects observed size without hanging", 1183472, 1024 * 1024, false},
		{"full CUA PNG budget plus base64 fits", 23 * 1024 * 1024, 0, true},
		{"host limit remains finite", 33 * 1024 * 1024, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, err := clientopts.BuildAgentClientOptions(context.Background(), nil, clientopts.AgentClientOptionsInput{
				WorkspacePath: t.TempDir(), RuntimeKind: "nxs", PermissionMode: sdkpermission.ModeDefault,
				ExtraEnv: map[string]string{imageProcessFixtureEnv: strconv.Itoa(tc.imageBytes)},
			})
			if err != nil {
				t.Fatal(err)
			}
			options.CLIPath = binary
			if tc.overrideLimit != 0 {
				options.MaxBufferSize = tc.overrideLimit
			}
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			client := nexusruntime.NewAgentClient(options)
			defer func() { _ = client.Disconnect(context.Background()) }()
			if err := client.Connect(ctx); err != nil {
				t.Fatal(err)
			}
			messages := client.ReceiveMessages(ctx)
			if err := client.Query(ctx, "synthetic image"); err != nil {
				t.Fatal(err)
			}
			for {
				select {
				case <-ctx.Done():
					t.Fatal("runtime stream remained open after a rejected frame")
				case message, ok := <-messages:
					if !ok {
						streamErr := client.(interface{ StreamError() error }).StreamError()
						if tc.wantComplete || streamErr == nil || !strings.Contains(streamErr.Error(), "maximum buffer size") {
							t.Fatalf("unexpected stream closure: %v", streamErr)
						}
						return
					}
					if message.Type == sdkprotocol.MessageTypeResult {
						if !tc.wantComplete {
							t.Fatal("oversized frame bypassed the finite limit")
						}
						return
					}
				}
			}
		})
	}
}

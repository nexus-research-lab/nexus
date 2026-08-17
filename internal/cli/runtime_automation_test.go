package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/spf13/cobra"
)

type runtimeAutomationRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn runtimeAutomationRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestRuntimeAutomationControllerUsesCapabilityAndStrictEnvelope(t *testing.T) {
	controller := remoteRuntimeAutomationController{
		endpoint: "http://127.0.0.1:8010/internal/runtime/automation",
		token:    "capability",
		client: &http.Client{Transport: runtimeAutomationRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get(protocol.NexusCommandCapabilityHeader) != "capability" {
				t.Fatalf("capability header = %q", request.Header.Get(protocol.NexusCommandCapabilityHeader))
			}
			payload, err := io.ReadAll(request.Body)
			if err != nil || !bytes.Contains(payload, []byte(`"action":"contract"`)) {
				t.Fatalf("request payload = %s err=%v", payload, err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body: io.NopCloser(strings.NewReader(
					`{"success":true,"data":{"query_operations":["list"],"mutation_operations":[],"mutation_allowed":false,"cross_agent_allowed":false,"operations":{}}}`,
				)),
			}, nil
		})},
	}
	contract, err := controller.Contract(context.Background())
	if err != nil {
		t.Fatalf("Contract: %v", err)
	}
	if len(contract.QueryOperations) != 1 || contract.QueryOperations[0] != "list" || contract.MutationAllowed {
		t.Fatalf("contract = %+v", contract)
	}
}

func TestDecodeRuntimeAutomationInputRejectsUnknownIdentityFields(t *testing.T) {
	if _, err := decodeRuntimeAutomationInput(`{"owner_user_id":"forged"}`); err == nil {
		t.Fatal("unknown owner identity field was accepted")
	}
	input, err := decodeRuntimeAutomationInput(`{"name":"task","deliver_result":true}`)
	if err != nil || input.Name != "task" || input.DeliverResult == nil || !*input.DeliverResult {
		t.Fatalf("decoded input = %+v err=%v", input, err)
	}
}

func TestRuntimeAutomationInputDefaultsToManagedRoundFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(inputPath, []byte(`{"name":"引号 ' 与 \" 都保留","instruction":"运行 date '+%Y-%m-%d'"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(protocol.NexusAutomationInputPathEnvName, inputPath)
	command := &cobra.Command{Use: "test"}
	flags := runtimeAutomationFlags{}
	bindRuntimeAutomationFlags(command, &flags)
	input, err := decodeRuntimeAutomationInputForCommand(command, flags)
	if err != nil || input.Name != `引号 ' 与 " 都保留` || input.Instruction != "运行 date '+%Y-%m-%d'" {
		t.Fatalf("managed input = %+v err=%v", input, err)
	}
}

func TestRuntimeAutomationInputRejectsAmbiguousOrUnmanagedFile(t *testing.T) {
	managedPath := filepath.Join(t.TempDir(), "managed.json")
	otherPath := filepath.Join(t.TempDir(), "other.json")
	for _, path := range []string{managedPath, otherPath} {
		if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(protocol.NexusAutomationInputPathEnvName, managedPath)
	command := &cobra.Command{Use: "test"}
	flags := runtimeAutomationFlags{}
	bindRuntimeAutomationFlags(command, &flags)
	if err := command.Flags().Set("input-file", otherPath); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRuntimeAutomationInputForCommand(command, flags); err == nil ||
		!strings.Contains(err.Error(), protocol.NexusAutomationInputPathEnvName) {
		t.Fatalf("unmanaged --input-file error = %v", err)
	}
	if err := command.Flags().Set("input", `{}`); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRuntimeAutomationInputForCommand(command, flags); err == nil ||
		!strings.Contains(err.Error(), "不能同时使用") {
		t.Fatalf("ambiguous input error = %v", err)
	}
}

func TestRuntimeAutomationInputSupportsBoundedStdinAndRejectsSymlink(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.SetIn(strings.NewReader(`{"query":"当前会话"}`))
	flags := runtimeAutomationFlags{}
	bindRuntimeAutomationFlags(command, &flags)
	if err := command.Flags().Set("input-file", "-"); err != nil {
		t.Fatal(err)
	}
	input, err := decodeRuntimeAutomationInputForCommand(command, flags)
	if err != nil || input.Query != "当前会话" {
		t.Fatalf("stdin input = %+v err=%v", input, err)
	}
	if _, err = readLimitedRuntimeAutomationInput(
		strings.NewReader(strings.Repeat("x", maxRuntimeAutomationInputBytes+1)), "stdin",
	); err == nil {
		t.Fatal("oversized stdin was accepted")
	}

	directory := t.TempDir()
	target := filepath.Join(directory, "target.json")
	link := filepath.Join(directory, "input.json")
	if err = os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	t.Setenv(protocol.NexusAutomationInputPathEnvName, link)
	if _, err = readRuntimeAutomationInput(command, link); err == nil {
		t.Fatal("symlink input file was accepted")
	}
}

func TestRuntimeAutomationInputRejectsNonPrivateMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix owner/group/other mode bits")
	}
	inputPath := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(inputPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(inputPath, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(protocol.NexusAutomationInputPathEnvName, inputPath)
	command := &cobra.Command{Use: "test"}
	if _, err := readRuntimeAutomationInput(command, inputPath); err == nil ||
		!strings.Contains(err.Error(), "owner 私有") {
		t.Fatalf("non-private input error = %v", err)
	}
}

var _ runtimeAutomationController = remoteRuntimeAutomationController{}

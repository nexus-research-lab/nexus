package main

import (
	"bytes"
	"os"
	"testing"
)

func TestBuildRootCommandHelpDoesNotRunServer(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	buf := new(bytes.Buffer)
	cmd := buildRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	os.Args = []string{"nexus-server", "--help"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to exit cleanly, got error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected help output, got empty string")
	}
	if bytes.Contains(buf.Bytes(), []byte("run goose up")) {
		t.Fatal("help output unexpectedly contains migration failure")
	}
}

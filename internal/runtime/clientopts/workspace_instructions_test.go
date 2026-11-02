package clientopts

import "testing"

func TestWorkspaceInstructionsGoToNXSLoaderOrFallBackToPrompt(t *testing.T) {
	files := []string{"AGENTS.md", "USER.md"}
	render := func() (string, error) { return "# USER.md\n\nnotes", nil }

	env := workspaceInstructionRuntimeEnv(runtimeKindNXS, files)
	if env[nexusWorkspaceInstructionFilesEnvName] != "AGENTS.md,USER.md" {
		t.Fatalf("nxs env = %#v, want named workspace files", env)
	}
	prompt, err := withWorkspaceInstructionPrompt(runtimeKindNXS, "base", render)
	if err != nil || prompt != "base" {
		t.Fatalf("nxs prompt = %q, %v; workspace files must stay out of the system prompt", prompt, err)
	}

	if env := workspaceInstructionRuntimeEnv(runtimeKindClaude, files); env != nil {
		t.Fatalf("claude env = %#v, want none", env)
	}
	prompt, err = withWorkspaceInstructionPrompt(runtimeKindClaude, "base", render)
	if err != nil || prompt != "base\n\n---\n\n# USER.md\n\nnotes" {
		t.Fatalf("claude prompt = %q, %v; want rendered workspace files appended", prompt, err)
	}
}

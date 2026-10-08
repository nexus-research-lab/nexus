// INPUT: owner 模型选择、完整源结构节点与模型抽象输出。
// OUTPUT: 默认对话模型选择、结构保真提示及关键节点保留校验的回归断言。
// POS: WorkGraph 初始草图抽取契约测试。
package workgraphworkflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type abstractionProviderRecorder struct {
	provider string
	model    string
}

func TestApplyAbstractionRejectsOmittedStructuralNode(t *testing.T) {
	sourceNodes := []protocol.WorkGraphWorkflowNode{
		{LogicalKey: "prepare", Terminal: false},
		{LogicalKey: "review", Terminal: true},
	}
	inputNodes := []AbstractionSourceNode{
		{LogicalKey: "prepare", MustPreserve: true},
		{LogicalKey: "review", MustPreserve: true},
	}
	_, err := applyAbstraction(sourceNodes, inputNodes, completeAbstractionOutput([]AbstractedNode{
		completeAbstractedNode("prepare"),
	}))
	if err == nil || !strings.Contains(err.Error(), "omitted structural node review") {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyAbstractionAllowsOmittingTrulyOptionalIsolatedNode(t *testing.T) {
	sourceNodes := []protocol.WorkGraphWorkflowNode{
		{LogicalKey: "deliver", Required: true, Terminal: true},
		{LogicalKey: "incidental"},
	}
	inputNodes := []AbstractionSourceNode{
		{LogicalKey: "deliver", MustPreserve: true},
		{LogicalKey: "incidental", MustPreserve: false},
	}
	validated, err := applyAbstraction(sourceNodes, inputNodes, completeAbstractionOutput([]AbstractedNode{
		completeAbstractedNode("deliver"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(validated.Nodes) != 1 || validated.Nodes[0].LogicalKey != "deliver" {
		t.Fatalf("validated nodes = %#v", validated.Nodes)
	}
}

func completeAbstractionOutput(nodes []AbstractedNode) AbstractionOutput {
	return AbstractionOutput{
		SlashName: "workflow", Title: "Workflow", Description: "Reusable workflow",
		Objective: "Produce a reusable result", CompletionCriteria: []string{"Result is complete"},
		Nodes: nodes,
	}
}

func completeAbstractedNode(logicalKey string) AbstractedNode {
	return AbstractedNode{
		LogicalKey: logicalKey, Role: protocol.WorkGraphWorkflowNodeKey,
		Subject: "Reusable stage", Objective: "Complete the stage", Deliverable: "Stage output",
		AcceptanceCriteria: []string{"Output is verifiable"},
	}
}

func (r *abstractionProviderRecorder) ResolveLLMConfig(_ context.Context, provider, model string) (*clientopts.RuntimeConfig, error) {
	r.provider = provider
	r.model = model
	return nil, errors.New("stop before model request")
}

type abstractionPreferencesReader struct {
	preferences preferencessvc.Preferences
}

func (r abstractionPreferencesReader) Get(context.Context, string) (preferencessvc.Preferences, error) {
	return r.preferences, nil
}

func TestLLMAbstractorUsesDefaultChatModelInsteadOfBackgroundModel(t *testing.T) {
	providers := &abstractionProviderRecorder{}
	abstractor := NewLLMAbstractor(providers, abstractionPreferencesReader{preferences: preferencessvc.Preferences{
		DefaultAgentOptions: protocol.Options{
			Provider: "default-provider",
			Model:    "default-model",
		},
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}})

	_, err := abstractor.Abstract(context.Background(), "owner-a", AbstractionInput{})
	if err == nil || err.Error() != "stop before model request" {
		t.Fatalf("abstract error = %v", err)
	}
	if providers.provider != "default-provider" || providers.model != "default-model" {
		t.Fatalf("resolved model = %q/%q, want default-provider/default-model", providers.provider, providers.model)
	}
}

package runtime

import "testing"

func TestNormalizeContextualInputBlocksUsesStablePriorityOrder(t *testing.T) {
	blocks := normalizeContextualInputBlocks([]ContextualInputBlock{
		NewContextualInputBlock("zeta", "same", 10, map[string]string{"b": "2"}),
		NewContextualInputBlock("alpha", "same", 10, nil),
		NewContextualInputBlock("later", "high", 20, nil),
	})
	want := []string{"later", "alpha", "zeta"}
	for index, name := range want {
		if blocks[index].Name != name {
			t.Fatalf("blocks[%d].Name = %q, want %q", index, blocks[index].Name, name)
		}
	}
}

package room

import runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

func normalizeRuntimeStderrLine(line string) string {
	return runtimectx.NormalizeRuntimeStderrLine(line)
}

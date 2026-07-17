import type { AgentRuntimeKind } from "@/types/settings/preferences";

export const AGENT_RUNTIME_KIND_OPTIONS: ReadonlyArray<{
  value: AgentRuntimeKind;
  labelKey: "settings.runtime.kernel_claude" | "settings.runtime.kernel_nxs";
}> = [
  { value: "claude", labelKey: "settings.runtime.kernel_claude" },
  { value: "nxs", labelKey: "settings.runtime.kernel_nxs" },
];

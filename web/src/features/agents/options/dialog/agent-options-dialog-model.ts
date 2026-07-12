import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type { AgentOptionsEditorSource } from "../agent-options-editor-model";

export type AgentOptionsDialogState =
  | { kind: "closed" }
  | AgentOptionsEditorSource;

interface AgentOptionsDialogHeader {
  subtitle: string;
  title: string;
}

export function getAgentOptionsDialogHeader(
  source: AgentOptionsEditorSource,
  t: I18nContextValue["t"],
): AgentOptionsDialogHeader {
  if (source.kind === "create") {
    return {
      subtitle: t("agent_options.subtitle_create"),
      title: t("agent_options.title_create"),
    };
  }
  return {
    subtitle: `${t("agent_options.id_prefix")}: ${source.agentId}`,
    title: source.initial.title,
  };
}

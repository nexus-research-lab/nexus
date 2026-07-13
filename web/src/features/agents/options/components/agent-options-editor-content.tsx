import type { ComponentProps, ReactNode } from "react";

import type { AgentOptionsTabKey } from "../agent-options-editor-model";
import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";
import { AgentOptionsIdentityTab } from "./identity/agent-options-identity-tab";
import type { AgentIdentityVariant } from "./identity/identity-layout";
import { AgentOptionsSkillsTab } from "./skills/agent-options-skills-tab";

export interface AgentOptionsEditorContentProps {
  activeTab: AgentOptionsTabKey;
  advanced: ComponentProps<typeof AgentOptionsAdvancedTab>;
  identity: Omit<ComponentProps<typeof AgentOptionsIdentityTab>, "variant">;
  identityVariant: AgentIdentityVariant;
  skills: ComponentProps<typeof AgentOptionsSkillsTab>;
}

type TabRenderer = (props: AgentOptionsEditorContentProps) => ReactNode;

const TAB_RENDERERS: Readonly<Record<AgentOptionsTabKey, TabRenderer>> = {
  advanced: ({ advanced }) => <AgentOptionsAdvancedTab {...advanced} />,
  identity: ({ identity, identityVariant }) => (
    <AgentOptionsIdentityTab {...identity} variant={identityVariant} />
  ),
  skills: ({ skills }) => <AgentOptionsSkillsTab {...skills} />,
};

export function AgentOptionsEditorContent(
  props: AgentOptionsEditorContentProps,
) {
  return TAB_RENDERERS[props.activeTab](props);
}

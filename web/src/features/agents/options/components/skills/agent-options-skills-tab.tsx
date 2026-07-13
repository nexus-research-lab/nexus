import { AgentOptionsSkillsView } from "./agent-options-skills-view";
import { useAgentSkillsController } from "./use-agent-skills-controller";

interface AgentOptionsSkillsTabProps {
  agentId?: string;
  isVisible: boolean;
}

export function AgentOptionsSkillsTab(props: AgentOptionsSkillsTabProps) {
  const controller = useAgentSkillsController(props);
  return <AgentOptionsSkillsView {...controller} />;
}

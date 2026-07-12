import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";
import { pickAgentEditableOptions } from "@/lib/agent-options";

export type AgentOptionsTabKey = "identity" | "skills" | "advanced";

export interface AgentEditorInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
}

export interface AgentOptionsEditorInitialValues {
  avatar: string;
  description: string;
  options: AgentEditorInitialOptions;
  title: string;
  vibeTags: string[];
}

export interface AgentOptionsCreateSource {
  initial: AgentOptionsEditorInitialValues;
  kind: "create";
}

export interface AgentOptionsEditSource {
  agentId: string;
  initial: AgentOptionsEditorInitialValues;
  kind: "edit";
}

export type AgentOptionsEditorSource =
  | AgentOptionsCreateSource
  | AgentOptionsEditSource;

export type AgentOptionsMode = AgentOptionsEditorSource["kind"];

export function buildAgentOptionsCreateSource(
  options: AgentEditorInitialOptions,
): AgentOptionsCreateSource {
  return {
    initial: {
      avatar: "",
      description: "",
      options,
      title: "",
      vibeTags: [],
    },
    kind: "create",
  };
}

export function buildAgentOptionsEditSource(
  agent: Agent,
): AgentOptionsEditSource {
  return {
    agentId: agent.agent_id,
    initial: {
      avatar: agent.avatar ?? "",
      description: agent.description ?? "",
      options: pickAgentEditableOptions(agent.options),
      title: agent.name,
      vibeTags: agent.vibe_tags ?? [],
    },
    kind: "edit",
  };
}

export interface AgentOptionsFormProps {
  isActive: boolean;
  onDelete?: (agentId: string) => void;
  onSave: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void | Promise<void>;
  onValidateName?: (name: string) => Promise<AgentNameValidationResult>;
  showDeleteButton?: boolean;
  source: AgentOptionsEditorSource;
}

export interface AgentOptionsInlineEditorProps extends AgentOptionsFormProps {
  activeTab: AgentOptionsTabKey;
  contentMaxWidthClassName: string;
  onTabChange: (tab: AgentOptionsTabKey) => void;
}

export interface AgentOptionsDialogEditorProps extends AgentOptionsFormProps {
  onCancel: () => void;
}

export interface AgentOptionsControllerOptions extends AgentOptionsFormProps {
  activeTab?: AgentOptionsTabKey;
  onSaveSuccess?: () => void;
  onTabChange?: (tab: AgentOptionsTabKey) => void;
}

export type SaveFeedback = {
  tone: "success" | "error";
  message: string;
};

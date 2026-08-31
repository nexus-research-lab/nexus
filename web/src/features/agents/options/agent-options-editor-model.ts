import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";
import { pickAgentEditableOptions } from "@/lib/agent-options";
import { getRandomAgentAvatarIconId } from "@/lib/avatar";

export type AgentOptionsTabKey = "identity" | "skills" | "advanced";
export type AgentOptionsSaveMode = "automatic" | "explicit";

export interface AgentOptionsPersistenceState {
  message: string;
  phase: "error" | "idle" | "pending" | "saving" | "success";
}

export interface AgentEditorInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
}

export interface AgentOptionsEditorInitialValues {
  avatar: string;
  description: string;
  options: AgentEditorInitialOptions;
  profileTemplate?: string;
  title: string;
  vibeTags: string[];
}

export interface AgentOptionsCreateSource {
  initial: AgentOptionsEditorInitialValues;
  kind: "create";
}

export interface AgentOptionsEditSource {
  agentId: string;
  isMain: boolean;
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
      avatar: getRandomAgentAvatarIconId(),
      description: "",
      options,
      profileTemplate: "",
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
    isMain: agent.is_main === true,
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
  onPersistenceStateChange?: (state: AgentOptionsPersistenceState) => void;
  onTabChange: (tab: AgentOptionsTabKey) => void;
  saveMode?: AgentOptionsSaveMode;
}

export interface AgentOptionsDialogEditorProps extends AgentOptionsFormProps {
  onCancel: () => void;
}

export interface AgentOptionsControllerOptions extends AgentOptionsFormProps {
  activeTab?: AgentOptionsTabKey;
  onSaveSuccess?: () => void;
  onTabChange?: (tab: AgentOptionsTabKey) => void;
  saveMode?: AgentOptionsSaveMode;
}

export type SaveFeedback =
  | {
    tone: "success";
    message: string;
  }
  | {
    blocksRepeat: boolean;
    tone: "error" | "warning";
    message: string;
    impact: string;
    nextStep: string;
  };

import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";

export type AgentOptionsMode = "create" | "edit";
export type AgentOptionsTabKey = "identity" | "skills" | "advanced";

export interface AgentOptionsFormProps {
  agentId?: string;
  mode: AgentOptionsMode;
  isActive: boolean;
  onDelete?: (agentId: string) => void;
  onSave: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void | Promise<void>;
  onValidateName?: (name: string) => Promise<AgentNameValidationResult>;
  initialTitle?: string;
  initialOptions?: Partial<AgentConfigOptions>;
  initialAvatar?: string;
  initialDescription?: string;
  initialVibeTags?: string[];
  showDeleteButton?: boolean;
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

export interface AgentEditorInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
}

export type SaveFeedback = {
  tone: "success" | "error";
  message: string;
};

import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";
import type { TabKey } from "@/features/agents/options/components/agent-options-nav";

export interface AgentOptionsEditorProps {
  agentId?: string;
  mode: "create" | "edit";
  isActive: boolean;
  onDelete?: (agentId: string) => void;
  onSave: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void | Promise<void>;
  onValidateName?: (name: string) => Promise<AgentNameValidationResult>;
  initialTitle?: string;
  initialOptions?: Partial<AgentConfigOptions>;
  initialAvatar?: string;
  initialDescription?: string;
  initialVibeTags?: string[];
  onCancel?: () => void;
  closeAfterSave?: boolean;
  showCancelButton?: boolean;
  showDeleteButton?: boolean;
  variant?: "dialog" | "inline";
  contentMaxWidthClassName?: string;
  activeTab?: TabKey;
  onTabChange?: (tab: TabKey) => void;
  hideInlineNav?: boolean;
}

export interface AgentDialogInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
}

export type SaveFeedback = {
  tone: "success" | "error";
  message: string;
};

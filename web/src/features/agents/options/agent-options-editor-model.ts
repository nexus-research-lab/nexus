import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";
import type { TabKey } from "@/features/agents/options/components/agent-options-nav";

export interface AgentOptionsEditorProps {
  agent_id?: string;
  mode: "create" | "edit";
  is_active: boolean;
  on_delete?: (agent_id: string) => void;
  on_save: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void | Promise<void>;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<AgentConfigOptions>;
  initial_avatar?: string;
  initial_description?: string;
  initial_vibe_tags?: string[];
  on_cancel?: () => void;
  close_after_save?: boolean;
  show_cancel_button?: boolean;
  show_delete_button?: boolean;
  variant?: "dialog" | "inline";
  content_max_width_class_name?: string;
  active_tab?: TabKey;
  on_tab_change?: (tab: TabKey) => void;
  hide_inline_nav?: boolean;
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

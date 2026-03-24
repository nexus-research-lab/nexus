import { CSSProperties, ReactNode } from "react";

import { Agent, AgentNameValidationResult } from "@/types/agent";
import { PermissionRiskLevel, PermissionUpdate } from "@/types/permission";
import { SessionOptions } from "@/types/session";

export interface ConfirmDialogProps {
  is_open: boolean;
  title: string;
  message: string;
  confirm_text?: string;
  cancel_text?: string;
  on_confirm: () => void;
  on_cancel: () => void;
  variant?: "danger" | "default";
}

export interface PromptDialogProps {
  is_open: boolean;
  title: string;
  message?: string;
  placeholder?: string;
  default_value?: string;
  on_confirm: (value: string) => void;
  on_cancel: () => void;
}

export interface PermissionDialogProps {
  is_open: boolean;
  tool_name: string;
  tool_input: Record<string, any>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  on_allow: (updated_permissions?: PermissionUpdate[]) => void;
  on_deny: (updated_permissions?: PermissionUpdate[]) => void;
  on_close: () => void;
}

export interface LottiePlayerProps {
  src: string;
  class_name?: string;
  inline_style?: CSSProperties;
}

export interface RouteScaffoldProps {
  badge: string;
  title: string;
  description: string;
  meta?: ReactNode;
  children?: ReactNode;
  class_name?: string;
}

export interface AppStageProps {
  children: ReactNode;
}

export interface AgentSwitcherProps {
  agents: Agent[];
  current_agent_id: string | null;
  recent_agents: Agent[];
  on_select_agent: (agent_id: string) => void;
  on_open_directory: () => void;
  on_create_agent: () => void;
}

export interface AgentDialogInitialOptions extends Partial<SessionOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
  skills_enabled?: boolean;
  setting_sources?: ("user" | "project" | "local")[];
}

export interface AgentOptionsProps {
  mode: "create" | "edit";
  is_open: boolean;
  on_close: () => void;
  on_save: (title: string, options: SessionOptions) => void;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<SessionOptions>;
}

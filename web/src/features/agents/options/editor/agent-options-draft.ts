import type {
  AgentIdentityDraft,
  AgentOptions,
  AgentProvider,
} from "@/types/agent/agent";

import {
  DEFAULT_AGENT_OPTION_MODEL,
  DEFAULT_AGENT_OPTION_PROVIDER,
  DEFAULT_AGENT_PERMISSION_MODE,
  normalizeAgentAllowedToolsForEditor,
  normalizeAgentOptionProvider,
} from "@/lib/agent-options";
import type {
  AgentEditorInitialOptions,
  AgentOptionsFormProps,
} from "../agent-options-editor-model";

export interface AgentOptionsDraft {
  allowedTools: string[];
  avatar: string;
  description: string;
  disallowedTools: string[];
  model: string;
  permissionMode: string;
  provider: AgentProvider;
  title: string;
  vibeTags: string[];
}

export interface AgentOptionsSubmission {
  identity: AgentIdentityDraft;
  options: AgentOptions;
  title: string;
}

interface CreateAgentOptionsDraftOptions {
  defaultTitle: string;
  initialAvatar: string;
  initialDescription: string;
  initialOptions: AgentEditorInitialOptions;
  initialTitle: string;
  initialVibeTags: string[];
}

interface AgentEditorScopeOptions {
  draft: AgentOptionsDraft;
  initialOptions: AgentEditorInitialOptions;
  props: Pick<AgentOptionsFormProps, "agentId" | "isActive" | "mode">;
}

export function createAgentOptionsDraft({
  defaultTitle,
  initialAvatar,
  initialDescription,
  initialOptions,
  initialTitle,
  initialVibeTags,
}: CreateAgentOptionsDraftOptions): AgentOptionsDraft {
  const model = initialOptions.model?.trim() || DEFAULT_AGENT_OPTION_MODEL;
  return {
    allowedTools: normalizeAgentAllowedToolsForEditor(initialOptions.allowed_tools),
    avatar: initialAvatar,
    description: initialDescription,
    disallowedTools: initialOptions.disallowed_tools ?? [],
    model,
    permissionMode: initialOptions.permission_mode || DEFAULT_AGENT_PERMISSION_MODE,
    provider: model
      ? normalizeAgentOptionProvider(initialOptions.provider) || DEFAULT_AGENT_OPTION_PROVIDER
      : DEFAULT_AGENT_OPTION_PROVIDER,
    title: initialTitle || defaultTitle,
    vibeTags: initialVibeTags,
  };
}

export function buildAgentEditorScopeKey({
  draft,
  initialOptions,
  props,
}: AgentEditorScopeOptions): string {
  return JSON.stringify({
    agentId: props.agentId ?? null,
    draft,
    initialOptions,
    isActive: props.isActive,
    mode: props.mode,
  });
}

export function buildAgentOptionsSubmission(
  draft: AgentOptionsDraft,
  sourceOptions: AgentEditorInitialOptions,
): AgentOptionsSubmission {
  const provider = draft.provider.trim();
  const model = draft.model.trim();
  const hasExplicitModel = Boolean(provider && model);
  return {
    identity: {
      avatar: draft.avatar,
      description: draft.description.trim(),
      vibe_tags: draft.vibeTags,
    },
    options: {
      provider: hasExplicitModel ? provider : DEFAULT_AGENT_OPTION_PROVIDER,
      model: hasExplicitModel ? model : DEFAULT_AGENT_OPTION_MODEL,
      permission_mode: draft.permissionMode,
      allowed_tools: normalizeAgentAllowedToolsForEditor(draft.allowedTools),
      disallowed_tools: draft.disallowedTools,
      max_turns: sourceOptions.max_turns,
      max_thinking_tokens: sourceOptions.max_thinking_tokens,
      mcp_servers: sourceOptions.mcp_servers,
      setting_sources: ["project"],
    },
    title: draft.title.trim(),
  };
}

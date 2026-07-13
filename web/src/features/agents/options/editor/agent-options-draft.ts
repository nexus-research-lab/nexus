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
  AgentOptionsEditorInitialValues,
  AgentOptionsEditorSource,
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
  initial: AgentOptionsEditorInitialValues;
}

interface AgentEditorScopeOptions {
  draft: AgentOptionsDraft;
  isActive: boolean;
  source: AgentOptionsEditorSource;
}

export function createAgentOptionsDraft({
  defaultTitle,
  initial,
}: CreateAgentOptionsDraftOptions): AgentOptionsDraft {
  const model = initial.options.model?.trim() || DEFAULT_AGENT_OPTION_MODEL;
  return {
    allowedTools: normalizeAgentAllowedToolsForEditor(initial.options.allowed_tools),
    avatar: initial.avatar,
    description: initial.description,
    disallowedTools: initial.options.disallowed_tools ?? [],
    model,
    permissionMode: initial.options.permission_mode || DEFAULT_AGENT_PERMISSION_MODE,
    provider: resolveInitialProvider(model, initial.options.provider),
    title: initial.title || defaultTitle,
    vibeTags: initial.vibeTags,
  };
}

function resolveInitialProvider(
  model: string,
  provider: AgentProvider | undefined,
): AgentProvider {
  if (!model) {
    return DEFAULT_AGENT_OPTION_PROVIDER;
  }
  return normalizeAgentOptionProvider(provider) || DEFAULT_AGENT_OPTION_PROVIDER;
}

export function buildAgentEditorScopeKey({
  draft,
  isActive,
  source,
}: AgentEditorScopeOptions): string {
  return JSON.stringify({
    draft,
    isActive,
    source,
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

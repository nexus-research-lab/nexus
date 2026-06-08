/**
 * # !/usr/bin/env ts
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：provider.ts
 * # @Date   ：2026/04/14 14:54
 * # @Author ：leemysw
 * # 2026/04/14 14:54   Create
 * # =====================================================
 */

export type ProviderApiFormat =
  | "chat_completions"
  | "responses"
  | "anthropic_messages"
  | "openai_image_generation"
  | "dashscope_image_generation"
  | "modelscope_image_generation";
export type ProviderKind = "llm" | "image_generation";

export interface ProviderModelCapabilities {
  vision?: boolean;
  image_output?: boolean;
  tool_calling?: boolean;
  reasoning?: boolean;
  embedding?: boolean;
}

export interface ProviderModelRecord {
  id: string;
  provider_id: string;
  model_id: string;
  display_name: string;
  category: string;
  enabled: boolean;
  is_default: boolean;
  capabilities_auto: ProviderModelCapabilities;
  capabilities_override: ProviderModelCapabilities;
  context_window?: number | null;
  max_output_tokens?: number | null;
  provider_options: Record<string, unknown>;
  last_seen_at?: string | null;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface ProviderConfigRecord {
  id: string;
  owner_user_id?: string;
  visibility: "public" | "private";
  provider_kind: ProviderKind;
  provider: string;
  preset_key: string;
  api_format: ProviderApiFormat;
  display_name: string;
  auth_token_masked: string;
  base_url: string;
  models_path: string;
  enabled: boolean;
  usage_count: number;
  used_by_agents: ProviderUsageAgent[];
  last_test_status: string;
  last_test_error: string;
  last_test_at?: string | null;
  can_manage: boolean;
  agent_runtime_supported: boolean;
  models: ProviderModelRecord[];
  created_at?: string | null;
  updated_at?: string | null;
}

export interface ProviderUsageAgent {
  agent_id: string;
  name: string;
  display_name: string;
  avatar?: string;
  is_main?: boolean;
}

export interface ProviderPresetFormat {
  provider_kind?: ProviderKind;
  api_format: ProviderApiFormat;
  base_url: string;
  models_path: string;
}

export interface ProviderPreset {
  preset_key: string;
  provider_kind: ProviderKind;
  display_name: string;
  description: string;
  key_url: string;
  default_api_format: ProviderApiFormat;
  formats: ProviderPresetFormat[];
}

export interface ProviderOption {
  provider: string;
  display_name: string;
  models: ProviderModelOption[];
}

export interface ProviderModelOption {
  model_id: string;
  display_name: string;
  is_default: boolean;
}

export interface ProviderModelSelection {
  provider: string;
  provider_display_name: string;
  model: string;
  model_display_name: string;
}

export interface ProviderOptionsResponse {
  default_provider: string | null;
  default_model: string | null;
  default_selection: ProviderModelSelection | null;
  default_image_provider: string | null;
  default_image_model: string | null;
  default_image_selection: ProviderModelSelection | null;
  items: ProviderOption[];
  background_items: ProviderOption[];
  image_items: ProviderOption[];
}

export interface ProviderConfigPayload {
  provider_kind: ProviderKind;
  provider: string;
  visibility?: "public" | "private";
  preset_key?: string;
  api_format?: ProviderApiFormat;
  display_name: string;
  auth_token: string;
  base_url: string;
  models_path?: string;
  enabled: boolean;
}

export interface UpdateProviderConfigPayload {
  provider_kind?: ProviderKind;
  preset_key?: string;
  api_format?: ProviderApiFormat;
  display_name: string;
  auth_token?: string;
  base_url: string;
  models_path?: string;
  enabled: boolean;
}

export interface FetchProviderModelsResponse {
  provider: string;
  models: ProviderModelRecord[];
  count: number;
}

export interface UpdateProviderModelPayload {
  enabled: boolean;
  is_default?: boolean;
  capabilities_override: ProviderModelCapabilities;
  context_window?: number | null;
  max_output_tokens?: number | null;
  provider_options: Record<string, unknown>;
}

export interface ProviderTestResult {
  provider: string;
  model?: string;
  success: boolean;
  status: string;
  error?: string;
  tested_at?: string | null;
}

export function format_provider_label(provider?: string | null, display_name?: string | null): string {
  const normalized_display_name = display_name?.trim();
  if (normalized_display_name) {
    return normalized_display_name;
  }

  const normalized_provider = provider?.trim();
  if (!normalized_provider) {
    return "Provider";
  }

  return normalized_provider
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

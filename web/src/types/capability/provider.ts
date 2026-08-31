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
export type ProviderEndpointMode = "fixed" | "resource" | "custom";

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
  /** Server-owned monotonic aggregate revision used only for conditional writes. */
  configuration_version: number;
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
  base_url_placeholder?: string;
  models_path: string;
}

export interface ProviderPreset {
  preset_key: string;
  provider_kind: ProviderKind;
  endpoint_mode: ProviderEndpointMode;
  display_name: string;
  description: string;
  key_url: string;
  default_api_format: ProviderApiFormat;
  formats: ProviderPresetFormat[];
}

export interface ProviderOption {
  provider: string;
  display_name: string;
  visibility?: "public" | "private";
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
  vision_items: ProviderOption[];
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
  configuration_version: number;
}

export type CCSwitchAppType = "claude" | "codex";
export type CCSwitchProviderStatus = "ready" | "incomplete" | "unsupported";

export interface CCSwitchModelPreview {
  model_id: string;
  display_name: string;
  context_window?: number | null;
  capabilities?: string[];
}

export interface CCSwitchProviderPreview {
  source_key: string;
  app_type: CCSwitchAppType;
  name: string;
  provider: string;
  api_format?: string;
  base_url?: string;
  current: boolean;
  existing: boolean;
  can_sync: boolean;
  current_runtime_supported: boolean;
  status: CCSwitchProviderStatus;
  reason?: string;
  runtime_support?: string[];
  default_model?: string;
  models: CCSwitchModelPreview[];
}

export interface CCSwitchPreview {
  detected: boolean;
  config_dir: string;
  database_path: string;
  schema_version?: number;
  provider_count: number;
  ready_count: number;
  model_count: number;
  needs_default: boolean;
  recommended_source?: string;
  providers: CCSwitchProviderPreview[];
}

export interface CCSwitchSyncPayload {
  config_dir?: string;
  source_keys: string[];
  set_default?: boolean;
}

export interface CCSwitchSyncResult {
  created: number;
  updated: number;
  provider_count: number;
  model_count: number;
  default_selection?: ProviderModelSelection;
}

export function formatProviderLabel(provider?: string | null, displayName?: string | null): string {
  const normalizedDisplayName = displayName?.trim();
  if (normalizedDisplayName) {
    return normalizedDisplayName;
  }

  const normalizedProvider = provider?.trim();
  if (!normalizedProvider) {
    return "Provider";
  }

  return normalizedProvider
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

export function formatProviderOptionLabel(
  option: Pick<ProviderOption, "display_name" | "provider" | "visibility">,
  subscriptionLabel: string,
): string {
  const providerLabel = option.display_name || formatProviderLabel(option.provider);
  if (option.visibility !== "public") {
    return providerLabel;
  }
  return `${subscriptionLabel} · ${providerLabel}`;
}

/**
 * # !/usr/bin/env tsx
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：provider-settings-panel.tsx
 * # @Date   ：2026/04/14 21:57
 * # @Author ：leemysw
 * # 2026/04/14 21:57   Create
 * # =====================================================
 */

"use client";

import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Brain,
  Cable,
  Database,
  ExternalLink,
  Eye,
  Image,
  ListPlus,
  Loader2,
  Play,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Trash2,
  Wrench,
} from "lucide-react";

import { invalidate_provider_availability } from "@/hooks/capability/use-provider-availability";
import {
  create_provider_config_api,
  delete_provider_config_api,
  fetch_provider_models_api,
  list_provider_configs_api,
  list_provider_presets_api,
  test_provider_config_api,
  test_provider_model_api,
  update_provider_config_api,
  update_provider_model_api,
} from "@/lib/api/provider-config-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { UiField, UiInput, UiSearchInput, UiTextarea } from "@/shared/ui/form-control";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
  ProviderModelCapabilities,
  ProviderModelRecord,
  ProviderPreset,
  ProviderPresetFormat,
  UpdateProviderConfigPayload,
  UpdateProviderModelPayload,
} from "@/types/capability/provider";

import { ProviderIcon } from "./provider-settings/provider-settings-icon";

type SettingsTabKey = "providers";
type FeedbackTone = "success" | "error";
type FormMode = "empty" | "create" | "edit";

interface FeedbackState {
  tone: FeedbackTone;
  title: string;
  message: string;
}

interface ProviderDraft {
  provider_kind: ProviderKind;
  provider: string;
  preset_key: string;
  api_format: ProviderApiFormat;
  display_name: string;
  auth_token: string;
  base_url: string;
  models_path: string;
  enabled: boolean;
}

interface ProviderSettingsPanelProps {
  embedded?: boolean;
}

interface ModelOptionsState {
  model: ProviderModelRecord;
  capabilities: ProviderModelCapabilities;
  context_window: string;
  max_output_tokens: string;
  provider_options_text: string;
}

const SETTINGS_TABS: { key: SettingsTabKey; label_key: "settings.tabs.providers" }[] = [
  { key: "providers", label_key: "settings.tabs.providers" },
];

const PROVIDER_LABEL_CLASS_NAME = "text-[13px] font-semibold text-(--text-strong)";

const API_FORMAT_LABELS: Record<ProviderApiFormat, string> = {
  chat_completions: "Chat Completions (/chat/completions)",
  responses: "Responses (/responses)",
  anthropic_messages: "Anthropic Messages (/v1/messages)",
  openai_image_generation: "OpenAI Image Generation (/images/generations)",
  dashscope_image_generation: "DashScope Image Generation",
  modelscope_image_generation: "ModelScope Image Generation",
};

const API_FORMAT_SHORT_LABELS: Record<ProviderApiFormat, string> = {
  chat_completions: "Completions",
  responses: "Responses",
  anthropic_messages: "Anthropic",
  openai_image_generation: "OpenAI Image",
  dashscope_image_generation: "DashScope Image",
  modelscope_image_generation: "ModelScope Image",
};

const AUTO_TEST_MODEL_VALUE = "__auto__";
const DEFAULT_AGENT_API_FORMAT: ProviderApiFormat = "anthropic_messages";
const SUPPORTED_AGENT_API_FORMATS = new Set<ProviderApiFormat>([
  "anthropic_messages",
  "chat_completions",
]);
const SUPPORTED_IMAGE_API_FORMATS = new Set<ProviderApiFormat>([
  "chat_completions",
  "openai_image_generation",
  "dashscope_image_generation",
  "modelscope_image_generation",
]);
const CONFIGURABLE_NON_RUNTIME_PRESET_KEYS = new Set(["custom", "openai"]);

const PRESET_PROVIDER_KEYS: Record<string, string> = {
  anthropic: "anthropic",
  openai: "openai",
  deepseek: "deepseek",
  "qwen-token-plan": "qwen-token-plan",
  "minimax-token-plan": "minimax-token-plan",
  "glm-coding-plan": "glm-coding-plan",
  "kimi-code": "kimi-code",
  "volcengine-coding-plan": "volcengine-coding-plan",
  doubao: "doubao",
  dashscope: "dashscope",
  modelscope: "modelscope",
  "azure": "azure",
};

function get_preset_provider_key(preset: ProviderPreset): string {
  return PRESET_PROVIDER_KEYS[preset.preset_key] ?? "";
}

function get_preset_format(preset: ProviderPreset | null, api_format?: ProviderApiFormat): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const target_format = api_format ?? preset.default_api_format;
  return preset.formats.find((item) => item.api_format === target_format) ?? preset.formats[0] ?? null;
}

function format_supports_provider_kind(format: ProviderPresetFormat, provider_kind: ProviderKind): boolean {
  if (format.provider_kind) {
    return format.provider_kind === provider_kind;
  }
  return api_format_supported_for_kind(provider_kind, format.api_format);
}

function get_supported_preset_format(preset: ProviderPreset | null, provider_kind?: ProviderKind): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const target_kind = provider_kind ?? preset.provider_kind;
  const explicit_format = preset.formats.find((item) => item.provider_kind === target_kind);
  if (explicit_format) {
    return explicit_format;
  }
  return preset.formats.find((item) => format_supports_provider_kind(item, target_kind)) ?? null;
}

function preset_provider_kinds(preset: ProviderPreset | null): ProviderKind[] {
  if (!preset) {
    return [];
  }
  const values = new Set<ProviderKind>();
  for (const format of preset.formats) {
    if (format.provider_kind) {
      values.add(format.provider_kind);
      continue;
    }
    values.add("llm");
    if (SUPPORTED_IMAGE_API_FORMATS.has(format.api_format)) {
      values.add("image_generation");
    }
  }
  if (values.size === 0) {
    values.add(preset.provider_kind);
  }
  return Array.from(values);
}

function preset_supports_current_runtime(preset: ProviderPreset): boolean {
  return !!get_supported_preset_format(preset);
}

function preset_allows_non_runtime_config(preset: ProviderPreset | null): boolean {
  return !!preset && CONFIGURABLE_NON_RUNTIME_PRESET_KEYS.has(preset.preset_key);
}

function preset_uses_builtin_endpoint(preset: ProviderPreset | null): boolean {
  return !!preset && preset.preset_key !== "custom";
}

function preset_is_configurable(preset: ProviderPreset): boolean {
  return preset_provider_kinds(preset).includes("image_generation")
    || preset_supports_current_runtime(preset)
    || preset_allows_non_runtime_config(preset);
}

function api_format_supported_for_kind(provider_kind: ProviderKind, api_format: ProviderApiFormat): boolean {
  if (provider_kind === "image_generation") {
    return SUPPORTED_IMAGE_API_FORMATS.has(api_format);
  }
  return SUPPORTED_AGENT_API_FORMATS.has(api_format);
}

function build_provider_draft(
  presets: ProviderPreset[],
  preset_key = "anthropic",
): ProviderDraft {
  const preset = presets.find((item) => item.preset_key === preset_key) ?? presets[0] ?? null;
  const provider_kind = preset?.provider_kind ?? "llm";
  const supported_format = get_supported_preset_format(preset, provider_kind);
  const format = supported_format ?? get_preset_format(preset);
  const provider_key = preset ? get_preset_provider_key(preset) : "";
  const is_custom = preset?.preset_key === "custom";
  return {
    provider_kind,
    provider: is_custom ? "" : provider_key,
    preset_key: preset?.preset_key ?? "custom",
    api_format: (format?.api_format ?? preset?.default_api_format ?? DEFAULT_AGENT_API_FORMAT) as ProviderApiFormat,
    display_name: is_custom ? "" : (preset?.display_name ?? ""),
    auth_token: "",
    base_url: format?.base_url ?? "",
    models_path: format?.models_path ?? "",
    enabled: false,
  };
}

function to_provider_draft(item: ProviderConfigRecord): ProviderDraft {
  return {
    provider_kind: item.provider_kind,
    provider: item.provider,
    preset_key: item.preset_key || "custom",
    api_format: item.api_format,
    display_name: item.display_name || item.provider,
    auth_token: "",
    base_url: item.base_url,
    models_path: item.models_path || "",
    enabled: item.enabled,
  };
}

function stringify_json(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2);
}

function get_provider_title(item: ProviderConfigRecord): string {
  return item.display_name || item.provider;
}

function is_custom_provider_record(item: ProviderConfigRecord): boolean {
  return !item.preset_key || item.preset_key === "custom";
}

function get_usage_agent_title(agent: ProviderConfigRecord["used_by_agents"][number]): string {
  return agent.display_name?.trim() || agent.name?.trim() || agent.agent_id;
}

type TranslateFn = (key: TranslationKey, params?: Record<string, string | number>) => string;

function parse_provider_options(raw: string, invalid_object_message: string): Record<string, unknown> {
  const trimmed = raw.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(invalid_object_message);
  }
  return parsed as Record<string, unknown>;
}

function get_effective_endpoint_format(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): ProviderPresetFormat | null {
  if (!preset_uses_builtin_endpoint(preset)) {
    return null;
  }
  return get_preset_format(preset, draft.api_format);
}

function get_effective_base_url(draft: ProviderDraft, preset: ProviderPreset | null): string {
  return get_effective_endpoint_format(draft, preset)?.base_url ?? draft.base_url;
}

function get_effective_models_path(draft: ProviderDraft, preset: ProviderPreset | null): string {
  return get_effective_endpoint_format(draft, preset)?.models_path ?? draft.models_path;
}

function build_provider_payload_from_draft(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): UpdateProviderConfigPayload {
  return {
    provider_kind: draft.provider_kind,
    preset_key: draft.preset_key,
    api_format: draft.api_format,
    display_name: draft.display_name.trim() || draft.provider.trim(),
    base_url: get_effective_base_url(draft, preset).trim(),
    models_path: get_effective_models_path(draft, preset).trim(),
    enabled: draft.enabled,
  };
}

function get_provider_draft_error(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  is_creating: boolean,
  translate: TranslateFn,
): string | null {
  if (!draft.provider.trim() && draft.preset_key === "custom") {
    return translate("settings.providers.validation_provider_name_required");
  }
  if (!draft.provider.trim()) {
    return translate("settings.providers.validation_provider_required");
  }
  if (!get_effective_base_url(draft, preset).trim()) {
    return translate("settings.providers.validation_base_url_required");
  }
  if (!draft.api_format.trim()) {
    return translate("settings.providers.validation_api_format_required");
  }
  if (is_creating && !draft.auth_token.trim()) {
    return translate("settings.providers.validation_auth_token_required");
  }
  return null;
}

function provider_draft_has_changes(
  draft: ProviderDraft,
  record: ProviderConfigRecord | null,
  preset: ProviderPreset | null,
): boolean {
  if (!record) {
    return true;
  }
  if (draft.auth_token.trim()) {
    return true;
  }
  return draft.preset_key !== (record.preset_key || "custom")
    || draft.api_format !== record.api_format
    || (draft.display_name.trim() || draft.provider.trim()) !== (record.display_name || record.provider)
    || get_effective_base_url(draft, preset).trim() !== record.base_url
    || get_effective_models_path(draft, preset).trim() !== (record.models_path || "")
    || draft.enabled !== record.enabled;
}

function format_token_preview(masked_token: string | null | undefined, empty_label: string): string {
  const normalized_masked_token = masked_token?.trim();
  if (!normalized_masked_token) {
    return empty_label;
  }
  return normalized_masked_token;
}

function provider_has_active_config(item: ProviderConfigRecord | null | undefined): boolean {
  return !!item?.enabled && !!item.auth_token_masked?.trim();
}

function format_count(value?: number | null): string {
  if (!value || value <= 0) {
    return "auto";
  }
  if (value >= 1000) {
    return `${Math.round(value / 1000)}K`;
  }
  return String(value);
}

function normalize_custom_provider_key(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function get_effective_capabilities(model: ProviderModelRecord): ProviderModelCapabilities {
  return {
    ...model.capabilities_auto,
    ...model.capabilities_override,
  };
}

function sort_models_enabled_first(models: ProviderModelRecord[]): ProviderModelRecord[] {
  return [...models].sort((left, right) => {
    if (left.enabled !== right.enabled) {
      return left.enabled ? -1 : 1;
    }
    return (left.display_name || left.model_id).localeCompare(right.display_name || right.model_id, "zh-Hans-CN");
  });
}

function order_provider_records(
  items: ProviderConfigRecord[],
  previous_items: ProviderConfigRecord[],
): ProviderConfigRecord[] {
  const previous_index_map = new Map(previous_items.map((item, index) => [item.provider, index]));
  return [...items].sort((left, right) => {
    const left_index = previous_index_map.get(left.provider);
    const right_index = previous_index_map.get(right.provider);
    if (left_index !== undefined && right_index !== undefined) {
      return left_index - right_index;
    }
    if (left_index !== undefined) {
      return -1;
    }
    if (right_index !== undefined) {
      return 1;
    }
    return get_provider_title(left).localeCompare(get_provider_title(right), "zh-Hans-CN");
  });
}

function first_builtin_preset_key(presets: ProviderPreset[]): string | null {
  return presets.find((preset) => preset.preset_key !== "custom")?.preset_key ?? null;
}

function provider_for_preset(
  items: ProviderConfigRecord[],
  preset_key: string,
): ProviderConfigRecord | null {
  return items.find((item) => item.preset_key === preset_key) ?? null;
}

function model_options_from_record(model: ProviderModelRecord): ModelOptionsState {
  return {
    model,
    capabilities: { ...model.capabilities_override },
    context_window: model.context_window ? String(model.context_window) : "",
    max_output_tokens: model.max_output_tokens ? String(model.max_output_tokens) : "",
    provider_options_text: stringify_json(model.provider_options ?? {}),
  };
}

function model_update_payload(
  model: ProviderModelRecord,
  override?: Partial<UpdateProviderModelPayload>,
): UpdateProviderModelPayload {
  return {
    enabled: model.enabled,
    is_default: model.is_default,
    capabilities_override: model.capabilities_override ?? {},
    context_window: model.context_window ?? null,
    max_output_tokens: model.max_output_tokens ?? null,
    provider_options: model.provider_options ?? {},
    ...override,
  };
}

function CapabilitySwitch({
  checked,
  label,
  icon,
  on_change,
}: {
  checked: boolean;
  label: string;
  icon: ReactNode;
  on_change: (checked: boolean) => void;
}) {
  return (
    <div className="flex min-h-10 items-center justify-between gap-3 rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_78%,transparent)] px-3 py-2">
      <div className="flex min-w-0 items-center gap-2 text-[13px] font-medium text-(--text-strong)">
        <span className="text-(--icon-default)">{icon}</span>
        <span className="truncate">{label}</span>
      </div>
      <GlassSwitch checked={checked} size="xs" on_change={on_change} />
    </div>
  );
}

export function ProviderSettingsPanel({ embedded = false }: ProviderSettingsPanelProps) {
  const { t } = useI18n();
  const [presets, set_presets] = useState<ProviderPreset[]>([]);
  const [providers, set_providers] = useState<ProviderConfigRecord[]>([]);
  const [selected_provider, set_selected_provider] = useState<string | null>(null);
  const [mode, set_mode] = useState<FormMode>("empty");
  const [draft, set_draft] = useState<ProviderDraft>(build_provider_draft([]));
  const [model_query, set_model_query] = useState("");
  const [loading, set_loading] = useState(true);
  const [submitting, set_submitting] = useState(false);
  const [pending_action, set_pending_action] = useState<string | null>(null);
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);
  const [delete_confirm_open, set_delete_confirm_open] = useState(false);
  const [delete_usage_open, set_delete_usage_open] = useState(false);
  const [delete_target_provider, set_delete_target_provider] = useState<string | null>(null);
  const [model_options, set_model_options] = useState<ModelOptionsState | null>(null);
  const [add_model_open, set_add_model_open] = useState(false);
  const [manual_model_id, set_manual_model_id] = useState("");
  const [manual_model_enabled, set_manual_model_enabled] = useState(true);
  const providers_ref = useRef<ProviderConfigRecord[]>([]);
  const selected_provider_ref = useRef<string | null>(null);
  const save_promise_ref = useRef<Promise<ProviderConfigRecord | null> | null>(null);

  useEffect(() => {
    providers_ref.current = providers;
  }, [providers]);

  useEffect(() => {
    selected_provider_ref.current = selected_provider;
  }, [selected_provider]);

  const selected_record = useMemo(
    () => providers.find((item) => item.provider === selected_provider) ?? null,
    [providers, selected_provider],
  );
  const delete_target_record = useMemo(
    () => providers.find((item) => item.provider === delete_target_provider) ?? null,
    [delete_target_provider, providers],
  );
  const current_preset = useMemo(
    () => presets.find((item) => item.preset_key === draft.preset_key) ?? presets.find((item) => item.preset_key === "custom") ?? null,
    [draft.preset_key, presets],
  );
  const provider_kind_options = useMemo(() => {
    const available_kinds = preset_provider_kinds(current_preset);
    const ordered_kinds: ProviderKind[] = ["llm", "image_generation"];
    return ordered_kinds
      .filter((kind) => available_kinds.length === 0 || available_kinds.includes(kind))
      .map((kind) => ({
        value: kind,
        label: kind === "image_generation"
          ? t("settings.providers.kind_image_generation")
          : t("settings.providers.kind_llm"),
      }));
  }, [current_preset, t]);
  const can_select_non_runtime_format = draft.provider_kind === "llm" && preset_allows_non_runtime_config(current_preset);
  const format_options = useMemo(
    () => {
      const seen = new Set<ProviderApiFormat>();
      return (current_preset?.formats ?? [])
        .filter((item) => {
          if (seen.has(item.api_format)) {
            return false;
          }
          seen.add(item.api_format);
          return true;
        })
        .map((item) => {
          const supported = format_supports_provider_kind(item, draft.provider_kind);
          return {
            value: item.api_format,
            label: supported || can_select_non_runtime_format
              ? API_FORMAT_LABELS[item.api_format]
              : `${API_FORMAT_LABELS[item.api_format]}${t("settings.providers.unsupported_suffix")}`,
            disabled: !supported && !can_select_non_runtime_format,
          };
        });
    },
    [can_select_non_runtime_format, current_preset, draft.provider_kind, t],
  );
  const filtered_models = useMemo(() => {
    const query = model_query.trim().toLowerCase();
    const models = selected_record?.models ?? [];
    if (!query) {
      return models;
    }
    return models.filter((model) => (
      model.model_id.toLowerCase().includes(query)
      || model.display_name.toLowerCase().includes(query)
      || model.category.toLowerCase().includes(query)
    ));
  }, [model_query, selected_record]);
  const is_editing = mode === "edit" && !!selected_record;
  const is_creating = mode === "create";
  const is_empty_mode = mode === "empty";
  const selected_can_manage = !is_editing || selected_record?.can_manage !== false;
  const can_save = useMemo(() => {
    if (is_empty_mode || !selected_can_manage) {
      return false;
    }
    return get_provider_draft_error(draft, current_preset, is_creating, t) === null;
  }, [current_preset, draft, is_creating, is_empty_mode, selected_can_manage, t]);

  const refresh_all = useCallback(async (preferred_provider?: string | null) => {
    try {
      const [next_presets, next_providers] = await Promise.all([
        list_provider_presets_api(),
        list_provider_configs_api(),
      ]);
      set_presets(next_presets);
      const ordered_items = order_provider_records(next_providers, providers_ref.current);
      set_providers(ordered_items);
      invalidate_provider_availability();
      const target = ordered_items.find((item) => item.provider === preferred_provider)
        ?? ordered_items.find((item) => item.provider === selected_provider_ref.current);
      if (target) {
        set_mode("edit");
        set_selected_provider(target.provider);
        set_draft(to_provider_draft(target));
      } else {
        const first_preset_key = first_builtin_preset_key(next_presets);
        const preset_target = first_preset_key
          ? provider_for_preset(ordered_items, first_preset_key)
          : null;
        if (preset_target) {
          set_mode("edit");
          set_selected_provider(preset_target.provider);
          set_draft(to_provider_draft(preset_target));
        } else {
          set_mode("create");
          set_selected_provider(null);
          set_draft(build_provider_draft(next_presets, first_preset_key ?? "custom"));
        }
      }
      set_feedback((current) => (current?.tone === "error" ? null : current));
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.load_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.retry_later"),
      });
    } finally {
      set_loading(false);
    }
  }, [t]);

  useEffect(() => {
    void refresh_all();
  }, [refresh_all]);

  const handle_select_provider = useCallback((provider: string) => {
    const target = providers.find((item) => item.provider === provider);
    if (!target) {
      return;
    }
    set_mode("edit");
    set_selected_provider(target.provider);
    set_model_query("");
    set_add_model_open(false);
    set_draft(to_provider_draft(target));
  }, [providers]);

  const handle_create_from_preset = useCallback((preset_key: string) => {
    set_mode("create");
    set_selected_provider(null);
    set_model_query("");
    set_add_model_open(false);
    set_draft(build_provider_draft(presets, preset_key));
  }, [presets]);

  const handle_provider_kind_change = useCallback((value: string) => {
    const provider_kind = value as ProviderKind;
    set_draft((current) => {
      const current_format = get_preset_format(current_preset, current.api_format);
      const format = current_format && format_supports_provider_kind(current_format, provider_kind)
        ? current_format
        : get_supported_preset_format(current_preset, provider_kind);
      const api_format = format?.api_format
        ?? (provider_kind === "image_generation" ? "chat_completions" : DEFAULT_AGENT_API_FORMAT);
      return {
        ...current,
        provider_kind,
        api_format,
        base_url: format?.base_url ?? current.base_url,
        models_path: format?.models_path ?? current.models_path,
      };
    });
  }, [current_preset]);

  const handle_api_format_change = useCallback((value: string) => {
    const api_format = value as ProviderApiFormat;
    const format = get_preset_format(current_preset, api_format);
    const supported = format ? format_supports_provider_kind(format, draft.provider_kind) : false;
    if (!supported && !can_select_non_runtime_format) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.api_format_unsupported_title"),
        message: t("settings.providers.api_format_unsupported_message"),
      });
      return;
    }
    set_draft((current) => ({
      ...current,
      api_format,
      base_url: format?.base_url ?? current.base_url,
      models_path: format?.models_path ?? current.models_path,
    }));
  }, [can_select_non_runtime_format, current_preset, draft.provider_kind, t]);

  const handle_save = useCallback(async (options?: {
    draft_overrides?: Partial<ProviderDraft>;
    show_error?: boolean;
    show_success?: boolean;
  }): Promise<ProviderConfigRecord | null> => {
    if (is_empty_mode) {
      return null;
    }
    if (is_editing && selected_record?.can_manage === false) {
      return selected_record;
    }
    if (save_promise_ref.current) {
      return save_promise_ref.current;
    }
    const next_draft: ProviderDraft = {
      ...draft,
      ...options?.draft_overrides,
    };
    const show_error = options?.show_error ?? true;
    const show_success = options?.show_success ?? false;
    const validation_error = get_provider_draft_error(next_draft, current_preset, is_creating, t);
    if (validation_error) {
      if (show_error) {
        set_feedback({
          tone: "error",
          title: t("settings.providers.config_incomplete_title"),
          message: validation_error,
        });
      }
      return null;
    }
    if (is_editing && !provider_draft_has_changes(next_draft, selected_record, current_preset)) {
      return selected_record;
    }
    const save_promise = (async () => {
      set_submitting(true);
      try {
        const payload = build_provider_payload_from_draft(next_draft, current_preset);
        const normalized_auth_token = next_draft.auth_token.trim();
        if (normalized_auth_token) {
          payload.auth_token = normalized_auth_token;
        }
        const result = is_editing && selected_record
          ? await update_provider_config_api(selected_record.provider, payload)
          : await create_provider_config_api({
            ...payload,
            provider: next_draft.provider.trim(),
            auth_token: normalized_auth_token,
            provider_kind: next_draft.provider_kind,
            display_name: payload.display_name,
            base_url: payload.base_url,
            enabled: payload.enabled,
          });
        await refresh_all(result.provider);
        if (show_success) {
          set_feedback({
            tone: "success",
            title: t("settings.providers.saved_title"),
            message: t("settings.providers.saved_message", { name: result.display_name || result.provider }),
          });
        }
        return result;
      } catch (error) {
        if (show_error) {
          set_feedback({
            tone: "error",
            title: t("settings.providers.save_failed_title"),
            message: error instanceof Error ? error.message : t("settings.providers.check_config_retry"),
          });
        }
        return null;
      } finally {
        set_submitting(false);
      }
    })();
    save_promise_ref.current = save_promise;
    try {
      return await save_promise;
    } finally {
      if (save_promise_ref.current === save_promise) {
        save_promise_ref.current = null;
      }
    }
  }, [current_preset, draft, is_creating, is_editing, is_empty_mode, refresh_all, selected_record, t]);

  const handle_provider_field_blur = useCallback(() => {
    if (!can_save || pending_action || submitting) {
      return;
    }
    if (is_editing && !provider_draft_has_changes(draft, selected_record, current_preset)) {
      return;
    }
    void handle_save({ show_error: false, show_success: false });
  }, [can_save, current_preset, draft, handle_save, is_editing, pending_action, selected_record, submitting]);

  const handle_enabled_change = useCallback((checked: boolean) => {
    if (!selected_can_manage) {
      return;
    }
    set_draft((current) => ({ ...current, enabled: checked }));
    void (async () => {
      const result = await handle_save({
        draft_overrides: { enabled: checked },
        show_error: true,
        show_success: false,
      });
      if (!result) {
        set_draft((current) => ({ ...current, enabled: !checked }));
      }
    })();
  }, [handle_save, selected_can_manage]);

  const handle_request_delete_provider = useCallback((item: ProviderConfigRecord) => {
    if (!is_custom_provider_record(item)) {
      return;
    }
    if (item.usage_count > 0) {
      set_delete_target_provider(item.provider);
      set_delete_usage_open(true);
      return;
    }
    set_delete_target_provider(item.provider);
    set_delete_confirm_open(true);
  }, []);

  const handle_delete = useCallback(async (force = false) => {
    if (!delete_target_record || submitting) {
      return;
    }
    if (delete_target_record.usage_count > 0 && !force) {
      set_delete_confirm_open(false);
      set_delete_usage_open(true);
      return;
    }
    try {
      set_submitting(true);
      const result = await delete_provider_config_api(delete_target_record.provider, { force });
      set_delete_confirm_open(false);
      set_delete_usage_open(false);
      set_delete_target_provider(null);
      await refresh_all();
      const replacement_message = result.replacement_provider
        ? t("settings.providers.delete_reassigned_message", {
          count: result.reassigned_runtime_count ?? 0,
          provider: result.replacement_provider,
        })
        : t("settings.providers.delete_removed_message", { name: get_provider_title(delete_target_record) });
      set_feedback({
        tone: "success",
        title: t("settings.providers.deleted_title"),
        message: replacement_message,
      });
    } catch (error) {
      set_delete_confirm_open(false);
      set_delete_usage_open(false);
      set_delete_target_provider(null);
      set_feedback({
        tone: "error",
        title: t("settings.providers.delete_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.delete_in_use_fallback"),
      });
    } finally {
      set_submitting(false);
    }
  }, [delete_target_record, refresh_all, submitting, t]);

  const handle_fetch_models = useCallback(async () => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action("fetch");
      const provider_record = await handle_save({ show_error: true, show_success: false });
      if (!provider_record) {
        return;
      }
      const result = await fetch_provider_models_api(provider_record.provider);
      await refresh_all(provider_record.provider);
      set_feedback({
        tone: "success",
        title: t("settings.providers.models_synced_title"),
        message: t("settings.providers.models_synced_message", { count: result.count }),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.models_sync_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.models_sync_failed_message"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [handle_save, pending_action, refresh_all, selected_can_manage, selected_record, t]);

  const handle_open_add_model = useCallback(() => {
    if (!selected_can_manage) {
      return;
    }
    set_manual_model_id("");
    set_manual_model_enabled(true);
    set_add_model_open(true);
  }, [selected_can_manage]);

  const handle_add_model = useCallback(async () => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    const model_id = manual_model_id.trim();
    if (!model_id) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_id_required_title"),
        message: t("settings.providers.model_id_required_message"),
      });
      return;
    }
    try {
      set_pending_action(`add-model:${model_id}`);
      await update_provider_model_api(selected_record.provider, model_id, {
        enabled: manual_model_enabled,
        is_default: false,
        capabilities_override: {},
        context_window: null,
        max_output_tokens: null,
        provider_options: {},
      });
      set_add_model_open(false);
      set_manual_model_id("");
      await refresh_all(selected_record.provider);
      set_feedback({
        tone: "success",
        title: t("settings.providers.model_added_title"),
        message: t("settings.providers.model_added_message", { model: model_id }),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_add_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.model_add_failed_message"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [manual_model_enabled, manual_model_id, pending_action, refresh_all, selected_can_manage, selected_record, t]);

  const handle_test_provider = useCallback(async () => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action("test");
      const provider_record = await handle_save({ show_error: true, show_success: false });
      if (!provider_record) {
        return;
      }
      const result = await test_provider_config_api(provider_record.provider);
      await refresh_all(provider_record.provider);
      set_feedback({
        tone: result.success ? "success" : "error",
        title: result.success
          ? t("settings.providers.provider_test_passed_title")
          : t("settings.providers.provider_test_failed_title"),
        message: result.success
          ? t("settings.providers.test_model_message", { model: result.model || t("settings.providers.auto_model") })
          : (result.error || t("settings.providers.connectivity_failed")),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.provider_test_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.check_network_auth"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [handle_save, pending_action, refresh_all, selected_can_manage, selected_record, t]);

  const handle_test_model = useCallback(async (model_id: string) => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    const normalized_model_id = model_id.trim();
    if (!normalized_model_id) {
      return;
    }
    try {
      set_pending_action(`test:${normalized_model_id}`);
      const provider_record = await handle_save({ show_error: true, show_success: false });
      if (!provider_record) {
        return;
      }
      const result = await test_provider_model_api(provider_record.provider, normalized_model_id);
      await refresh_all(provider_record.provider);
      set_feedback({
        tone: result.success ? "success" : "error",
        title: result.success
          ? t("settings.providers.model_test_passed_title")
          : t("settings.providers.model_test_failed_title"),
        message: result.success
          ? t("settings.providers.test_model_message", { model: result.model || normalized_model_id })
          : (result.error || t("settings.providers.connectivity_failed")),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_test_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.check_network_auth_model"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [handle_save, pending_action, refresh_all, selected_can_manage, selected_record, t]);

  const handle_test_selection = useCallback((value: string) => {
    if (value === AUTO_TEST_MODEL_VALUE) {
      void handle_test_provider();
      return;
    }
    void handle_test_model(value);
  }, [handle_test_model, handle_test_provider]);

  const handle_toggle_model = useCallback(async (model: ProviderModelRecord, enabled: boolean) => {
    if (!selected_record || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action(`model:${model.model_id}`);
      await update_provider_model_api(
        selected_record.provider,
        model.model_id,
        model_update_payload(model, { enabled }),
      );
      await refresh_all(selected_record.provider);
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_status_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.retry_later"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [pending_action, refresh_all, selected_can_manage, selected_record, t]);

  const handle_save_model_options = useCallback(async () => {
    if (!selected_record || !model_options || pending_action || !selected_can_manage) {
      return;
    }
    try {
      set_pending_action(`options:${model_options.model.model_id}`);
      const provider_options = parse_provider_options(
        model_options.provider_options_text,
        t("settings.providers.provider_options_json_object"),
      );
      await update_provider_model_api(selected_record.provider, model_options.model.model_id, {
        enabled: model_options.model.enabled,
        is_default: model_options.model.is_default,
        capabilities_override: model_options.capabilities,
        context_window: model_options.context_window.trim() ? Number(model_options.context_window) : null,
        max_output_tokens: model_options.max_output_tokens.trim() ? Number(model_options.max_output_tokens) : null,
        provider_options,
      });
      set_model_options(null);
      await refresh_all(selected_record.provider);
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.providers.model_options_save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.check_json_format"),
      });
    } finally {
      set_pending_action(null);
    }
  }, [model_options, pending_action, refresh_all, selected_can_manage, selected_record, t]);

  const configured_by_preset = useMemo(() => {
    const result = new Map<string, ProviderConfigRecord>();
    for (const item of providers) {
      if (item.preset_key && item.preset_key !== "custom" && !result.has(item.preset_key)) {
        result.set(item.preset_key, item);
      }
    }
    return result;
  }, [providers]);
  const custom_providers = useMemo(
    () => providers.filter((item) => item.preset_key === "custom" || !configured_by_preset.has(item.preset_key)),
    [configured_by_preset, providers],
  );
  const preset_sidebar_items = presets.filter((preset) => preset.preset_key !== "custom");
  const detail_title = is_editing && selected_record
    ? get_provider_title(selected_record)
    : draft.display_name || current_preset?.display_name || t("settings.providers.custom_provider");
  const is_custom_provider = draft.preset_key === "custom";
  const uses_builtin_endpoint = preset_uses_builtin_endpoint(current_preset);
  const current_format = get_preset_format(current_preset, draft.api_format);
  const current_format_supports_kind = current_format
    ? format_supports_provider_kind(current_format, draft.provider_kind)
    : false;
  const is_api_format_configurable = current_format_supports_kind || can_select_non_runtime_format;
  const show_runtime_format_badge = draft.provider_kind === "llm" && !SUPPORTED_AGENT_API_FORMATS.has(draft.api_format);
  const show_provider_shape_controls = is_custom_provider;
  const has_models_endpoint = !!get_effective_models_path(draft, current_preset).trim();
  const displayed_models = sort_models_enabled_first(filtered_models);
  const test_model_options = useMemo(() => {
    const models = sort_models_enabled_first(selected_record?.models ?? []);
    return [
      { value: AUTO_TEST_MODEL_VALUE, label: t("settings.providers.auto_select_model") },
      ...models.map((model) => {
        const display_name = model.display_name || model.model_id;
        return {
          value: model.model_id,
          label: display_name === model.model_id ? model.model_id : `${display_name} · ${model.model_id}`,
        };
      }),
    ];
  }, [selected_record, t]);
  const builtin_endpoint_formats = uses_builtin_endpoint ? current_preset?.formats ?? [] : [];
  const manual_model_placeholder = selected_record?.models[0]?.model_id
    || (draft.api_format === "anthropic_messages" ? "opus-4.7" : "model-id");
  const delete_usage_agents = delete_target_record?.used_by_agents ?? [];

  const panel_content = (
    <div className={cn("mx-auto flex h-full min-h-0 w-full flex-col px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <div className="flex min-h-0 flex-1 items-stretch gap-5 overflow-hidden">
        <aside
          className="max-w-full shrink-0 border-r border-(--divider-subtle-color) pr-4"
          style={{ width: 190 }}
        >
          <div className="soft-scrollbar h-full min-h-0 overflow-y-auto pr-2">
            {loading ? (
              <div className="flex min-h-[260px] items-center justify-center text-(--text-soft)">
                <Loader2 className="h-4 w-4 animate-spin" />
              </div>
            ) : (
              <div className="space-y-1 py-2">
                <button
                  className={cn(
                    "flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-[13px] font-semibold transition-[background,color] duration-(--motion-duration-fast)",
                    is_creating && draft.preset_key === "custom"
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                      : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  onClick={() => handle_create_from_preset("custom")}
                  type="button"
                >
                  <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px] border border-dashed border-(--surface-interactive-active-border) text-primary">
                    <Plus className="h-3.5 w-3.5" />
                  </span>
                  <span className="min-w-0 flex-1 truncate">{t("settings.providers.custom_provider")}</span>
                </button>

                {preset_sidebar_items.map((preset) => {
                  const item = configured_by_preset.get(preset.preset_key);
                  const is_active = item
                    ? item.provider === selected_provider && is_editing
                    : is_creating && draft.preset_key === preset.preset_key;
                  const is_unsupported_preset = !preset_is_configurable(preset);
                  return (
                    <button
                      className={cn(
                        "flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-[13px] font-semibold transition-[background,color] duration-(--motion-duration-fast)",
                        is_unsupported_preset
                          ? "cursor-not-allowed text-(--text-soft) opacity-50"
                          : is_active
                          ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                      )}
                      disabled={is_unsupported_preset}
                      key={preset.preset_key}
                      onClick={() => {
                        if (is_unsupported_preset) {
                          return;
                        }
                        if (item) {
                          handle_select_provider(item.provider);
                        } else {
                          handle_create_from_preset(preset.preset_key);
                        }
                      }}
                      type="button"
                    >
                      <ProviderIcon
                        active={!is_unsupported_preset && provider_has_active_config(item)}
                        name={preset.display_name}
                        preset_key={preset.preset_key}
                      />
                      <span className="min-w-0 flex-1 truncate">{preset.display_name}</span>
                      {is_unsupported_preset ? (
                        <span className="shrink-0 rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-semibold text-(--text-soft)">
                          {t("settings.providers.unsupported_badge")}
                        </span>
                      ) : null}
                    </button>
                  );
                })}

                {custom_providers.map((item) => {
                  const is_active = item.provider === selected_provider && is_editing;
                  const can_show_delete = is_custom_provider_record(item) && item.can_manage;
                  return (
                    <div
                      className={cn(
                        "group flex min-h-10 w-full items-center rounded-[10px] transition-[background,color] duration-(--motion-duration-fast)",
                        is_active
                          ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                      )}
                      key={item.provider}
                    >
                      <button
                        className="flex min-h-10 min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left text-[13px] font-semibold"
                        onClick={() => handle_select_provider(item.provider)}
                        type="button"
                      >
                        <ProviderIcon
                          active={provider_has_active_config(item)}
                          name={get_provider_title(item)}
                          preset_key={item.preset_key}
                        />
                        <span className="min-w-0 flex-1 truncate">{get_provider_title(item)}</span>
                      </button>
                      {can_show_delete ? (
                        <UiIconButton
                          aria-label={t("settings.providers.delete_aria", { name: get_provider_title(item) })}
                          class_name={cn(
                            "mr-1 h-7 w-7 transition-opacity group-hover:opacity-100 focus-visible:opacity-100",
                            is_active ? "opacity-100" : "opacity-0",
                          )}
                          disabled={submitting || pending_action !== null}
                          onClick={() => handle_request_delete_provider(item)}
                          size="xs"
                          title={item.usage_count > 0
                            ? t("settings.providers.delete_in_use_title", { count: item.usage_count })
                            : t("settings.providers.delete_provider")}
                          tone={item.usage_count > 0 ? undefined : "danger"}
                          type="button"
                          variant="ghost"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </UiIconButton>
                      ) : null}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </aside>

        <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          {is_empty_mode ? null : (
            <div className="flex min-h-0 flex-1 flex-col bg-transparent px-5 py-2">
              <div className="mb-4 flex shrink-0 items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2.5">
                    <h2 className="truncate text-[18px] font-semibold tracking-tight text-(--text-strong)">
                      {detail_title}
                    </h2>
                    {selected_record ? (
                      <span className={cn(
                        "rounded-full px-2 py-0.5 text-[11px] font-semibold",
                        draft.enabled
                          ? "bg-[rgba(44,156,89,0.14)] text-[rgb(33,133,74)]"
                          : "bg-(--surface-muted-background) text-(--text-muted)",
                      )}
                      >
                        {draft.enabled
                          ? t("settings.providers.status_active")
                          : t("settings.providers.status_inactive")}
                      </span>
                    ) : null}
                  </div>
                  {current_preset?.description ? (
                    <p className="mt-1 max-w-2xl truncate text-[12px] leading-5 text-(--text-muted)">
                      {current_preset.description}
                    </p>
                  ) : null}
                </div>

                <div className="flex shrink-0 items-center gap-2 pt-0.5">
                  {is_editing ? (
                    <UiSelectMenu
                      aria_label={t("settings.providers.test_provider")}
                      button_class_name="px-2"
                      class_name="w-auto min-w-18"
                      disabled={pending_action !== null || submitting || !is_api_format_configurable || !selected_can_manage}
                      leading={pending_action?.startsWith("test") ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Play className="h-3.5 w-3.5" />
                      )}
                      menu_class_name="min-w-[220px]"
                      on_change={handle_test_selection}
                      options={test_model_options}
                      placeholder={t("settings.providers.test")}
                      size="xs"
                      value=""
                    />
                  ) : null}
                  <GlassSwitch
                    checked={draft.enabled}
                    disabled={pending_action !== null || submitting || !is_api_format_configurable || !selected_can_manage}
                    size="sm"
                    on_change={handle_enabled_change}
                  />
                </div>
              </div>

              <div className="flex min-h-0 flex-1 flex-col gap-4">
                {show_provider_shape_controls ? (
                  <div className={cn(
                    "grid gap-4",
                    is_custom_provider
                      ? "md:grid-cols-[minmax(0,1fr)_180px_260px]"
                      : "md:grid-cols-[180px_260px]",
                  )}
                  >
                    {is_custom_provider ? (
                      <label className="space-y-2">
                        <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.provider_name")}</span>
                        <UiInput
                          autoCapitalize="off"
                          autoCorrect="off"
                          control_size="lg"
                          disabled={!selected_can_manage}
                          onChange={(event) => {
                            const next_name = event.target.value;
                            set_draft((current) => ({
                              ...current,
                              display_name: next_name,
                              provider: is_creating ? normalize_custom_provider_key(next_name) : current.provider,
                            }));
                          }}
                          onBlur={handle_provider_field_blur}
                          placeholder={t("settings.providers.provider_name_placeholder")}
                          spellCheck={false}
                          type="text"
                          value={draft.display_name}
                        />
                      </label>
                    ) : null}

                    <label className="space-y-2">
                      <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.kind")}</span>
                      <UiSelectMenu
                        aria_label={t("settings.providers.kind")}
                        class_name="h-11"
                        disabled={!selected_can_manage || is_editing || provider_kind_options.length <= 1}
                        on_change={handle_provider_kind_change}
                        options={provider_kind_options}
                        size="sm"
                        value={draft.provider_kind}
                      />
                    </label>

                    <label className="space-y-2">
                      <span className="flex items-center gap-2">
                        <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.api_format")}</span>
                        {show_runtime_format_badge ? (
                          <span
                            className="rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-medium leading-4 text-(--text-muted)"
                            title={t("settings.providers.api_format_runtime_hint")}
                          >
                            {t("settings.providers.api_format_runtime_badge")}
                          </span>
                        ) : null}
                      </span>
                      <UiSelectMenu
                        aria_label={t("settings.providers.api_format")}
                        class_name="h-11"
                        disabled={!selected_can_manage || format_options.length <= 1}
                        on_change={handle_api_format_change}
                        options={format_options}
                        size="sm"
                        value={draft.api_format}
                      />
                    </label>
                  </div>
                ) : null}

                <label className="block space-y-2">
                  <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.api_key")}</span>
                  <UiInput
                    autoCapitalize="off"
                    autoComplete="off"
                    autoCorrect="off"
                    control_size="md"
                    data-form-type="other"
                    data-lpignore="true"
                    disabled={!selected_can_manage}
                    name="provider-auth-token"
                    onChange={(event) => set_draft((current) => ({ ...current, auth_token: event.target.value }))}
                    onBlur={handle_provider_field_blur}
                    placeholder={is_editing
                      ? format_token_preview(
                        selected_record?.auth_token_masked,
                        t("settings.providers.api_key_empty"),
                      )
                      : t("settings.providers.api_key_placeholder")}
                    spellCheck={false}
                    type="password"
                    value={draft.auth_token}
                  />
                  {current_preset?.key_url ? (
                    <a
                      className="inline-flex items-center gap-1 text-[12px] font-medium text-primary hover:underline"
                      href={current_preset.key_url}
                      rel="noreferrer"
                      target="_blank"
                    >
                      {t("settings.providers.get_api_key_from", { name: detail_title })}
                      <ExternalLink className="h-3.5 w-3.5" />
                    </a>
                  ) : null}
                </label>

                <div className="block space-y-2">
                  <span className={PROVIDER_LABEL_CLASS_NAME}>{t("settings.providers.base_url")}</span>
                  {uses_builtin_endpoint ? (
                    <div className="space-y-1.5">
                      {builtin_endpoint_formats.map((format) => (
                        <div
                          className="input-shell grid min-h-9 grid-cols-1 items-center gap-1.5 rounded-[12px] px-3.5 py-1.5 text-sm text-(--text-default) sm:grid-cols-[88px_minmax(0,1fr)] sm:gap-3"
                          key={format.api_format}
                        >
                          <span
                            className="inline-flex h-6 w-fit max-w-full items-center rounded-full bg-(--surface-muted-background) px-2 text-[11px] font-semibold text-(--text-muted)"
                            title={API_FORMAT_LABELS[format.api_format]}
                          >
                            {API_FORMAT_SHORT_LABELS[format.api_format]}
                          </span>
                          <span className="min-w-0 break-all font-mono text-(--text-strong)">
                            {format.base_url}
                          </span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <UiInput
                      autoCapitalize="off"
                      autoCorrect="off"
                      control_size="md"
                      disabled={!selected_can_manage}
                      onChange={(event) => set_draft((current) => ({ ...current, base_url: event.target.value }))}
                      onBlur={handle_provider_field_blur}
                      placeholder={current_format?.base_url || "https://api.example.com/v1"}
                      spellCheck={false}
                      type="text"
                      value={draft.base_url}
                    />
                  )}
                </div>

                <div className="flex min-h-0 flex-1 flex-col gap-3 pt-1">
                  <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
                    <div className="flex min-w-0 items-baseline gap-2">
                      <h3 className="text-[14px] font-semibold tracking-tight text-(--text-strong)">
                        {t("settings.providers.models")}
                      </h3>
                      {selected_record ? (
                        <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-(--surface-muted-background) px-1.5 text-[11px] font-semibold text-(--text-muted)">
                          {displayed_models.length}
                        </span>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-2">
                      {is_editing && selected_record ? (
                        <>
                          <UiButton
                            disabled={pending_action !== null || !is_api_format_configurable || !selected_can_manage}
                            onClick={handle_open_add_model}
                            size="xs"
                            type="button"
                            variant="surface"
                          >
                            <Plus className="h-3.5 w-3.5" />
                            {t("settings.providers.add_model")}
                          </UiButton>
                          <UiButton
                            disabled={pending_action !== null || !is_api_format_configurable || !selected_can_manage || !has_models_endpoint}
                            onClick={() => void handle_fetch_models()}
                            size="xs"
                            title={!has_models_endpoint ? t("settings.providers.sync_models_unavailable") : undefined}
                            type="button"
                            variant="surface"
                          >
                            {pending_action === "fetch" ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                            {t("settings.providers.sync_models")}
                          </UiButton>
                        </>
                      ) : null}
                    </div>
                  </div>

                  <UiSearchInput
                    class_name="w-full"
                    control_size="md"
                    on_change={set_model_query}
                    placeholder={t("settings.providers.search_models")}
                    value={model_query}
                    variant="dialog"
                  />

                  <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto rounded-[12px] border border-(--divider-subtle-color)">
                    {!selected_record || displayed_models.length === 0 ? (
                      <div className="flex min-h-28 items-center justify-center text-sm text-(--text-soft)">
                        {selected_record
                          ? t("settings.providers.models_empty")
                          : t("settings.providers.models_after_save")}
                      </div>
                    ) : (
                      displayed_models.map((model) => {
                        const capabilities = get_effective_capabilities(model);
                        const pending_model = pending_action?.endsWith(model.model_id) ?? false;
                        const display_name = model.display_name || model.model_id;
                        const show_model_id = model.model_id !== display_name;
                        return (
                          <div
                            className="grid min-h-9 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 border-b border-(--divider-subtle-color) px-2.5 py-1 last:border-b-0"
                            key={model.model_id}
                          >
                            <div className="flex min-w-0 items-center gap-2">
                              <span className="min-w-0 truncate font-mono text-[13px] leading-5 text-(--text-strong)">
                                {display_name}
                              </span>
                              <span className="flex shrink-0 items-center gap-1.5 text-[10px] leading-4 text-(--text-muted)">
                                {capabilities.tool_calling ? <Wrench className="h-3 w-3" /> : null}
                                {capabilities.reasoning ? <Brain className="h-3 w-3" /> : null}
                                {capabilities.vision ? <Eye className="h-3 w-3" /> : null}
                                {capabilities.image_output ? <Image className="h-3 w-3" /> : null}
                                <span>{format_count(model.context_window)}</span>
                              </span>
                            </div>
                            <div className="flex min-w-0 items-center gap-2">
                              {show_model_id ? (
                              <span className="hidden max-w-[120px] truncate font-mono text-[11px] text-(--text-soft) xl:inline">
                                {model.model_id}
                              </span>
                              ) : null}
                              <UiIconButton
                                onClick={() => set_model_options(model_options_from_record(model))}
                                size="xs"
                                title={t("settings.providers.model_options")}
                                type="button"
                                variant="ghost"
                              >
                                <SlidersHorizontal className="h-3.5 w-3.5" />
                              </UiIconButton>
                              {pending_model ? (
                                <Loader2 className="h-4 w-4 animate-spin text-(--text-muted)" />
                              ) : (
                                <GlassSwitch
                                  checked={model.enabled}
                                  disabled={pending_action !== null || !selected_can_manage}
                                  size="xs"
                                  on_change={(checked) => void handle_toggle_model(model, checked)}
                                />
                              )}
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>

              </div>
            </div>
          )}
        </section>
      </div>
    </div>
  );

  return (
    <>
      {embedded ? panel_content : (
        <WorkspaceSurfaceScaffold
          body_scrollable
          stable_gutter
          header={(
            <WorkspaceSurfaceHeader
              active_tab="providers"
              density="compact"
              leading={<Cable className="h-4 w-4" />}
              tabs={SETTINGS_TABS.map((item) => ({ key: item.key, label: t(item.label_key) }))}
              title={t("settings.title")}
            />
          )}
        >
          {panel_content}
        </WorkspaceSurfaceScaffold>
      )}

      <FeedbackBannerStack
        items={feedback ? [{
          key: "feedback",
          message: feedback.message,
          on_dismiss: () => set_feedback(null),
          title: feedback.title,
          tone: feedback.tone,
        }] : []}
      />

      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={delete_confirm_open}
        message={t("settings.providers.delete_confirm_runtime_message", {
          name: delete_target_record ? get_provider_title(delete_target_record) : "",
        })}
        on_cancel={() => {
          set_delete_confirm_open(false);
          set_delete_usage_open(false);
          set_delete_target_provider(null);
        }}
        on_confirm={() => {
          void handle_delete();
        }}
        title={t("settings.providers.delete_provider")}
        variant="danger"
      />

      {delete_usage_open && delete_target_record ? (
        <UiDialogPortal>
          <UiDialogBackdrop
            class_name="z-[9999]"
            labelled_by="provider-delete-blocked-title"
            on_close={() => {
              set_delete_usage_open(false);
              set_delete_target_provider(null);
            }}
          >
            <UiDialogShell size="sm">
              <UiDialogHeader
                icon={<Trash2 className="h-4.5 w-4.5" />}
                on_close={() => {
                  set_delete_usage_open(false);
                  set_delete_target_provider(null);
                }}
                subtitle={t("settings.providers.delete_usage_subtitle", { name: get_provider_title(delete_target_record) })}
                title={t("settings.providers.delete_usage_title")}
                title_id="provider-delete-blocked-title"
              />
              <UiDialogBody class_name="space-y-3">
                <div className="rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-muted-background) px-3 py-2 text-[12px] leading-5 text-(--text-muted)">
                  {t("settings.providers.force_delete_description")}
                </div>
                {delete_usage_agents.length > 0 ? (
                  <div className="max-h-64 overflow-y-auto rounded-[12px] border border-(--divider-subtle-color)">
                    {delete_usage_agents.map((agent) => (
                      <div
                        className="flex min-h-11 items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-2 last:border-b-0"
                        key={agent.agent_id}
                      >
                        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[9px] border border-(--divider-subtle-color) bg-(--background) text-[11px] font-semibold text-(--text-muted)">
                          {(get_usage_agent_title(agent).slice(0, 2) || "AG").toUpperCase()}
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex min-w-0 items-center gap-2">
                            <span className="truncate text-[13px] font-semibold text-(--text-strong)">
                              {get_usage_agent_title(agent)}
                            </span>
                            {agent.is_main ? (
                              <span className="rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-semibold text-(--text-muted)">
                                {t("settings.providers.main_agent_badge")}
                              </span>
                            ) : null}
                          </div>
                          <div className="truncate font-mono text-[11px] text-(--text-soft)">
                            {agent.agent_id}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-[12px] border border-(--divider-subtle-color) px-3 py-3 text-[12px] leading-5 text-(--text-muted)">
                    {t("settings.providers.delete_usage_stale", { count: delete_target_record.usage_count })}
                  </div>
                )}
              </UiDialogBody>
              <UiDialogFooter>
                <UiButton
                  onClick={() => {
                    set_delete_usage_open(false);
                    set_delete_target_provider(null);
                  }}
                  type="button"
                  variant="surface"
                >
                  {t("common.cancel")}
                </UiButton>
                <UiButton
                  disabled={submitting}
                  onClick={() => {
                    void handle_delete(true);
                  }}
                  tone="danger"
                  type="button"
                  variant="solid"
                >
                  {t("settings.providers.force_delete")}
                </UiButton>
              </UiDialogFooter>
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}

      {add_model_open ? (
        <UiDialogPortal>
          <UiDialogBackdrop
            class_name="z-[9999]"
            labelled_by="provider-add-model-title"
            on_close={() => set_add_model_open(false)}
          >
            <UiDialogFormShell
              class_name="max-w-[520px]"
              onSubmit={(event) => {
                event.preventDefault();
                void handle_add_model();
              }}
              size="md"
            >
              <UiDialogHeader
                icon={<ListPlus className="h-4.5 w-4.5" />}
                on_close={() => set_add_model_open(false)}
                subtitle={t("settings.providers.add_model_subtitle")}
                title={t("settings.providers.add_model_title")}
                title_id="provider-add-model-title"
              />
              <UiDialogBody class_name="space-y-4">
                <UiField
                  description={t("settings.providers.add_model_description")}
                  label={t("settings.providers.model_id")}
                >
                  <UiInput
                    autoCapitalize="off"
                    autoCorrect="off"
                    autoFocus
                    control_size="lg"
                    class_name="font-mono"
                    onChange={(event) => set_manual_model_id(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        event.preventDefault();
                        void handle_add_model();
                      }
                    }}
                    placeholder={manual_model_placeholder}
                    spellCheck={false}
                    type="text"
                    value={manual_model_id}
                  />
                </UiField>
                <div className="flex items-center justify-between gap-3 rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_76%,transparent)] px-3.5 py-3">
                  <div className="min-w-0">
                    <div className="text-[13px] font-semibold text-(--text-strong)">
                      {t("settings.providers.enable_after_add")}
                    </div>
                    <div className="mt-0.5 text-[11px] leading-4 text-(--text-muted)">
                      {t("settings.providers.enable_after_add_description")}
                    </div>
                  </div>
                  <GlassSwitch
                    checked={manual_model_enabled}
                    size="xs"
                    on_change={set_manual_model_enabled}
                  />
                </div>
              </UiDialogBody>
              <UiDialogFooter>
                <UiButton
                  onClick={() => set_add_model_open(false)}
                  type="button"
                  variant="surface"
                >
                  {t("common.cancel")}
                </UiButton>
                <UiButton
                  disabled={!manual_model_id.trim() || pending_action?.startsWith("add-model:") || !selected_can_manage}
                  onClick={() => void handle_add_model()}
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  {pending_action?.startsWith("add-model:") ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ListPlus className="h-3.5 w-3.5" />}
                  {manual_model_enabled
                    ? t("settings.providers.add_and_enable")
                    : t("settings.providers.add")}
                </UiButton>
              </UiDialogFooter>
            </UiDialogFormShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}

      {model_options ? (
        <UiDialogPortal>
          <UiDialogBackdrop
            class_name="z-[9999]"
            labelled_by="provider-model-options-title"
            on_close={() => set_model_options(null)}
          >
            <UiDialogShell class_name="max-w-[620px]" size="lg">
              <UiDialogHeader
                icon={<SlidersHorizontal className="h-4.5 w-4.5" />}
                icon_class_name="rounded-[12px]"
                on_close={() => set_model_options(null)}
                subtitle={(
                  <span className="inline-flex min-w-0 items-center gap-1.5">
                    <span>{t("settings.providers.model_options_subtitle")}</span>
                    <code className="max-w-[260px] truncate rounded-[7px] bg-(--surface-muted-background) px-1.5 py-0.5 font-mono text-[11px] text-(--text-default)">
                      {model_options.model.model_id}
                    </code>
                  </span>
                )}
                title={t("settings.providers.model_options")}
                title_id="provider-model-options-title"
              />
              <UiDialogBody class_name="space-y-5" scrollable>
                <section className="space-y-2.5">
                  <div>
                    <h3 className="text-[13px] font-semibold text-(--text-strong)">
                      {t("settings.providers.model_capabilities")}
                    </h3>
                    <p className="mt-0.5 text-[11px] leading-4 text-(--text-muted)">
                      {t("settings.providers.model_capabilities_description")}
                    </p>
                  </div>
                  <div className="grid gap-2.5 md:grid-cols-2">
                    <CapabilitySwitch
                      checked={!!model_options.capabilities.vision}
                      icon={<Eye className="h-3.5 w-3.5" />}
                      label={t("settings.providers.capability_vision")}
                      on_change={(checked) => set_model_options((current) => current ? ({
                        ...current,
                        capabilities: { ...current.capabilities, vision: checked },
                      }) : current)}
                    />
                    <CapabilitySwitch
                      checked={!!model_options.capabilities.image_output}
                      icon={<Image className="h-3.5 w-3.5" />}
                      label={t("settings.providers.capability_image_output")}
                      on_change={(checked) => set_model_options((current) => current ? ({
                        ...current,
                        capabilities: { ...current.capabilities, image_output: checked },
                      }) : current)}
                    />
                    <CapabilitySwitch
                      checked={!!model_options.capabilities.tool_calling}
                      icon={<Wrench className="h-3.5 w-3.5" />}
                      label={t("settings.providers.capability_tool_calling")}
                      on_change={(checked) => set_model_options((current) => current ? ({
                        ...current,
                        capabilities: { ...current.capabilities, tool_calling: checked },
                      }) : current)}
                    />
                    <CapabilitySwitch
                      checked={!!model_options.capabilities.reasoning}
                      icon={<Brain className="h-3.5 w-3.5" />}
                      label={t("settings.providers.capability_reasoning")}
                      on_change={(checked) => set_model_options((current) => current ? ({
                        ...current,
                        capabilities: { ...current.capabilities, reasoning: checked },
                      }) : current)}
                    />
                    <CapabilitySwitch
                      checked={!!model_options.capabilities.embedding}
                      icon={<Database className="h-3.5 w-3.5" />}
                      label={t("settings.providers.capability_embedding")}
                      on_change={(checked) => set_model_options((current) => current ? ({
                        ...current,
                        capabilities: { ...current.capabilities, embedding: checked },
                      }) : current)}
                    />
                  </div>
                </section>

                <section className="grid gap-3 md:grid-cols-2">
                  <label className="space-y-1.5">
                    <span className="text-[12px] font-medium text-(--text-muted)">
                      {t("settings.providers.context_window")}
                    </span>
                    <UiInput
                      control_size="sm"
                      inputMode="numeric"
                      onChange={(event) => set_model_options((current) => current ? ({ ...current, context_window: event.target.value }) : current)}
                      placeholder="auto"
                      value={model_options.context_window}
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-[12px] font-medium text-(--text-muted)">
                      {t("settings.providers.max_output_tokens")}
                    </span>
                    <UiInput
                      control_size="sm"
                      inputMode="numeric"
                      onChange={(event) => set_model_options((current) => current ? ({ ...current, max_output_tokens: event.target.value }) : current)}
                      placeholder="auto"
                      value={model_options.max_output_tokens}
                    />
                  </label>
                </section>

                <label className="block space-y-1.5">
                  <span className="text-[12px] font-medium text-(--text-muted)">
                    {t("settings.providers.provider_options_json")}
                  </span>
                  <UiTextarea
                    class_name="min-h-28 font-mono text-[12px] leading-5"
                    control_size="md"
                    onChange={(event) => set_model_options((current) => current ? ({ ...current, provider_options_text: event.target.value }) : current)}
                    spellCheck={false}
                    value={model_options.provider_options_text}
                  />
                </label>
              </UiDialogBody>
              <UiDialogFooter class_name="gap-2">
                <UiButton
                  onClick={() => set_model_options(null)}
                  size="sm"
                  type="button"
                  variant="surface"
                >
                  {t("common.cancel")}
                </UiButton>
                <UiButton
                  disabled={pending_action?.startsWith("options:") || !selected_can_manage}
                  onClick={() => void handle_save_model_options()}
                  size="sm"
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  {pending_action?.startsWith("options:") ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : t("common.save")}
                </UiButton>
              </UiDialogFooter>
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}
    </>
  );
}

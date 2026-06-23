"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Cable,
} from "lucide-react";

import { invalidate_provider_availability } from "@/hooks/capability/use-provider-availability";
import {
  create_provider_config_api,
  delete_provider_config_api,
  list_provider_configs_api,
  list_provider_presets_api,
  update_provider_config_api,
} from "@/lib/api/provider-config-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
  ProviderPreset,
} from "@/types/capability/provider";

import { ProviderAddModelDialog } from "./provider-settings/provider-settings-add-model-dialog";
import { ProviderSettingsConfigForm } from "./provider-settings/provider-settings-config-form";
import { ProviderDeleteUsageDialog } from "./provider-settings/provider-settings-delete-usage-dialog";
import { ProviderSettingsDetailHeader } from "./provider-settings/provider-settings-detail-header";
import { ProviderSettingsModelList } from "./provider-settings/provider-settings-model-list";
import { ProviderModelOptionsDialog } from "./provider-settings/provider-settings-model-options-dialog";
import { ProviderSettingsSidebar } from "./provider-settings/provider-settings-sidebar";
import { useProviderModelActions } from "./provider-settings/use-provider-model-actions";
import {
  API_FORMAT_LABELS,
  DEFAULT_AGENT_API_FORMAT,
  FeedbackState,
  FormMode,
  ProviderDraft,
  SETTINGS_TABS,
  SUPPORTED_AGENT_API_FORMATS,
  build_provider_draft,
  build_provider_payload_from_draft,
  first_builtin_preset_key,
  format_supports_provider_kind,
  get_effective_models_path,
  get_provider_draft_error,
  get_provider_title,
  get_preset_format,
  get_supported_preset_format,
  is_custom_provider_record,
  normalize_custom_provider_key,
  order_provider_records,
  preset_allows_non_runtime_config,
  preset_provider_kinds,
  preset_uses_builtin_endpoint,
  provider_draft_has_changes,
  provider_for_preset,
  to_provider_draft,
} from "./provider-settings/provider-settings-model";

interface ProviderSettingsPanelProps {
  embedded?: boolean;
}

export function ProviderSettingsPanel({ embedded = false }: ProviderSettingsPanelProps) {
  const { t } = useI18n();
  const [presets, set_presets] = useState<ProviderPreset[]>([]);
  const [providers, set_providers] = useState<ProviderConfigRecord[]>([]);
  const [selected_provider, set_selected_provider] = useState<string | null>(null);
  const [mode, set_mode] = useState<FormMode>("empty");
  const [draft, set_draft] = useState<ProviderDraft>(build_provider_draft([]));
  const [loading, set_loading] = useState(true);
  const [submitting, set_submitting] = useState(false);
  const [pending_action, set_pending_action] = useState<string | null>(null);
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);
  const [delete_confirm_open, set_delete_confirm_open] = useState(false);
  const [delete_usage_open, set_delete_usage_open] = useState(false);
  const [delete_target_provider, set_delete_target_provider] = useState<string | null>(null);
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

  const {
    add_model_open,
    displayed_models,
    handle_add_model,
    handle_fetch_models,
    handle_open_add_model,
    handle_save_model_options,
    handle_test_selection,
    handle_toggle_model,
    manual_model_enabled,
    manual_model_id,
    manual_model_placeholder,
    model_options,
    model_query,
    reset_model_controls,
    set_add_model_open,
    set_manual_model_enabled,
    set_manual_model_id,
    set_model_options,
    set_model_options_from_record,
    set_model_query,
    test_model_options,
  } = useProviderModelActions({
    api_format: draft.api_format,
    pending_action,
    refresh_all,
    save_provider: handle_save,
    selected_can_manage,
    selected_record,
    set_feedback,
    set_pending_action,
    t,
  });

  const handle_select_provider = useCallback((provider: string) => {
    const target = providers.find((item) => item.provider === provider);
    if (!target) {
      return;
    }
    set_mode("edit");
    set_selected_provider(target.provider);
    reset_model_controls();
    set_draft(to_provider_draft(target));
  }, [providers, reset_model_controls]);

  const handle_create_from_preset = useCallback((preset_key: string) => {
    set_mode("create");
    set_selected_provider(null);
    reset_model_controls();
    set_draft(build_provider_draft(presets, preset_key));
  }, [presets, reset_model_controls]);

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
  const builtin_endpoint_formats = uses_builtin_endpoint ? current_preset?.formats ?? [] : [];
  const panel_content = (
    <div className={cn("mx-auto flex h-full min-h-0 w-full flex-col px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <div className="flex min-h-0 flex-1 items-stretch gap-5 overflow-hidden">
        <ProviderSettingsSidebar
          configured_by_preset={configured_by_preset}
          custom_providers={custom_providers}
          draft_preset_key={draft.preset_key}
          is_creating={is_creating}
          is_editing={is_editing}
          loading={loading}
          on_create_from_preset={handle_create_from_preset}
          on_request_delete_provider={handle_request_delete_provider}
          on_select_provider={handle_select_provider}
          pending_action={pending_action}
          preset_sidebar_items={preset_sidebar_items}
          selected_provider={selected_provider}
          submitting={submitting}
        />

        <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          {is_empty_mode ? null : (
            <div className="flex min-h-0 flex-1 flex-col bg-transparent px-5 py-2">
              <ProviderSettingsDetailHeader
                detail_title={detail_title}
                enabled={draft.enabled}
                has_selected_record={!!selected_record}
                is_api_format_configurable={is_api_format_configurable}
                is_editing={is_editing}
                on_enabled_change={handle_enabled_change}
                on_test_selection={handle_test_selection}
                pending_action={pending_action}
                preset_description={current_preset?.description}
                selected_can_manage={selected_can_manage}
                submitting={submitting}
                test_model_options={test_model_options}
              />

              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <ProviderSettingsConfigForm
                  builtin_endpoint_formats={builtin_endpoint_formats}
                  current_format={current_format}
                  current_preset={current_preset}
                  detail_title={detail_title}
                  draft={draft}
                  format_options={format_options}
                  is_custom_provider={is_custom_provider}
                  is_editing={is_editing}
                  on_api_format_change={handle_api_format_change}
                  on_auth_token_change={(value) => set_draft((current) => ({ ...current, auth_token: value }))}
                  on_base_url_change={(value) => set_draft((current) => ({ ...current, base_url: value }))}
                  on_field_blur={handle_provider_field_blur}
                  on_provider_display_name_change={(next_name) => {
                    set_draft((current) => ({
                      ...current,
                      display_name: next_name,
                      provider: is_creating ? normalize_custom_provider_key(next_name) : current.provider,
                    }));
                  }}
                  on_provider_kind_change={handle_provider_kind_change}
                  provider_kind_options={provider_kind_options}
                  selected_can_manage={selected_can_manage}
                  selected_record={selected_record}
                  show_provider_shape_controls={show_provider_shape_controls}
                  show_runtime_format_badge={show_runtime_format_badge}
                  uses_builtin_endpoint={uses_builtin_endpoint}
                />

                <ProviderSettingsModelList
                  displayed_models={displayed_models}
                  has_models_endpoint={has_models_endpoint}
                  is_api_format_configurable={is_api_format_configurable}
                  is_editing={is_editing}
                  model_query={model_query}
                  on_default_model_disable_attempt={(model) => {
                    const display_name = model.display_name || model.model_id;
                    set_feedback({
                      tone: "error",
                      title: t("settings.providers.default_model_disable_title"),
                      message: t("settings.providers.default_model_disable_message", { model: display_name }),
                    });
                  }}
                  on_fetch_models={() => void handle_fetch_models()}
                  on_model_options={set_model_options_from_record}
                  on_model_query_change={set_model_query}
                  on_open_add_model={handle_open_add_model}
                  on_toggle_model={(model, checked) => void handle_toggle_model(model, checked)}
                  pending_action={pending_action}
                  selected_can_manage={selected_can_manage}
                  selected_record={selected_record}
                />

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

      <ProviderDeleteUsageDialog
        delete_target_record={delete_target_record}
        is_open={delete_usage_open}
        on_cancel={() => {
          set_delete_usage_open(false);
          set_delete_target_provider(null);
        }}
        on_force_delete={() => {
          void handle_delete(true);
        }}
        submitting={submitting}
      />

      <ProviderAddModelDialog
        is_open={add_model_open}
        manual_model_enabled={manual_model_enabled}
        manual_model_id={manual_model_id}
        manual_model_placeholder={manual_model_placeholder}
        on_add={() => void handle_add_model()}
        on_close={() => set_add_model_open(false)}
        pending_action={pending_action}
        selected_can_manage={selected_can_manage}
        set_manual_model_enabled={set_manual_model_enabled}
        set_manual_model_id={set_manual_model_id}
      />

      <ProviderModelOptionsDialog
        model_options={model_options}
        on_close={() => set_model_options(null)}
        on_save={() => void handle_save_model_options()}
        pending_action={pending_action}
        selected_can_manage={selected_can_manage}
        set_model_options={set_model_options}
      />
    </>
  );
}

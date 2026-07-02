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
  const [presets, setPresets] = useState<ProviderPreset[]>([]);
  const [providers, setProviders] = useState<ProviderConfigRecord[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);
  const [mode, setMode] = useState<FormMode>("empty");
  const [draft, setDraft] = useState<ProviderDraft>(build_provider_draft([]));
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteUsageOpen, setDeleteUsageOpen] = useState(false);
  const [deleteTargetProvider, setDeleteTargetProvider] = useState<string | null>(null);
  const providersRef = useRef<ProviderConfigRecord[]>([]);
  const selectedProviderRef = useRef<string | null>(null);
  const savePromiseRef = useRef<Promise<ProviderConfigRecord | null> | null>(null);

  useEffect(() => {
    providersRef.current = providers;
  }, [providers]);

  useEffect(() => {
    selectedProviderRef.current = selectedProvider;
  }, [selectedProvider]);

  const selectedRecord = useMemo(
    () => providers.find((item) => item.provider === selectedProvider) ?? null,
    [providers, selectedProvider],
  );
  const deleteTargetRecord = useMemo(
    () => providers.find((item) => item.provider === deleteTargetProvider) ?? null,
    [deleteTargetProvider, providers],
  );
  const currentPreset = useMemo(
    () => presets.find((item) => item.preset_key === draft.preset_key) ?? presets.find((item) => item.preset_key === "custom") ?? null,
    [draft.preset_key, presets],
  );
  const providerKindOptions = useMemo(() => {
    const availableKinds = preset_provider_kinds(currentPreset);
    const orderedKinds: ProviderKind[] = ["llm", "image_generation"];
    return orderedKinds
      .filter((kind) => availableKinds.length === 0 || availableKinds.includes(kind))
      .map((kind) => ({
        value: kind,
        label: kind === "image_generation"
          ? t("settings.providers.kind_image_generation")
          : t("settings.providers.kind_llm"),
      }));
  }, [currentPreset, t]);
  const canSelectNonRuntimeFormat = draft.provider_kind === "llm" && preset_allows_non_runtime_config(currentPreset);
  const formatOptions = useMemo(
    () => {
      const seen = new Set<ProviderApiFormat>();
      return (currentPreset?.formats ?? [])
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
            label: supported || canSelectNonRuntimeFormat
              ? API_FORMAT_LABELS[item.api_format]
              : `${API_FORMAT_LABELS[item.api_format]}${t("settings.providers.unsupported_suffix")}`,
            disabled: !supported && !canSelectNonRuntimeFormat,
          };
        });
    },
    [canSelectNonRuntimeFormat, currentPreset, draft.provider_kind, t],
  );
  const isEditing = mode === "edit" && !!selectedRecord;
  const isCreating = mode === "create";
  const isEmptyMode = mode === "empty";
  const selectedCanManage = !isEditing || selectedRecord?.can_manage !== false;
  const canSave = useMemo(() => {
    if (isEmptyMode || !selectedCanManage) {
      return false;
    }
    return get_provider_draft_error(draft, currentPreset, isCreating, t) === null;
  }, [currentPreset, draft, isCreating, isEmptyMode, selectedCanManage, t]);

  const refreshAll = useCallback(async (preferredProvider?: string | null) => {
    try {
      const [nextPresets, nextProviders] = await Promise.all([
        list_provider_presets_api(),
        list_provider_configs_api(),
      ]);
      setPresets(nextPresets);
      const orderedItems = order_provider_records(nextProviders, providersRef.current);
      setProviders(orderedItems);
      invalidate_provider_availability();
      const target = orderedItems.find((item) => item.provider === preferredProvider)
        ?? orderedItems.find((item) => item.provider === selectedProviderRef.current);
      if (target) {
        setMode("edit");
        setSelectedProvider(target.provider);
        setDraft(to_provider_draft(target));
      } else {
        const firstPresetKey = first_builtin_preset_key(nextPresets);
        const presetTarget = firstPresetKey
          ? provider_for_preset(orderedItems, firstPresetKey)
          : null;
        if (presetTarget) {
          setMode("edit");
          setSelectedProvider(presetTarget.provider);
          setDraft(to_provider_draft(presetTarget));
        } else {
          setMode("create");
          setSelectedProvider(null);
          setDraft(build_provider_draft(nextPresets, firstPresetKey ?? "custom"));
        }
      }
      setFeedback((current) => (current?.tone === "error" ? null : current));
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.load_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.retry_later"),
      });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void refreshAll();
  }, [refreshAll]);

  const handleProviderKindChange = useCallback((value: string) => {
    const providerKind = value as ProviderKind;
    setDraft((current) => {
      const currentFormat = get_preset_format(currentPreset, current.api_format);
      const format = currentFormat && format_supports_provider_kind(currentFormat, providerKind)
        ? currentFormat
        : get_supported_preset_format(currentPreset, providerKind);
      const apiFormat = format?.api_format
        ?? (providerKind === "image_generation" ? "chat_completions" : DEFAULT_AGENT_API_FORMAT);
      return {
        ...current,
        provider_kind: providerKind,
        api_format: apiFormat,
        base_url: format?.base_url ?? current.base_url,
        models_path: format?.models_path ?? current.models_path,
      };
    });
  }, [currentPreset]);

  const handleApiFormatChange = useCallback((value: string) => {
    const apiFormat = value as ProviderApiFormat;
    const format = get_preset_format(currentPreset, apiFormat);
    const supported = format ? format_supports_provider_kind(format, draft.provider_kind) : false;
    if (!supported && !canSelectNonRuntimeFormat) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.api_format_unsupported_title"),
        message: t("settings.providers.api_format_unsupported_message"),
      });
      return;
    }
    setDraft((current) => ({
      ...current,
      api_format: apiFormat,
      base_url: format?.base_url ?? current.base_url,
      models_path: format?.models_path ?? current.models_path,
    }));
  }, [canSelectNonRuntimeFormat, currentPreset, draft.provider_kind, t]);

  const handleSave = useCallback(async (options?: {
    draft_overrides?: Partial<ProviderDraft>;
    show_error?: boolean;
    show_success?: boolean;
  }): Promise<ProviderConfigRecord | null> => {
    if (isEmptyMode) {
      return null;
    }
    if (isEditing && selectedRecord?.can_manage === false) {
      return selectedRecord;
    }
    if (savePromiseRef.current) {
      return savePromiseRef.current;
    }
    const nextDraft: ProviderDraft = {
      ...draft,
      ...options?.draft_overrides,
    };
    const showError = options?.show_error ?? true;
    const showSuccess = options?.show_success ?? false;
    const validationError = get_provider_draft_error(nextDraft, currentPreset, isCreating, t);
    if (validationError) {
      if (showError) {
        setFeedback({
          tone: "error",
          title: t("settings.providers.config_incomplete_title"),
          message: validationError,
        });
      }
      return null;
    }
    if (isEditing && !provider_draft_has_changes(nextDraft, selectedRecord, currentPreset)) {
      return selectedRecord;
    }
    const savePromise = (async () => {
      setSubmitting(true);
      try {
        const payload = build_provider_payload_from_draft(nextDraft, currentPreset);
        const normalizedAuthToken = nextDraft.auth_token.trim();
        if (normalizedAuthToken) {
          payload.auth_token = normalizedAuthToken;
        }
        const result = isEditing && selectedRecord
          ? await update_provider_config_api(selectedRecord.provider, payload)
          : await create_provider_config_api({
            ...payload,
            provider: nextDraft.provider.trim(),
            auth_token: normalizedAuthToken,
            provider_kind: nextDraft.provider_kind,
            display_name: payload.display_name,
            base_url: payload.base_url,
            enabled: payload.enabled,
          });
        await refreshAll(result.provider);
        if (showSuccess) {
          setFeedback({
            tone: "success",
            title: t("settings.providers.saved_title"),
            message: t("settings.providers.saved_message", { name: result.display_name || result.provider }),
          });
        }
        return result;
      } catch (error) {
        if (showError) {
          setFeedback({
            tone: "error",
            title: t("settings.providers.save_failed_title"),
            message: error instanceof Error ? error.message : t("settings.providers.check_config_retry"),
          });
        }
        return null;
      } finally {
        setSubmitting(false);
      }
    })();
    savePromiseRef.current = savePromise;
    try {
      return await savePromise;
    } finally {
      if (savePromiseRef.current === savePromise) {
        savePromiseRef.current = null;
      }
    }
  }, [currentPreset, draft, isCreating, isEditing, isEmptyMode, refreshAll, selectedRecord, t]);

  const handleProviderFieldBlur = useCallback(() => {
    if (!canSave || pendingAction || submitting) {
      return;
    }
    if (isEditing && !provider_draft_has_changes(draft, selectedRecord, currentPreset)) {
      return;
    }
    void handleSave({ show_error: false, show_success: false });
  }, [canSave, currentPreset, draft, handleSave, isEditing, pendingAction, selectedRecord, submitting]);

  const handleEnabledChange = useCallback((checked: boolean) => {
    if (!selectedCanManage) {
      return;
    }
    setDraft((current) => ({ ...current, enabled: checked }));
    void (async () => {
      const result = await handleSave({
        draft_overrides: { enabled: checked },
        show_error: true,
        show_success: false,
      });
      if (!result) {
        setDraft((current) => ({ ...current, enabled: !checked }));
      }
    })();
  }, [handleSave, selectedCanManage]);

  const handleRequestDeleteProvider = useCallback((item: ProviderConfigRecord) => {
    if (!is_custom_provider_record(item)) {
      return;
    }
    if (item.usage_count > 0) {
      setDeleteTargetProvider(item.provider);
      setDeleteUsageOpen(true);
      return;
    }
    setDeleteTargetProvider(item.provider);
    setDeleteConfirmOpen(true);
  }, []);

  const handleDelete = useCallback(async (force = false) => {
    if (!deleteTargetRecord || submitting) {
      return;
    }
    if (deleteTargetRecord.usage_count > 0 && !force) {
      setDeleteConfirmOpen(false);
      setDeleteUsageOpen(true);
      return;
    }
    try {
      setSubmitting(true);
      const result = await delete_provider_config_api(deleteTargetRecord.provider, { force });
      setDeleteConfirmOpen(false);
      setDeleteUsageOpen(false);
      setDeleteTargetProvider(null);
      await refreshAll();
      const replacementMessage = result.replacement_provider
        ? t("settings.providers.delete_reassigned_message", {
          count: result.reassigned_runtime_count ?? 0,
          provider: result.replacement_provider,
        })
        : t("settings.providers.delete_removed_message", { name: get_provider_title(deleteTargetRecord) });
      setFeedback({
        tone: "success",
        title: t("settings.providers.deleted_title"),
        message: replacementMessage,
      });
    } catch (error) {
      setDeleteConfirmOpen(false);
      setDeleteUsageOpen(false);
      setDeleteTargetProvider(null);
      setFeedback({
        tone: "error",
        title: t("settings.providers.delete_failed_title"),
        message: error instanceof Error ? error.message : t("settings.providers.delete_in_use_fallback"),
      });
    } finally {
      setSubmitting(false);
    }
  }, [deleteTargetRecord, refreshAll, submitting, t]);

  const {
    add_model_open: addModelOpen,
    displayed_models: displayedModels,
    handle_add_model: handleAddModel,
    handle_fetch_models: handleFetchModels,
    handle_open_add_model: handleOpenAddModel,
    handle_save_model_options: handleSaveModelOptions,
    handle_test_selection: handleTestSelection,
    handle_toggle_model: handleToggleModel,
    manual_model_enabled: manualModelEnabled,
    manual_model_id: manualModelId,
    manual_model_placeholder: manualModelPlaceholder,
    model_options: modelOptions,
    model_query: modelQuery,
    reset_model_controls: resetModelControls,
    set_add_model_open: setAddModelOpen,
    set_manual_model_enabled: setManualModelEnabled,
    set_manual_model_id: setManualModelId,
    set_model_options: setModelOptions,
    set_model_options_from_record: setModelOptionsFromRecord,
    set_model_query: setModelQuery,
    test_model_options: testModelOptions,
  } = useProviderModelActions({
    api_format: draft.api_format,
    pending_action: pendingAction,
    refresh_all: refreshAll,
    save_provider: handleSave,
    selected_can_manage: selectedCanManage,
    selected_record: selectedRecord,
    set_feedback: setFeedback,
    set_pending_action: setPendingAction,
    t,
  });

  const handleSelectProvider = useCallback((provider: string) => {
    const target = providers.find((item) => item.provider === provider);
    if (!target) {
      return;
    }
    setMode("edit");
    setSelectedProvider(target.provider);
    resetModelControls();
    setDraft(to_provider_draft(target));
  }, [providers, resetModelControls]);

  const handleCreateFromPreset = useCallback((presetKey: string) => {
    setMode("create");
    setSelectedProvider(null);
    resetModelControls();
    setDraft(build_provider_draft(presets, presetKey));
  }, [presets, resetModelControls]);

  const configuredByPreset = useMemo(() => {
    const result = new Map<string, ProviderConfigRecord>();
    for (const item of providers) {
      if (item.preset_key && item.preset_key !== "custom" && !result.has(item.preset_key)) {
        result.set(item.preset_key, item);
      }
    }
    return result;
  }, [providers]);
  const customProviders = useMemo(
    () => providers.filter((item) => item.preset_key === "custom" || !configuredByPreset.has(item.preset_key)),
    [configuredByPreset, providers],
  );
  const presetSidebarItems = presets.filter((preset) => preset.preset_key !== "custom");
  const detailTitle = isEditing && selectedRecord
    ? get_provider_title(selectedRecord)
    : draft.display_name || currentPreset?.display_name || t("settings.providers.custom_provider");
  const isCustomProvider = draft.preset_key === "custom";
  const usesBuiltinEndpoint = preset_uses_builtin_endpoint(currentPreset);
  const currentFormat = get_preset_format(currentPreset, draft.api_format);
  const currentFormatSupportsKind = currentFormat
    ? format_supports_provider_kind(currentFormat, draft.provider_kind)
    : false;
  const isApiFormatConfigurable = currentFormatSupportsKind || canSelectNonRuntimeFormat;
  const showRuntimeFormatBadge = draft.provider_kind === "llm" && !SUPPORTED_AGENT_API_FORMATS.has(draft.api_format);
  const showProviderShapeControls = isCustomProvider;
  const hasModelsEndpoint = !!get_effective_models_path(draft, currentPreset).trim();
  const builtinEndpointFormats = usesBuiltinEndpoint ? currentPreset?.formats ?? [] : [];
  const panelContent = (
    <div className={cn("mx-auto flex h-full min-h-0 w-full flex-col px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <div className="flex min-h-0 flex-1 items-stretch gap-5 overflow-hidden">
        <ProviderSettingsSidebar
          configured_by_preset={configuredByPreset}
          custom_providers={customProviders}
          draft_preset_key={draft.preset_key}
          is_creating={isCreating}
          is_editing={isEditing}
          loading={loading}
          on_create_from_preset={handleCreateFromPreset}
          on_request_delete_provider={handleRequestDeleteProvider}
          on_select_provider={handleSelectProvider}
          pending_action={pendingAction}
          preset_sidebar_items={presetSidebarItems}
          selected_provider={selectedProvider}
          submitting={submitting}
        />

        <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          {isEmptyMode ? null : (
            <div className="flex min-h-0 flex-1 flex-col bg-transparent px-5 py-2">
              <ProviderSettingsDetailHeader
                detail_title={detailTitle}
                enabled={draft.enabled}
                has_selected_record={!!selectedRecord}
                is_api_format_configurable={isApiFormatConfigurable}
                is_editing={isEditing}
                on_enabled_change={handleEnabledChange}
                on_test_selection={handleTestSelection}
                pending_action={pendingAction}
                preset_description={currentPreset?.description}
                selected_can_manage={selectedCanManage}
                submitting={submitting}
                test_model_options={testModelOptions}
              />

              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <ProviderSettingsConfigForm
                  builtin_endpoint_formats={builtinEndpointFormats}
                  current_format={currentFormat}
                  current_preset={currentPreset}
                  detail_title={detailTitle}
                  draft={draft}
                  format_options={formatOptions}
                  is_custom_provider={isCustomProvider}
                  is_editing={isEditing}
                  on_api_format_change={handleApiFormatChange}
                  on_auth_token_change={(value) => setDraft((current) => ({ ...current, auth_token: value }))}
                  on_base_url_change={(value) => setDraft((current) => ({ ...current, base_url: value }))}
                  on_field_blur={handleProviderFieldBlur}
                  on_provider_display_name_change={(nextName) => {
                    setDraft((current) => ({
                      ...current,
                      display_name: nextName,
                      provider: isCreating ? normalize_custom_provider_key(nextName) : current.provider,
                    }));
                  }}
                  on_provider_kind_change={handleProviderKindChange}
                  provider_kind_options={providerKindOptions}
                  selected_can_manage={selectedCanManage}
                  selected_record={selectedRecord}
                  show_provider_shape_controls={showProviderShapeControls}
                  show_runtime_format_badge={showRuntimeFormatBadge}
                  uses_builtin_endpoint={usesBuiltinEndpoint}
                />

                <ProviderSettingsModelList
                  displayed_models={displayedModels}
                  has_models_endpoint={hasModelsEndpoint}
                  is_api_format_configurable={isApiFormatConfigurable}
                  is_editing={isEditing}
                  model_query={modelQuery}
                  on_default_model_disable_attempt={(model) => {
                    const displayName = model.display_name || model.model_id;
                    setFeedback({
                      tone: "error",
                      title: t("settings.providers.default_model_disable_title"),
                      message: t("settings.providers.default_model_disable_message", { model: displayName }),
                    });
                  }}
                  on_fetch_models={() => void handleFetchModels()}
                  on_model_options={setModelOptionsFromRecord}
                  on_model_query_change={setModelQuery}
                  on_open_add_model={handleOpenAddModel}
                  on_toggle_model={(model, checked) => void handleToggleModel(model, checked)}
                  pending_action={pendingAction}
                  selected_can_manage={selectedCanManage}
                  selected_record={selectedRecord}
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
      {embedded ? panelContent : (
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
          {panelContent}
        </WorkspaceSurfaceScaffold>
      )}

      <FeedbackBannerStack
        items={feedback ? [{
          key: "feedback",
          message: feedback.message,
          on_dismiss: () => setFeedback(null),
          title: feedback.title,
          tone: feedback.tone,
        }] : []}
      />

      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={deleteConfirmOpen}
        message={t("settings.providers.delete_confirm_runtime_message", {
          name: deleteTargetRecord ? get_provider_title(deleteTargetRecord) : "",
        })}
        on_cancel={() => {
          setDeleteConfirmOpen(false);
          setDeleteUsageOpen(false);
          setDeleteTargetProvider(null);
        }}
        on_confirm={() => {
          void handleDelete();
        }}
        title={t("settings.providers.delete_provider")}
        variant="danger"
      />

      <ProviderDeleteUsageDialog
        delete_target_record={deleteTargetRecord}
        is_open={deleteUsageOpen}
        on_cancel={() => {
          setDeleteUsageOpen(false);
          setDeleteTargetProvider(null);
        }}
        on_force_delete={() => {
          void handleDelete(true);
        }}
        submitting={submitting}
      />

      <ProviderAddModelDialog
        is_open={addModelOpen}
        manual_model_enabled={manualModelEnabled}
        manual_model_id={manualModelId}
        manual_model_placeholder={manualModelPlaceholder}
        on_add={() => void handleAddModel()}
        on_close={() => setAddModelOpen(false)}
        pending_action={pendingAction}
        selected_can_manage={selectedCanManage}
        set_manual_model_enabled={setManualModelEnabled}
        set_manual_model_id={setManualModelId}
      />

      <ProviderModelOptionsDialog
        model_options={modelOptions}
        on_close={() => setModelOptions(null)}
        on_save={() => void handleSaveModelOptions()}
        pending_action={pendingAction}
        selected_can_manage={selectedCanManage}
        set_model_options={setModelOptions}
      />
    </>
  );
}

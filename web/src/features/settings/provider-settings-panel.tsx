"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Cable,
} from "lucide-react";

import { invalidateProviderAvailability } from "@/hooks/capability/use-provider-availability";
import {
  createProviderConfigApi,
  deleteProviderConfigApi,
  listProviderConfigsApi,
  listProviderPresetsApi,
  updateProviderConfigApi,
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
  buildProviderDraft,
  buildProviderPayloadFromDraft,
  firstBuiltinPresetKey,
  formatSupportsProviderKind,
  getEffectiveModelsPath,
  getProviderDraftError,
  getProviderTitle,
  getPresetFormat,
  getSupportedPresetFormat,
  isCustomProviderRecord,
  normalizeCustomProviderKey,
  orderProviderRecords,
  presetAllowsNonRuntimeConfig,
  presetProviderKinds,
  presetUsesBuiltinEndpoint,
  providerDraftHasChanges,
  providerForPreset,
  toProviderDraft,
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
  const [draft, setDraft] = useState<ProviderDraft>(buildProviderDraft([]));
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
    const availableKinds = presetProviderKinds(currentPreset);
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
  const canSelectNonRuntimeFormat = draft.provider_kind === "llm" && presetAllowsNonRuntimeConfig(currentPreset);
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
          const supported = formatSupportsProviderKind(item, draft.provider_kind);
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
    return getProviderDraftError(draft, currentPreset, isCreating, t) === null;
  }, [currentPreset, draft, isCreating, isEmptyMode, selectedCanManage, t]);

  const refreshAll = useCallback(async (preferredProvider?: string | null) => {
    try {
      const [nextPresets, nextProviders] = await Promise.all([
        listProviderPresetsApi(),
        listProviderConfigsApi(),
      ]);
      setPresets(nextPresets);
      const orderedItems = orderProviderRecords(nextProviders, providersRef.current);
      setProviders(orderedItems);
      invalidateProviderAvailability();
      const target = orderedItems.find((item) => item.provider === preferredProvider)
        ?? orderedItems.find((item) => item.provider === selectedProviderRef.current);
      if (target) {
        setMode("edit");
        setSelectedProvider(target.provider);
        setDraft(toProviderDraft(target));
      } else {
        const firstPresetKey = firstBuiltinPresetKey(nextPresets);
        const presetTarget = firstPresetKey
          ? providerForPreset(orderedItems, firstPresetKey)
          : null;
        if (presetTarget) {
          setMode("edit");
          setSelectedProvider(presetTarget.provider);
          setDraft(toProviderDraft(presetTarget));
        } else {
          setMode("create");
          setSelectedProvider(null);
          setDraft(buildProviderDraft(nextPresets, firstPresetKey ?? "custom"));
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
      const currentFormat = getPresetFormat(currentPreset, current.api_format);
      const format = currentFormat && formatSupportsProviderKind(currentFormat, providerKind)
        ? currentFormat
        : getSupportedPresetFormat(currentPreset, providerKind);
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
    const format = getPresetFormat(currentPreset, apiFormat);
    const supported = format ? formatSupportsProviderKind(format, draft.provider_kind) : false;
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
    draftOverrides?: Partial<ProviderDraft>;
    showError?: boolean;
    showSuccess?: boolean;
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
      ...options?.draftOverrides,
    };
    const showError = options?.showError ?? true;
    const showSuccess = options?.showSuccess ?? false;
    const validationError = getProviderDraftError(nextDraft, currentPreset, isCreating, t);
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
    if (isEditing && !providerDraftHasChanges(nextDraft, selectedRecord, currentPreset)) {
      return selectedRecord;
    }
    const savePromise = (async () => {
      setSubmitting(true);
      try {
        const payload = buildProviderPayloadFromDraft(nextDraft, currentPreset);
        const normalizedAuthToken = nextDraft.auth_token.trim();
        if (normalizedAuthToken) {
          payload.auth_token = normalizedAuthToken;
        }
        const result = isEditing && selectedRecord
          ? await updateProviderConfigApi(selectedRecord.provider, payload)
          : await createProviderConfigApi({
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
    if (isEditing && !providerDraftHasChanges(draft, selectedRecord, currentPreset)) {
      return;
    }
    void handleSave({ showError: false, showSuccess: false });
  }, [canSave, currentPreset, draft, handleSave, isEditing, pendingAction, selectedRecord, submitting]);

  const handleEnabledChange = useCallback((checked: boolean) => {
    if (!selectedCanManage) {
      return;
    }
    setDraft((current) => ({ ...current, enabled: checked }));
    void (async () => {
      const result = await handleSave({
        draftOverrides: { enabled: checked },
        showError: true,
        showSuccess: false,
      });
      if (!result) {
        setDraft((current) => ({ ...current, enabled: !checked }));
      }
    })();
  }, [handleSave, selectedCanManage]);

  const handleRequestDeleteProvider = useCallback((item: ProviderConfigRecord) => {
    if (!isCustomProviderRecord(item)) {
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
      const result = await deleteProviderConfigApi(deleteTargetRecord.provider, { force });
      setDeleteConfirmOpen(false);
      setDeleteUsageOpen(false);
      setDeleteTargetProvider(null);
      await refreshAll();
      const replacementMessage = result.replacement_provider
        ? t("settings.providers.delete_reassigned_message", {
          count: result.reassigned_runtime_count ?? 0,
          provider: result.replacement_provider,
        })
        : t("settings.providers.delete_removed_message", { name: getProviderTitle(deleteTargetRecord) });
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
    addModelOpen,
    displayedModels,
    handleAddModel,
    handleFetchModels,
    handleOpenAddModel,
    handleSaveModelOptions,
    handleTestSelection,
    handleToggleModel,
    manualModelEnabled,
    manualModelId,
    manualModelPlaceholder,
    modelOptions,
    modelQuery,
    resetModelControls,
    setAddModelOpen,
    setManualModelEnabled,
    setManualModelId,
    setModelOptions,
    setModelOptionsFromRecord,
    setModelQuery,
    testModelOptions,
  } = useProviderModelActions({
    apiFormat: draft.api_format,
    pendingAction,
    refreshAll,
    saveProvider: handleSave,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    setPendingAction,
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
    setDraft(toProviderDraft(target));
  }, [providers, resetModelControls]);

  const handleCreateFromPreset = useCallback((presetKey: string) => {
    setMode("create");
    setSelectedProvider(null);
    resetModelControls();
    setDraft(buildProviderDraft(presets, presetKey));
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
    ? getProviderTitle(selectedRecord)
    : draft.display_name || currentPreset?.display_name || t("settings.providers.custom_provider");
  const isCustomProvider = draft.preset_key === "custom";
  const usesBuiltinEndpoint = presetUsesBuiltinEndpoint(currentPreset);
  const currentFormat = getPresetFormat(currentPreset, draft.api_format);
  const currentFormatSupportsKind = currentFormat
    ? formatSupportsProviderKind(currentFormat, draft.provider_kind)
    : false;
  const isApiFormatConfigurable = currentFormatSupportsKind || canSelectNonRuntimeFormat;
  const showRuntimeFormatBadge = draft.provider_kind === "llm" && !SUPPORTED_AGENT_API_FORMATS.has(draft.api_format);
  const showProviderShapeControls = isCustomProvider;
  const hasModelsEndpoint = !!getEffectiveModelsPath(draft, currentPreset).trim();
  const builtinEndpointFormats = usesBuiltinEndpoint ? currentPreset?.formats ?? [] : [];
  const panelContent = (
    <div className={cn("mx-auto flex h-full min-h-0 w-full flex-col px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <div className="flex min-h-0 flex-1 items-stretch gap-5 overflow-hidden">
        <ProviderSettingsSidebar
          configuredByPreset={configuredByPreset}
          customProviders={customProviders}
          draftPresetKey={draft.preset_key}
          isCreating={isCreating}
          isEditing={isEditing}
          loading={loading}
          onCreateFromPreset={handleCreateFromPreset}
          onRequestDeleteProvider={handleRequestDeleteProvider}
          onSelectProvider={handleSelectProvider}
          pendingAction={pendingAction}
          presetSidebarItems={presetSidebarItems}
          selectedProvider={selectedProvider}
          submitting={submitting}
        />

        <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          {isEmptyMode ? null : (
            <div className="flex min-h-0 flex-1 flex-col bg-transparent px-5 py-2">
              <ProviderSettingsDetailHeader
                detailTitle={detailTitle}
                enabled={draft.enabled}
                hasSelectedRecord={!!selectedRecord}
                isApiFormatConfigurable={isApiFormatConfigurable}
                isEditing={isEditing}
                onEnabledChange={handleEnabledChange}
                onTestSelection={handleTestSelection}
                pendingAction={pendingAction}
                presetDescription={currentPreset?.description}
                selectedCanManage={selectedCanManage}
                submitting={submitting}
                testModelOptions={testModelOptions}
              />

              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <ProviderSettingsConfigForm
                  builtinEndpointFormats={builtinEndpointFormats}
                  currentFormat={currentFormat}
                  currentPreset={currentPreset}
                  detailTitle={detailTitle}
                  draft={draft}
                  formatOptions={formatOptions}
                  isCustomProvider={isCustomProvider}
                  isEditing={isEditing}
                  onApiFormatChange={handleApiFormatChange}
                  onAuthTokenChange={(value) => setDraft((current) => ({ ...current, auth_token: value }))}
                  onBaseUrlChange={(value) => setDraft((current) => ({ ...current, base_url: value }))}
                  onFieldBlur={handleProviderFieldBlur}
                  onProviderDisplayNameChange={(nextName) => {
                    setDraft((current) => ({
                      ...current,
                      display_name: nextName,
                      provider: isCreating ? normalizeCustomProviderKey(nextName) : current.provider,
                    }));
                  }}
                  onProviderKindChange={handleProviderKindChange}
                  providerKindOptions={providerKindOptions}
                  selectedCanManage={selectedCanManage}
                  selectedRecord={selectedRecord}
                  showProviderShapeControls={showProviderShapeControls}
                  showRuntimeFormatBadge={showRuntimeFormatBadge}
                  usesBuiltinEndpoint={usesBuiltinEndpoint}
                />

                <ProviderSettingsModelList
                  displayedModels={displayedModels}
                  hasModelsEndpoint={hasModelsEndpoint}
                  isApiFormatConfigurable={isApiFormatConfigurable}
                  isEditing={isEditing}
                  modelQuery={modelQuery}
                  onDefaultModelDisableAttempt={(model) => {
                    const displayName = model.display_name || model.model_id;
                    setFeedback({
                      tone: "error",
                      title: t("settings.providers.default_model_disable_title"),
                      message: t("settings.providers.default_model_disable_message", { model: displayName }),
                    });
                  }}
                  onFetchModels={() => void handleFetchModels()}
                  onModelOptions={setModelOptionsFromRecord}
                  onModelQueryChange={setModelQuery}
                  onOpenAddModel={handleOpenAddModel}
                  onToggleModel={(model, checked) => void handleToggleModel(model, checked)}
                  pendingAction={pendingAction}
                  selectedCanManage={selectedCanManage}
                  selectedRecord={selectedRecord}
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
          bodyScrollable
          stableGutter
          header={(
            <WorkspaceSurfaceHeader
              activeTab="providers"
              density="compact"
              leading={<Cable className="h-4 w-4" />}
              tabs={SETTINGS_TABS.map((item) => ({ key: item.key, label: t(item.labelKey) }))}
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
          onDismiss: () => setFeedback(null),
          title: feedback.title,
          tone: feedback.tone,
        }] : []}
      />

      <ConfirmDialog
        confirmText={t("common.delete")}
        isOpen={deleteConfirmOpen}
        message={t("settings.providers.delete_confirm_runtime_message", {
          name: deleteTargetRecord ? getProviderTitle(deleteTargetRecord) : "",
        })}
        onCancel={() => {
          setDeleteConfirmOpen(false);
          setDeleteUsageOpen(false);
          setDeleteTargetProvider(null);
        }}
        onConfirm={() => {
          void handleDelete();
        }}
        title={t("settings.providers.delete_provider")}
        variant="danger"
      />

      <ProviderDeleteUsageDialog
        deleteTargetRecord={deleteTargetRecord}
        isOpen={deleteUsageOpen}
        onCancel={() => {
          setDeleteUsageOpen(false);
          setDeleteTargetProvider(null);
        }}
        onForceDelete={() => {
          void handleDelete(true);
        }}
        submitting={submitting}
      />

      <ProviderAddModelDialog
        isOpen={addModelOpen}
        manualModelEnabled={manualModelEnabled}
        manualModelId={manualModelId}
        manualModelPlaceholder={manualModelPlaceholder}
        onAdd={() => void handleAddModel()}
        onClose={() => setAddModelOpen(false)}
        pendingAction={pendingAction}
        selectedCanManage={selectedCanManage}
        setManualModelEnabled={setManualModelEnabled}
        setManualModelId={setManualModelId}
      />

      <ProviderModelOptionsDialog
        modelOptions={modelOptions}
        onClose={() => setModelOptions(null)}
        onSave={() => void handleSaveModelOptions()}
        pendingAction={pendingAction}
        selectedCanManage={selectedCanManage}
        setModelOptions={setModelOptions}
      />
    </>
  );
}

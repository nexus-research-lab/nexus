"use client";

import {
  Brain,
  Eye,
  Image,
  Loader2,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Wrench,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { UiSearchInput } from "@/shared/ui/form-control";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import type {
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import {
  format_count,
  get_effective_capabilities,
} from "./provider-settings-model";

interface ProviderSettingsModelListProps {
  displayed_models: ProviderModelRecord[];
  has_models_endpoint: boolean;
  is_api_format_configurable: boolean;
  is_editing: boolean;
  model_query: string;
  on_default_model_disable_attempt: (model: ProviderModelRecord) => void;
  on_fetch_models: () => void;
  on_model_options: (model: ProviderModelRecord) => void;
  on_model_query_change: (query: string) => void;
  on_open_add_model: () => void;
  on_toggle_model: (model: ProviderModelRecord, enabled: boolean) => void;
  pending_action: string | null;
  selected_can_manage: boolean;
  selected_record: ProviderConfigRecord | null;
}

export function ProviderSettingsModelList({
  displayed_models: displayedModels,
  has_models_endpoint: hasModelsEndpoint,
  is_api_format_configurable: isApiFormatConfigurable,
  is_editing: isEditing,
  model_query: modelQuery,
  on_default_model_disable_attempt: onDefaultModelDisableAttempt,
  on_fetch_models: onFetchModels,
  on_model_options: onModelOptions,
  on_model_query_change: onModelQueryChange,
  on_open_add_model: onOpenAddModel,
  on_toggle_model: onToggleModel,
  pending_action: pendingAction,
  selected_can_manage: selectedCanManage,
  selected_record: selectedRecord,
}: ProviderSettingsModelListProps) {
  const { t } = useI18n();

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 pt-1">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-baseline gap-2">
          <h3 className="text-[14px] font-semibold tracking-tight text-(--text-strong)">
            {t("settings.providers.models")}
          </h3>
          {selectedRecord ? (
            <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-(--surface-muted-background) px-1.5 text-[11px] font-semibold text-(--text-muted)">
              {displayedModels.length}
            </span>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          {isEditing && selectedRecord ? (
            <>
              <UiButton
                disabled={pendingAction !== null || !isApiFormatConfigurable || !selectedCanManage}
                onClick={onOpenAddModel}
                size="xs"
                type="button"
                variant="surface"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("settings.providers.add_model")}
              </UiButton>
              <UiButton
                disabled={pendingAction !== null || !isApiFormatConfigurable || !selectedCanManage || !hasModelsEndpoint}
                onClick={onFetchModels}
                size="xs"
                title={!hasModelsEndpoint ? t("settings.providers.sync_models_unavailable") : undefined}
                type="button"
                variant="surface"
              >
                {pendingAction === "fetch" ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                {t("settings.providers.sync_models")}
              </UiButton>
            </>
          ) : null}
        </div>
      </div>

      <UiSearchInput
        class_name="w-full"
        control_size="md"
        on_change={onModelQueryChange}
        placeholder={t("settings.providers.search_models")}
        value={modelQuery}
        variant="dialog"
      />

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto rounded-[12px] border border-(--divider-subtle-color)">
        {!selectedRecord || displayedModels.length === 0 ? (
          <div className="flex min-h-28 items-center justify-center text-sm text-(--text-soft)">
            {selectedRecord
              ? t("settings.providers.models_empty")
              : t("settings.providers.models_after_save")}
          </div>
        ) : (
          displayedModels.map((model) => {
            const capabilities = get_effective_capabilities(model);
            const pendingModel = pendingAction?.endsWith(model.model_id) ?? false;
            const displayName = model.display_name || model.model_id;
            const showModelId = model.model_id !== displayName;
            const disableModelToggle = pendingAction !== null || !selectedCanManage || model.is_default;
            const modelToggleTitle = model.is_default
              ? t("settings.providers.default_model_disable_tooltip")
              : undefined;
            return (
              <div
                className="grid min-h-9 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 border-b border-(--divider-subtle-color) px-2.5 py-1 last:border-b-0"
                key={model.model_id}
              >
                <div className="flex min-w-0 items-center gap-2">
                  <span className="min-w-0 truncate font-mono text-[13px] leading-5 text-(--text-strong)">
                    {displayName}
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
                  {showModelId ? (
                    <span className="hidden max-w-[120px] truncate font-mono text-[11px] text-(--text-soft) xl:inline">
                      {model.model_id}
                    </span>
                  ) : null}
                  <UiIconButton
                    onClick={() => onModelOptions(model)}
                    size="xs"
                    title={t("settings.providers.model_options")}
                    type="button"
                    variant="ghost"
                  >
                    <SlidersHorizontal className="h-3.5 w-3.5" />
                  </UiIconButton>
                  {pendingModel ? (
                    <Loader2 className="h-4 w-4 animate-spin text-(--text-muted)" />
                  ) : (
                    // eslint-disable-next-line jsx-a11y/no-static-element-interactions, jsx-a11y/click-events-have-key-events -- 禁用态默认开关的点击反馈包装；键盘可达性由内部 GlassSwitch 提供
                    <span
                      onClick={() => {
                        if (model.is_default) {
                          onDefaultModelDisableAttempt(model);
                        }
                      }}
                      title={modelToggleTitle}
                    >
                      <GlassSwitch
                        checked={model.enabled}
                        disabled={disableModelToggle}
                        size="xs"
                        on_change={(checked) => onToggleModel(model, checked)}
                      />
                    </span>
                  )}
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

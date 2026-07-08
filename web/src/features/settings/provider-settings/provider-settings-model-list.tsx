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
  formatCount,
  getEffectiveCapabilities,
} from "./provider-settings-model";

interface ProviderSettingsModelListProps {
  displayedModels: ProviderModelRecord[];
  hasModelsEndpoint: boolean;
  isApiFormatConfigurable: boolean;
  isEditing: boolean;
  modelQuery: string;
  onDefaultModelDisableAttempt: (model: ProviderModelRecord) => void;
  onFetchModels: () => void;
  onModelOptions: (model: ProviderModelRecord) => void;
  onModelQueryChange: (query: string) => void;
  onOpenAddModel: () => void;
  onToggleModel: (model: ProviderModelRecord, enabled: boolean) => void;
  pendingAction: string | null;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
}

export function ProviderSettingsModelList({
  displayedModels: displayedModels,
  hasModelsEndpoint: hasModelsEndpoint,
  isApiFormatConfigurable: isApiFormatConfigurable,
  isEditing: isEditing,
  modelQuery: modelQuery,
  onDefaultModelDisableAttempt: onDefaultModelDisableAttempt,
  onFetchModels: onFetchModels,
  onModelOptions: onModelOptions,
  onModelQueryChange: onModelQueryChange,
  onOpenAddModel: onOpenAddModel,
  onToggleModel: onToggleModel,
  pendingAction: pendingAction,
  selectedCanManage: selectedCanManage,
  selectedRecord: selectedRecord,
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
        className="w-full"
        controlSize="md"
        onChange={onModelQueryChange}
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
            const capabilities = getEffectiveCapabilities(model);
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
                    <span>{formatCount(model.context_window)}</span>
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
                        onChange={(checked) => onToggleModel(model, checked)}
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

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
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import type {
  ProviderConfigRecord,
  ProviderModelCapabilities,
  ProviderModelRecord,
} from "@/types/capability/provider";

import {
  formatCount,
} from "../model/provider-settings-presentation";
import { getEffectiveCapabilities } from "../model/provider-model-model";
import type { ProviderPendingAction } from "../actions/use-provider-command";

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
  pendingAction: ProviderPendingAction | null;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
}

type ProviderCapabilityKey = keyof Pick<
  ProviderModelCapabilities,
  "image_output" | "reasoning" | "tool_calling" | "vision"
>;

const PROVIDER_CAPABILITY_ICONS: Array<{
  Icon: LucideIcon;
  key: ProviderCapabilityKey;
}> = [
  { Icon: Wrench, key: "tool_calling" },
  { Icon: Brain, key: "reasoning" },
  { Icon: Eye, key: "vision" },
  { Icon: Image, key: "image_output" },
];

function ProviderModelListHeader({
  hasModelsEndpoint,
  isApiFormatConfigurable,
  isEditing,
  modelCount,
  onFetchModels,
  onOpenAddModel,
  pendingAction,
  selectedCanManage,
  selectedRecord,
}: Pick<
  ProviderSettingsModelListProps,
  | "hasModelsEndpoint"
  | "isApiFormatConfigurable"
  | "isEditing"
  | "onFetchModels"
  | "onOpenAddModel"
  | "pendingAction"
  | "selectedCanManage"
  | "selectedRecord"
> & { modelCount: number }) {
  const { t } = useI18n();
  const actionsVisible = isEditing && selectedRecord !== null;
  const actionsDisabled = pendingAction !== null
    || !isApiFormatConfigurable
    || !selectedCanManage;
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
      <div className="flex min-w-0 items-baseline gap-2">
        <h3 className="text-[14px] font-semibold tracking-tight text-(--text-strong)">
          {t("settings.providers.models")}
        </h3>
        {selectedRecord ? (
          <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-(--surface-muted-background) px-1.5 text-[11px] font-semibold text-(--text-muted)">
            {modelCount}
          </span>
        ) : null}
      </div>
      {actionsVisible ? (
        <div className="flex items-center gap-2">
          <UiButton
            disabled={actionsDisabled}
            onClick={onOpenAddModel}
            size="xs"
            type="button"
            variant="surface"
          >
            <Plus className="h-3.5 w-3.5" />
            {t("settings.providers.add_model")}
          </UiButton>
          <UiButton
            disabled={actionsDisabled || !hasModelsEndpoint}
            onClick={onFetchModels}
            size="xs"
            title={!hasModelsEndpoint
              ? t("settings.providers.sync_models_unavailable")
              : undefined}
            type="button"
            variant="surface"
          >
            {pendingAction?.kind === "fetch-models" ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" />
            )}
            {t("settings.providers.sync_models")}
          </UiButton>
        </div>
      ) : null}
    </div>
  );
}

function ProviderModelCapabilities({ model }: { model: ProviderModelRecord }) {
  const capabilities = getEffectiveCapabilities(model);
  return (
    <span className="flex shrink-0 items-center gap-1.5 text-[10px] leading-4 text-(--text-muted)">
      {PROVIDER_CAPABILITY_ICONS.map(({ Icon, key }) => (
        capabilities[key] ? <Icon className="h-3 w-3" key={key} /> : null
      ))}
      <span>{formatCount(model.context_window)}</span>
    </span>
  );
}

function DefaultModelToggle({
  model,
  onDefaultModelDisableAttempt,
}: Pick<
  ProviderSettingsModelListProps,
  "onDefaultModelDisableAttempt"
> & { model: ProviderModelRecord }) {
  const { t } = useI18n();
  const requestDisable = () => onDefaultModelDisableAttempt(model);
  return (
    <span
      className="inline-flex"
      onClick={requestDisable}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          requestDisable();
        }
      }}
      role="button"
      tabIndex={0}
      title={t("settings.providers.default_model_disable_tooltip")}
    >
      <GlassSwitch
        checked={model.enabled}
        disabled
        size="xs"
        onChange={() => undefined}
      />
    </span>
  );
}

function ProviderModelToggle({
  model,
  onDefaultModelDisableAttempt,
  onToggleModel,
  pendingAction,
  selectedCanManage,
}: Pick<
  ProviderSettingsModelListProps,
  | "onDefaultModelDisableAttempt"
  | "onToggleModel"
  | "pendingAction"
  | "selectedCanManage"
> & { model: ProviderModelRecord }) {
  const isPending = pendingAction?.kind === "toggle-model"
    && pendingAction.modelId === model.model_id;
  if (isPending) {
    return <Loader2 className="h-4 w-4 animate-spin text-(--text-muted)" />;
  }
  if (model.is_default) {
    return (
      <DefaultModelToggle
        model={model}
        onDefaultModelDisableAttempt={onDefaultModelDisableAttempt}
      />
    );
  }
  return (
    <GlassSwitch
      checked={model.enabled}
      disabled={pendingAction !== null || !selectedCanManage}
      size="xs"
      onChange={(checked) => onToggleModel(model, checked)}
    />
  );
}

function ProviderModelRow({
  model,
  onDefaultModelDisableAttempt,
  onModelOptions,
  onToggleModel,
  pendingAction,
  selectedCanManage,
}: Pick<
  ProviderSettingsModelListProps,
  | "onDefaultModelDisableAttempt"
  | "onModelOptions"
  | "onToggleModel"
  | "pendingAction"
  | "selectedCanManage"
> & { model: ProviderModelRecord }) {
  const { t } = useI18n();
  const displayName = model.display_name || model.model_id;
  return (
    <div className="grid min-h-9 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 border-b border-(--divider-subtle-color) px-2.5 py-1 last:border-b-0">
      <div className="flex min-w-0 items-center gap-2">
        <span className="min-w-0 truncate font-mono text-[13px] leading-5 text-(--text-strong)">
          {displayName}
        </span>
        <ProviderModelCapabilities model={model} />
      </div>
      <div className="flex min-w-0 items-center gap-2">
        {model.model_id !== displayName ? (
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
        <ProviderModelToggle
          model={model}
          onDefaultModelDisableAttempt={onDefaultModelDisableAttempt}
          onToggleModel={onToggleModel}
          pendingAction={pendingAction}
          selectedCanManage={selectedCanManage}
        />
      </div>
    </div>
  );
}

function ProviderModelListBody({
  displayedModels,
  onDefaultModelDisableAttempt,
  onModelOptions,
  onToggleModel,
  pendingAction,
  selectedCanManage,
  selectedRecord,
}: Pick<
  ProviderSettingsModelListProps,
  | "displayedModels"
  | "onDefaultModelDisableAttempt"
  | "onModelOptions"
  | "onToggleModel"
  | "pendingAction"
  | "selectedCanManage"
  | "selectedRecord"
>) {
  const { t } = useI18n();
  if (!selectedRecord || displayedModels.length === 0) {
    return (
      <div className="flex min-h-28 items-center justify-center text-sm text-(--text-soft)">
        {selectedRecord
          ? t("settings.providers.models_empty")
          : t("settings.providers.models_after_save")}
      </div>
    );
  }
  return displayedModels.map((model) => (
    <ProviderModelRow
      key={model.model_id}
      model={model}
      onDefaultModelDisableAttempt={onDefaultModelDisableAttempt}
      onModelOptions={onModelOptions}
      onToggleModel={onToggleModel}
      pendingAction={pendingAction}
      selectedCanManage={selectedCanManage}
    />
  ));
}

export function ProviderSettingsModelList({
  displayedModels,
  hasModelsEndpoint,
  isApiFormatConfigurable,
  isEditing,
  modelQuery,
  onDefaultModelDisableAttempt,
  onFetchModels,
  onModelOptions,
  onModelQueryChange,
  onOpenAddModel,
  onToggleModel,
  pendingAction,
  selectedCanManage,
  selectedRecord,
}: ProviderSettingsModelListProps) {
  const { t } = useI18n();

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 pt-1">
      <ProviderModelListHeader
        hasModelsEndpoint={hasModelsEndpoint}
        isApiFormatConfigurable={isApiFormatConfigurable}
        isEditing={isEditing}
        modelCount={displayedModels.length}
        onFetchModels={onFetchModels}
        onOpenAddModel={onOpenAddModel}
        pendingAction={pendingAction}
        selectedCanManage={selectedCanManage}
        selectedRecord={selectedRecord}
      />

      <UiSearchInput
        className="w-full"
        controlSize="md"
        onChange={onModelQueryChange}
        placeholder={t("settings.providers.search_models")}
        value={modelQuery}
        variant="dialog"
      />

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto rounded-[12px] border border-(--divider-subtle-color)">
        <ProviderModelListBody
          displayedModels={displayedModels}
          onDefaultModelDisableAttempt={onDefaultModelDisableAttempt}
          onModelOptions={onModelOptions}
          onToggleModel={onToggleModel}
          pendingAction={pendingAction}
          selectedCanManage={selectedCanManage}
          selectedRecord={selectedRecord}
        />
      </div>
    </div>
  );
}

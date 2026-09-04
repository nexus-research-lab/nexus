/**
 * INPUT: 当前 Provider、模型目录、筛选和互斥命令状态。
 * OUTPUT: 模型列表、能力标识及同步、默认、删除和启停动作。
 * POS: Provider 详情内的模型目录视图；不拥有命令事务或共享加载样式。
 */
"use client";

import {
  Brain,
  Eye,
  Image,
  Loader2,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Star,
  Trash2,
  Wrench,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  onRequestDeleteModel: (model: ProviderModelRecord) => void;
  onSetDefaultModel: (model: ProviderModelRecord) => void;
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
        <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
          {t("settings.providers.models")}
        </h3>
        {selectedRecord ? (
          <UiBadge shape="pill" size="xs" tone="idle">
            {modelCount}
          </UiBadge>
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
              <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
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
    <span className={cn("flex shrink-0 items-center gap-1.5", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
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
    <GlassSwitch
      aria-label={t("settings.providers.toggle_model", {
        name: model.display_name || model.model_id,
      })}
      checked={model.enabled}
      onChange={() => requestDisable()}
      size="xs"
      title={t("settings.providers.default_model_disable_tooltip")}
    />
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
  const { t } = useI18n();
  const isPending = pendingAction?.kind === "toggle-model"
    && pendingAction.modelId === model.model_id;
  if (isPending) {
    return (
      <Loader2
        className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
      />
    );
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
      aria-label={t("settings.providers.toggle_model", {
        name: model.display_name || model.model_id,
      })}
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
  onRequestDeleteModel,
  onSetDefaultModel,
  onToggleModel,
  pendingAction,
  selectedCanManage,
  selectedRecord,
}: Pick<
  ProviderSettingsModelListProps,
  | "onDefaultModelDisableAttempt"
  | "onModelOptions"
  | "onRequestDeleteModel"
  | "onSetDefaultModel"
  | "onToggleModel"
  | "pendingAction"
  | "selectedCanManage"
  | "selectedRecord"
> & { model: ProviderModelRecord }) {
  const { t } = useI18n();
  const displayName = model.display_name || model.model_id;
  const isDeletePending = pendingAction?.kind === "delete-model"
    && pendingAction.modelId === model.model_id;
  const isDefaultPending = pendingAction?.kind === "set-default-model"
    && pendingAction.modelId === model.model_id;
  const showSubscriptionDefault = selectedRecord?.visibility === "public"
    && selectedRecord.provider_kind === "llm"
    && selectedRecord.agent_runtime_supported
    && !getEffectiveCapabilities(model).image_output;
  return (
    <div className="grid min-h-9 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 border-b border-(--divider-subtle-color) px-2.5 py-1 last:border-b-0">
      <div className="flex min-w-0 items-center gap-2">
        <span className={cn("min-w-0 truncate", getUiTypographyClassName({ role: "code", tone: "strong" }))}>
          {displayName}
        </span>
        <ProviderModelCapabilities model={model} />
      </div>
      <div className="flex min-w-0 items-center gap-2">
        {showSubscriptionDefault ? (
          model.is_default ? (
            <UiBadge
              icon={<Star className="h-3 w-3 fill-current" />}
              size="xs"
              tone="primary"
            >
              {t("settings.providers.subscription_default_model")}
            </UiBadge>
          ) : (
            <UiButton
              disabled={pendingAction !== null || !selectedCanManage}
              onClick={() => onSetDefaultModel(model)}
              size="xs"
              tone="primary"
              type="button"
              variant="ghost"
            >
              {isDefaultPending ? (
                <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              ) : (
                <Star className="h-3.5 w-3.5" />
              )}
              {t("settings.providers.set_subscription_default")}
            </UiButton>
          )
        ) : null}
        {model.model_id !== displayName ? (
          <span className={cn("hidden max-w-[120px] truncate xl:inline", getUiTypographyClassName({ role: "code", tone: "soft" }))}>
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
        {!model.is_default ? (
          <UiIconButton
            aria-label={t("settings.providers.delete_model_aria", {
              name: displayName,
            })}
            disabled={pendingAction !== null || !selectedCanManage}
            onClick={() => onRequestDeleteModel(model)}
            size="xs"
            title={t("settings.providers.delete_model")}
            tone="danger"
            type="button"
            variant="ghost"
          >
            {isDeletePending ? (
              <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
            ) : (
              <Trash2 className="h-3.5 w-3.5" />
            )}
          </UiIconButton>
        ) : null}
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
  onRequestDeleteModel,
  onSetDefaultModel,
  onToggleModel,
  pendingAction,
  selectedCanManage,
  selectedRecord,
}: Pick<
  ProviderSettingsModelListProps,
  | "displayedModels"
  | "onDefaultModelDisableAttempt"
  | "onModelOptions"
  | "onRequestDeleteModel"
  | "onSetDefaultModel"
  | "onToggleModel"
  | "pendingAction"
  | "selectedCanManage"
  | "selectedRecord"
>) {
  const { t } = useI18n();
  if (!selectedRecord || displayedModels.length === 0) {
    return (
      <div className={cn("flex min-h-28 items-center justify-center", getUiTypographyClassName({ role: "supporting", tone: "soft" }))}>
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
      onRequestDeleteModel={onRequestDeleteModel}
      onSetDefaultModel={onSetDefaultModel}
      onToggleModel={onToggleModel}
      pendingAction={pendingAction}
      selectedCanManage={selectedCanManage}
      selectedRecord={selectedRecord}
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
  onRequestDeleteModel,
  onSetDefaultModel,
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

      <div className="soft-scrollbar surface-radius-md min-h-0 flex-1 overflow-y-auto border border-(--divider-subtle-color)">
        <ProviderModelListBody
          displayedModels={displayedModels}
          onDefaultModelDisableAttempt={onDefaultModelDisableAttempt}
          onModelOptions={onModelOptions}
          onRequestDeleteModel={onRequestDeleteModel}
          onSetDefaultModel={onSetDefaultModel}
          onToggleModel={onToggleModel}
          pendingAction={pendingAction}
          selectedCanManage={selectedCanManage}
          selectedRecord={selectedRecord}
        />
      </div>
    </div>
  );
}

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
  displayed_models,
  has_models_endpoint,
  is_api_format_configurable,
  is_editing,
  model_query,
  on_default_model_disable_attempt,
  on_fetch_models,
  on_model_options,
  on_model_query_change,
  on_open_add_model,
  on_toggle_model,
  pending_action,
  selected_can_manage,
  selected_record,
}: ProviderSettingsModelListProps) {
  const { t } = useI18n();

  return (
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
                onClick={on_open_add_model}
                size="xs"
                type="button"
                variant="surface"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("settings.providers.add_model")}
              </UiButton>
              <UiButton
                disabled={pending_action !== null || !is_api_format_configurable || !selected_can_manage || !has_models_endpoint}
                onClick={on_fetch_models}
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
        on_change={on_model_query_change}
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
            const disable_model_toggle = pending_action !== null || !selected_can_manage || model.is_default;
            const model_toggle_title = model.is_default
              ? t("settings.providers.default_model_disable_tooltip")
              : undefined;
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
                    onClick={() => on_model_options(model)}
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
                    // eslint-disable-next-line jsx-a11y/no-static-element-interactions, jsx-a11y/click-events-have-key-events -- 禁用态默认开关的点击反馈包装;键盘可达性由内部 GlassSwitch 提供
                    <span
                      onClick={() => {
                        if (model.is_default) {
                          on_default_model_disable_attempt(model);
                        }
                      }}
                      title={model_toggle_title}
                    >
                      <GlassSwitch
                        checked={model.enabled}
                        disabled={disable_model_toggle}
                        size="xs"
                        on_change={(checked) => on_toggle_model(model, checked)}
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

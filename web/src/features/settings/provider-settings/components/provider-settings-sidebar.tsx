/**
 * INPUT: Provider 预设、已配置记录、当前选择与目录动作。
 * OUTPUT: 桌面纵向侧栏；窄屏上方双列、限高可滚动的服务目录。
 * POS: Provider 工作区的对象选择面，不承载配置表单。
 */
"use client";

import { ArrowDownToLine, Loader2, Plus, Trash2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import { ProviderIcon } from "./provider-settings-icon";
import {
  getProviderTitle,
  isCustomProviderRecord,
  providerHasActiveConfig,
} from "../model/provider-config-model";
import { presetIsConfigurable } from "../model/provider-preset-model";
import type { ProviderPendingAction } from "../actions/use-provider-command";

interface ProviderSettingsSidebarProps {
  configuredByPreset: Map<string, ProviderConfigRecord>;
  customProviders: ProviderConfigRecord[];
  draftPresetKey: string;
  isCreating: boolean;
  isEditing: boolean;
  loading: boolean;
  onCreateFromPreset: (presetKey: string) => void;
  onOpenCCSwitchImport: () => void;
  onRequestDeleteProvider: (item: ProviderConfigRecord) => void;
  onSelectProvider: (provider: string) => void;
  pendingAction: ProviderPendingAction | null;
  presetSidebarItems: ProviderPreset[];
  selectedProvider: string | null;
  showCCSwitchImport: boolean;
}

export function ProviderSettingsSidebar({
  configuredByPreset,
  customProviders,
  draftPresetKey,
  isCreating,
  isEditing,
  loading,
  onCreateFromPreset,
  onOpenCCSwitchImport,
  onRequestDeleteProvider,
  onSelectProvider,
  pendingAction,
  presetSidebarItems,
  selectedProvider,
  showCCSwitchImport,
}: ProviderSettingsSidebarProps) {
  const { t } = useI18n();

  return (
    <aside
      className="w-full max-w-full shrink-0 border-b border-(--divider-subtle-color) pb-2 sm:w-[190px] sm:border-b-0 sm:border-r sm:pb-0 sm:pr-4"
    >
      <div className="soft-scrollbar max-h-[180px] min-h-0 overflow-y-auto sm:h-full sm:max-h-none sm:pr-2">
        {loading ? (
          <div className="flex min-h-[260px] items-center justify-center text-(--text-soft)">
            <Loader2 className="h-4 w-4 animate-spin" />
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-1 py-2 sm:block sm:space-y-1">
            <div className="col-span-2 mb-2 grid grid-cols-2 gap-1 border-b border-(--divider-subtle-color) pb-2 sm:block sm:space-y-1">
              <button
                className={cn(
                  "flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-sm font-semibold transition-[background,color] duration-(--motion-duration-fast)",
                  isCreating && draftPresetKey === "custom"
                    ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                )}
                onClick={() => onCreateFromPreset("custom")}
                type="button"
              >
                <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px] border border-dashed border-(--surface-interactive-active-border) text-primary">
                  <Plus className="h-3.5 w-3.5" />
                </span>
                <span className="min-w-0 flex-1 truncate">{t("settings.providers.custom_provider")}</span>
              </button>

              {showCCSwitchImport ? (
                <button
                  className="flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-sm font-semibold text-(--text-default) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                  onClick={onOpenCCSwitchImport}
                  type="button"
                >
                  <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-muted-background) text-(--icon-muted)">
                    <ArrowDownToLine className="h-3.5 w-3.5" />
                  </span>
                  <span className="min-w-0 flex-1 truncate">{t("settings.providers.ccswitch_action")}</span>
                </button>
              ) : null}
            </div>

            {presetSidebarItems.map((preset) => {
              const item = configuredByPreset.get(preset.preset_key);
              const isActive = item
                ? item.provider === selectedProvider && isEditing
                : isCreating && draftPresetKey === preset.preset_key;
              const isUnsupportedPreset = !presetIsConfigurable(preset);
              return (
                <button
                  className={cn(
                    "flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-sm font-semibold transition-[background,color] duration-(--motion-duration-fast)",
                    isUnsupportedPreset
                      ? "cursor-not-allowed text-(--text-soft) opacity-50"
                      : isActive
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                      : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  disabled={isUnsupportedPreset}
                  key={preset.preset_key}
                  onClick={() => {
                    if (isUnsupportedPreset) {
                      return;
                    }
                    if (item) {
                      onSelectProvider(item.provider);
                    } else {
                      onCreateFromPreset(preset.preset_key);
                    }
                  }}
                  type="button"
                >
                  <ProviderIcon
                    active={!isUnsupportedPreset && providerHasActiveConfig(item)}
                    name={preset.display_name}
                    presetKey={preset.preset_key}
                  />
                  <span className="min-w-0 flex-1 truncate">{preset.display_name}</span>
                  {isUnsupportedPreset ? (
                    <span className="shrink-0 rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-2xs font-semibold text-(--text-soft)">
                      {t("settings.providers.unsupported_badge")}
                    </span>
                  ) : null}
                </button>
              );
            })}

            {customProviders.map((item) => {
              const isActive = item.provider === selectedProvider && isEditing;
              const canShowDelete = isCustomProviderRecord(item) && item.can_manage;
              return (
                <div
                  className={cn(
                    "group flex min-h-10 w-full items-center rounded-[10px] transition-[background,color] duration-(--motion-duration-fast)",
                    isActive
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                      : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  key={item.provider}
                >
                  <button
                    className="flex min-h-10 min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left text-sm font-semibold"
                    onClick={() => onSelectProvider(item.provider)}
                    type="button"
                  >
                    <ProviderIcon
                      active={providerHasActiveConfig(item)}
                      name={getProviderTitle(item)}
                      presetKey={item.preset_key}
                    />
                    <span className="min-w-0 flex-1 truncate">{getProviderTitle(item)}</span>
                  </button>
                  {canShowDelete ? (
                    <UiIconButton
                      aria-label={t("settings.providers.delete_aria", { name: getProviderTitle(item) })}
                      className={cn(
                        "mr-1 h-7 w-7 transition-opacity group-hover:opacity-100 focus-visible:opacity-100",
                        isActive ? "opacity-100" : "opacity-0",
                      )}
                      disabled={pendingAction !== null}
                      onClick={() => onRequestDeleteProvider(item)}
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
  );
}

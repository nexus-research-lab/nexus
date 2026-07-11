"use client";

import { Loader2, Plus, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
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
  onRequestDeleteProvider: (item: ProviderConfigRecord) => void;
  onSelectProvider: (provider: string) => void;
  pendingAction: ProviderPendingAction | null;
  presetSidebarItems: ProviderPreset[];
  selectedProvider: string | null;
}

export function ProviderSettingsSidebar({
  configuredByPreset,
  customProviders,
  draftPresetKey,
  isCreating,
  isEditing,
  loading,
  onCreateFromPreset,
  onRequestDeleteProvider,
  onSelectProvider,
  pendingAction,
  presetSidebarItems,
  selectedProvider,
}: ProviderSettingsSidebarProps) {
  const { t } = useI18n();

  return (
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

            {presetSidebarItems.map((preset) => {
              const item = configuredByPreset.get(preset.preset_key);
              const isActive = item
                ? item.provider === selectedProvider && isEditing
                : isCreating && draftPresetKey === preset.preset_key;
              const isUnsupportedPreset = !presetIsConfigurable(preset);
              return (
                <button
                  className={cn(
                    "flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-[13px] font-semibold transition-[background,color] duration-(--motion-duration-fast)",
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
                    <span className="shrink-0 rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-semibold text-(--text-soft)">
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
                    className="flex min-h-10 min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left text-[13px] font-semibold"
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

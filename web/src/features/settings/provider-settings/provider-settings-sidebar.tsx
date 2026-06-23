"use client";

import { Loader2, Plus, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import { ProviderIcon } from "./provider-settings-icon";
import {
  get_provider_title,
  is_custom_provider_record,
  preset_is_configurable,
  provider_has_active_config,
} from "./provider-settings-model";

interface ProviderSettingsSidebarProps {
  configured_by_preset: Map<string, ProviderConfigRecord>;
  custom_providers: ProviderConfigRecord[];
  draft_preset_key: string;
  is_creating: boolean;
  is_editing: boolean;
  loading: boolean;
  on_create_from_preset: (preset_key: string) => void;
  on_request_delete_provider: (item: ProviderConfigRecord) => void;
  on_select_provider: (provider: string) => void;
  pending_action: string | null;
  preset_sidebar_items: ProviderPreset[];
  selected_provider: string | null;
  submitting: boolean;
}

export function ProviderSettingsSidebar({
  configured_by_preset,
  custom_providers,
  draft_preset_key,
  is_creating,
  is_editing,
  loading,
  on_create_from_preset,
  on_request_delete_provider,
  on_select_provider,
  pending_action,
  preset_sidebar_items,
  selected_provider,
  submitting,
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
                is_creating && draft_preset_key === "custom"
                  ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                  : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
              )}
              onClick={() => on_create_from_preset("custom")}
              type="button"
            >
              <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px] border border-dashed border-(--surface-interactive-active-border) text-primary">
                <Plus className="h-3.5 w-3.5" />
              </span>
              <span className="min-w-0 flex-1 truncate">{t("settings.providers.custom_provider")}</span>
            </button>

            {preset_sidebar_items.map((preset) => {
              const item = configured_by_preset.get(preset.preset_key);
              const is_active = item
                ? item.provider === selected_provider && is_editing
                : is_creating && draft_preset_key === preset.preset_key;
              const is_unsupported_preset = !preset_is_configurable(preset);
              return (
                <button
                  className={cn(
                    "flex min-h-10 w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-[13px] font-semibold transition-[background,color] duration-(--motion-duration-fast)",
                    is_unsupported_preset
                      ? "cursor-not-allowed text-(--text-soft) opacity-50"
                      : is_active
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                      : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  disabled={is_unsupported_preset}
                  key={preset.preset_key}
                  onClick={() => {
                    if (is_unsupported_preset) {
                      return;
                    }
                    if (item) {
                      on_select_provider(item.provider);
                    } else {
                      on_create_from_preset(preset.preset_key);
                    }
                  }}
                  type="button"
                >
                  <ProviderIcon
                    active={!is_unsupported_preset && provider_has_active_config(item)}
                    name={preset.display_name}
                    preset_key={preset.preset_key}
                  />
                  <span className="min-w-0 flex-1 truncate">{preset.display_name}</span>
                  {is_unsupported_preset ? (
                    <span className="shrink-0 rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[10px] font-semibold text-(--text-soft)">
                      {t("settings.providers.unsupported_badge")}
                    </span>
                  ) : null}
                </button>
              );
            })}

            {custom_providers.map((item) => {
              const is_active = item.provider === selected_provider && is_editing;
              const can_show_delete = is_custom_provider_record(item) && item.can_manage;
              return (
                <div
                  className={cn(
                    "group flex min-h-10 w-full items-center rounded-[10px] transition-[background,color] duration-(--motion-duration-fast)",
                    is_active
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                      : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  key={item.provider}
                >
                  <button
                    className="flex min-h-10 min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left text-[13px] font-semibold"
                    onClick={() => on_select_provider(item.provider)}
                    type="button"
                  >
                    <ProviderIcon
                      active={provider_has_active_config(item)}
                      name={get_provider_title(item)}
                      preset_key={item.preset_key}
                    />
                    <span className="min-w-0 flex-1 truncate">{get_provider_title(item)}</span>
                  </button>
                  {can_show_delete ? (
                    <UiIconButton
                      aria-label={t("settings.providers.delete_aria", { name: get_provider_title(item) })}
                      class_name={cn(
                        "mr-1 h-7 w-7 transition-opacity group-hover:opacity-100 focus-visible:opacity-100",
                        is_active ? "opacity-100" : "opacity-0",
                      )}
                      disabled={submitting || pending_action !== null}
                      onClick={() => on_request_delete_provider(item)}
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

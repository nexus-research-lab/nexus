import { Loader2, Play } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import { UiSelectMenu } from "@/shared/ui/select-menu";

interface ProviderSettingsDetailHeaderProps {
  detail_title: string;
  enabled: boolean;
  has_selected_record: boolean;
  is_api_format_configurable: boolean;
  is_editing: boolean;
  on_enabled_change: (checked: boolean) => void;
  on_test_selection: (value: string) => void;
  pending_action: string | null;
  preset_description?: string | null;
  selected_can_manage: boolean;
  submitting: boolean;
  test_model_options: Array<{ label: string; value: string }>;
}

export function ProviderSettingsDetailHeader({
  detail_title,
  enabled,
  has_selected_record,
  is_api_format_configurable,
  is_editing,
  on_enabled_change,
  on_test_selection,
  pending_action,
  preset_description,
  selected_can_manage,
  submitting,
  test_model_options,
}: ProviderSettingsDetailHeaderProps) {
  const { t } = useI18n();

  return (
    <div className="mb-4 flex shrink-0 items-start justify-between gap-3">
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2.5">
          <h2 className="truncate text-[18px] font-semibold tracking-tight text-(--text-strong)">
            {detail_title}
          </h2>
          {has_selected_record ? (
            <span
              className={cn(
                "rounded-full px-2 py-0.5 text-[11px] font-semibold",
                enabled
                  ? "bg-[rgba(44,156,89,0.14)] text-[rgb(33,133,74)]"
                  : "bg-(--surface-muted-background) text-(--text-muted)",
              )}
            >
              {enabled
                ? t("settings.providers.status_active")
                : t("settings.providers.status_inactive")}
            </span>
          ) : null}
        </div>
        {preset_description ? (
          <p className="mt-1 max-w-2xl truncate text-[12px] leading-5 text-(--text-muted)">
            {preset_description}
          </p>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-2 pt-0.5">
        {is_editing ? (
          <UiSelectMenu
            aria_label={t("settings.providers.test_provider")}
            button_class_name="px-2"
            class_name="w-auto min-w-18"
            disabled={pending_action !== null || submitting || !is_api_format_configurable || !selected_can_manage}
            leading={pending_action?.startsWith("test") ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="h-3.5 w-3.5" />
            )}
            menu_class_name="min-w-[220px]"
            on_change={on_test_selection}
            options={test_model_options}
            placeholder={t("settings.providers.test")}
            size="xs"
            value=""
          />
        ) : null}
        <GlassSwitch
          checked={enabled}
          disabled={pending_action !== null || submitting || !is_api_format_configurable || !selected_can_manage}
          size="sm"
          on_change={on_enabled_change}
        />
      </div>
    </div>
  );
}

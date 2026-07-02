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
  detail_title: detailTitle,
  enabled,
  has_selected_record: hasSelectedRecord,
  is_api_format_configurable: isApiFormatConfigurable,
  is_editing: isEditing,
  on_enabled_change: onEnabledChange,
  on_test_selection: onTestSelection,
  pending_action: pendingAction,
  preset_description: presetDescription,
  selected_can_manage: selectedCanManage,
  submitting,
  test_model_options: testModelOptions,
}: ProviderSettingsDetailHeaderProps) {
  const { t } = useI18n();

  return (
    <div className="mb-4 flex shrink-0 items-start justify-between gap-3">
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2.5">
          <h2 className="truncate text-[18px] font-semibold tracking-tight text-(--text-strong)">
            {detailTitle}
          </h2>
          {hasSelectedRecord ? (
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
        {presetDescription ? (
          <p className="mt-1 max-w-2xl truncate text-[12px] leading-5 text-(--text-muted)">
            {presetDescription}
          </p>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-2 pt-0.5">
        {isEditing ? (
          <UiSelectMenu
            aria_label={t("settings.providers.test_provider")}
            button_class_name="px-2"
            class_name="w-auto min-w-18"
            disabled={pendingAction !== null || submitting || !isApiFormatConfigurable || !selectedCanManage}
            leading={pendingAction?.startsWith("test") ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="h-3.5 w-3.5" />
            )}
            menu_class_name="min-w-[220px]"
            on_change={onTestSelection}
            options={testModelOptions}
            placeholder={t("settings.providers.test")}
            size="xs"
            value=""
          />
        ) : null}
        <GlassSwitch
          checked={enabled}
          disabled={pendingAction !== null || submitting || !isApiFormatConfigurable || !selectedCanManage}
          size="sm"
          on_change={onEnabledChange}
        />
      </div>
    </div>
  );
}

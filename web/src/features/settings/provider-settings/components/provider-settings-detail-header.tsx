import { Loader2, Play } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import { UiSelectMenu } from "@/shared/ui/select-menu";

interface ProviderSettingsDetailHeaderProps {
  detailTitle: string;
  enabled: boolean;
  hasSelectedRecord: boolean;
  isApiFormatConfigurable: boolean;
  isEditing: boolean;
  onEnabledChange: (checked: boolean) => void;
  onTestSelection: (value: string) => void;
  pendingAction: string | null;
  presetDescription?: string | null;
  selectedCanManage: boolean;
  submitting: boolean;
  testModelOptions: Array<{ label: string; value: string }>;
}

export function ProviderSettingsDetailHeader({
  detailTitle: detailTitle,
  enabled,
  hasSelectedRecord: hasSelectedRecord,
  isApiFormatConfigurable: isApiFormatConfigurable,
  isEditing: isEditing,
  onEnabledChange: onEnabledChange,
  onTestSelection: onTestSelection,
  pendingAction: pendingAction,
  presetDescription: presetDescription,
  selectedCanManage: selectedCanManage,
  submitting,
  testModelOptions: testModelOptions,
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
            ariaLabel={t("settings.providers.test_provider")}
            buttonClassName="px-2"
            className="w-auto min-w-18"
            disabled={pendingAction !== null || submitting || !isApiFormatConfigurable || !selectedCanManage}
            leading={pendingAction?.startsWith("test") ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="h-3.5 w-3.5" />
            )}
            menuClassName="min-w-[220px]"
            onChange={onTestSelection}
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
          onChange={onEnabledChange}
        />
      </div>
    </div>
  );
}

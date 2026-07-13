import { Loader2, Play } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import type { ProviderPendingAction } from "../actions/use-provider-command";

interface ProviderSettingsDetailHeaderProps {
  detailTitle: string;
  enabled: boolean;
  hasSelectedRecord: boolean;
  isApiFormatConfigurable: boolean;
  isEditing: boolean;
  onEnabledChange: (checked: boolean) => void;
  onTestSelection: (value: string) => void;
  pendingAction: ProviderPendingAction | null;
  presetDescription?: string | null;
  selectedCanManage: boolean;
  testModelOptions: Array<{ label: string; value: string }>;
}

const TEST_PENDING_ACTIONS = new Set<ProviderPendingAction["kind"]>([
  "test-model",
  "test-provider",
]);

function ProviderStatusBadge({
  enabled,
  visible,
}: {
  enabled: boolean;
  visible: boolean;
}) {
  const { t } = useI18n();
  if (!visible) {
    return null;
  }
  return (
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
  );
}

function ProviderTestMenu({
  disabled,
  isEditing,
  onTestSelection,
  pendingAction,
  testModelOptions,
}: Pick<
  ProviderSettingsDetailHeaderProps,
  "isEditing" | "onTestSelection" | "pendingAction" | "testModelOptions"
> & { disabled: boolean }) {
  const { t } = useI18n();
  if (!isEditing) {
    return null;
  }
  const isTesting = pendingAction
    ? TEST_PENDING_ACTIONS.has(pendingAction.kind)
    : false;
  return (
    <UiSelectMenu
      ariaLabel={t("settings.providers.test_provider")}
      buttonClassName="px-2"
      className="w-auto min-w-18"
      disabled={disabled}
      leading={isTesting ? (
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
  );
}

export function ProviderSettingsDetailHeader({
  detailTitle,
  enabled,
  hasSelectedRecord,
  isApiFormatConfigurable,
  isEditing,
  onEnabledChange,
  onTestSelection,
  pendingAction,
  presetDescription,
  selectedCanManage,
  testModelOptions,
}: ProviderSettingsDetailHeaderProps) {
  const controlsDisabled = pendingAction !== null
    || !isApiFormatConfigurable
    || !selectedCanManage;

  return (
    <div className="mb-4 flex shrink-0 items-start justify-between gap-3">
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2.5">
          <h2 className="truncate text-[18px] font-semibold tracking-tight text-(--text-strong)">
            {detailTitle}
          </h2>
          <ProviderStatusBadge enabled={enabled} visible={hasSelectedRecord} />
        </div>
        {presetDescription ? (
          <p className="mt-1 max-w-2xl truncate text-[12px] leading-5 text-(--text-muted)">
            {presetDescription}
          </p>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-2 pt-0.5">
        <ProviderTestMenu
          disabled={controlsDisabled}
          isEditing={isEditing}
          onTestSelection={onTestSelection}
          pendingAction={pendingAction}
          testModelOptions={testModelOptions}
        />
        <GlassSwitch
          checked={enabled}
          disabled={controlsDisabled}
          size="sm"
          onChange={onEnabledChange}
        />
      </div>
    </div>
  );
}

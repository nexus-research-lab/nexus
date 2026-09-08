/**
 * INPUT: 精确 Provider 身份、名称、启用状态、测试选项与写权限。
 * OUTPUT: 完整名称、公共状态徽标与显式执行的 Provider 测试动作菜单。
 * POS: Provider 配置正文的对象身份栏，不承载预设营销说明。
 */
import { ChevronDown, Loader2, Play } from "lucide-react";
import { useRef } from "react";
import { UiButton } from "@/shared/ui/button/button";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";

import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
  providerId: string | null;
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
    <UiBadge
      size="sm"
      tone={enabled ? "success" : "idle"}
    >
      {enabled
        ? t("settings.providers.status_active")
        : t("settings.providers.status_inactive")}
    </UiBadge>
  );
}

function ProviderTestMenu({
  disabled,
  isEditing,
  onTestSelection,
  pendingAction,
  providerId,
  testModelOptions,
}: Pick<
  ProviderSettingsDetailHeaderProps,
  "isEditing" | "onTestSelection" | "pendingAction" | "providerId" | "testModelOptions"
> & { disabled: boolean }) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const unavailable = disabled || !providerId || testModelOptions.length === 0;
  const [isOpen, setIsOpen] = useResettableState(false,
    JSON.stringify([providerId, isEditing, unavailable, testModelOptions.map((option) => option.value).sort()]));
  if (!isEditing) return null;
  const isTesting = pendingAction ? TEST_PENDING_ACTIONS.has(pendingAction.kind) : false;
  const menuOpen = isOpen && !unavailable;
  return (
    <>
      <UiButton
        ref={buttonRef}
        aria-busy={isTesting || undefined}
        aria-expanded={menuOpen}
        aria-haspopup="menu"
        aria-label={t("settings.providers.test_provider")}
        disabled={unavailable}
        onClick={() => setIsOpen((open) => !open)}
        size="xs"
        title={t("settings.providers.test_provider")}
        variant="surface"
      >
        {isTesting ? <Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "sm" })} />
          : <Play aria-hidden="true" className="h-3.5 w-3.5" />}
        {t("settings.providers.test")}
        <ChevronDown aria-hidden="true" className="h-3.5 w-3.5" />
      </UiButton>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("settings.providers.test_provider")}
        isOpen={menuOpen}
        items={testModelOptions}
        minWidth={220}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (!unavailable && testModelOptions.some((option) => option.value === value)) onTestSelection(value);
        }}
      />
    </>
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
  providerId,
  selectedCanManage,
  testModelOptions,
}: ProviderSettingsDetailHeaderProps) {
  const { t } = useI18n();
  const controlsDisabled = pendingAction !== null
    || !isApiFormatConfigurable
    || !selectedCanManage;

  return (
    <div className="mb-4 flex shrink-0 flex-wrap items-start justify-between gap-3">
      <div className="min-w-0 flex-1 basis-48">
        <div className="flex min-w-0 flex-wrap items-center gap-2.5">
          <h2
            className={cn(
              "min-w-0 wrap-anywhere",
              getUiTypographyClassName({ role: "pageTitle", tone: "strong" }),
            )}
          >
            {detailTitle}
          </h2>
          <ProviderStatusBadge enabled={enabled} visible={hasSelectedRecord} />
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2 pt-0.5">
        <ProviderTestMenu
          disabled={controlsDisabled}
          isEditing={isEditing}
          onTestSelection={onTestSelection}
          pendingAction={pendingAction}
          providerId={providerId}
          testModelOptions={testModelOptions}
        />
        <GlassSwitch
          aria-label={t("settings.providers.toggle_provider", {
            name: detailTitle,
          })}
          checked={enabled}
          disabled={controlsDisabled}
          size="sm"
          onChange={onEnabledChange}
        />
      </div>
    </div>
  );
}

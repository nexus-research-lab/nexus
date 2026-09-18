// INPUT: 单项模型能力、当前开关值、禁用状态与更新动作。
// OUTPUT: 自动、明确支持、明确不支持三态覆盖；自动不会写入 false。
// POS: Provider 模型能力设置的最小行组件。
import { cn } from "@/shared/ui/class-name";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function CapabilitySelect({
  checked,
  disabled = false,
  label,
  onChange,
}: {
  checked: boolean | undefined;
  disabled?: boolean;
  label: string;
  onChange: (checked: boolean | undefined) => void;
}) {
  const { t } = useI18n();
  return (
    <label className="flex min-h-11 items-center justify-between gap-3 px-1 py-2">
      <span className={cn("min-w-0 [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "control", tone: "strong" }))}>{label}</span>
      <UiSelectMenu
        ariaLabel={label}
        value={checked === undefined ? "auto" : String(checked)}
        disabled={disabled}
        surface="dialog"
        size="sm"
        options={[
          { value: "auto", label: t("settings.providers.capability_auto") },
          { value: "true", label: t("settings.providers.capability_yes") },
          { value: "false", label: t("settings.providers.capability_no") },
        ]}
        onChange={(value) => onChange(value === "auto" ? undefined : value === "true")}
      />
    </label>
  );
}

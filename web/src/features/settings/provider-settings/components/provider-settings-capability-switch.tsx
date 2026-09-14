// INPUT: 单项模型能力、当前开关值、禁用状态与更新动作。
// OUTPUT: 可换行名称与原生标签关联的开关，文字点击复用同一开关动作。
// POS: Provider 模型能力设置的最小行组件。
import { cn } from "@/shared/ui/class-name";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function CapabilitySwitch({
  checked,
  disabled = false,
  label,
  onChange,
}: {
  checked: boolean;
  disabled?: boolean;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex min-h-11 items-center justify-between gap-3 px-1 py-2">
      <span className={cn("min-w-0 [overflow-wrap:anywhere]", getUiTypographyClassName({ role: "control", tone: "strong" }))}>{label}</span>
      <GlassSwitch
        aria-label={label}
        checked={checked}
        disabled={disabled}
        size="xs"
        onChange={onChange}
      />
    </label>
  );
}

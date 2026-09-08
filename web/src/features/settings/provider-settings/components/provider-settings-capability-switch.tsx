// INPUT: 单项模型能力、当前开关值与更新动作。
// OUTPUT: 与桌面设置行一致的纯文字开关，不为每项制造图标卡片。
// POS: Provider 模型能力设置的最小行组件。
import { cn } from "@/shared/ui/class-name";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function CapabilitySwitch({
  checked,
  label,
  onChange,
}: {
  checked: boolean;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex min-h-11 items-center justify-between gap-3 px-1 py-2">
      <span className={cn("truncate", getUiTypographyClassName({ role: "control", tone: "strong" }))}>{label}</span>
      <GlassSwitch
        aria-label={label}
        checked={checked}
        size="xs"
        onChange={onChange}
      />
    </div>
  );
}

import { type ReactNode } from "react";

import { GlassSwitch } from "@/shared/ui/liquid-glass";

export function CapabilitySwitch({
  checked,
  label,
  icon,
  on_change,
}: {
  checked: boolean;
  label: string;
  icon: ReactNode;
  on_change: (checked: boolean) => void;
}) {
  return (
    <div className="flex min-h-10 items-center justify-between gap-3 rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_78%,transparent)] px-3 py-2">
      <div className="flex min-w-0 items-center gap-2 text-[13px] font-medium text-(--text-strong)">
        <span className="text-(--icon-default)">{icon}</span>
        <span className="truncate">{label}</span>
      </div>
      <GlassSwitch checked={checked} size="xs" on_change={on_change} />
    </div>
  );
}

// INPUT: 行级动作、可访问名称与禁用状态。
// OUTPUT: 统一省略号入口和共享动作菜单。
// POS: 设置目录行的动作组合，不判断业务权限。
"use client";

import { MoreHorizontal } from "lucide-react";
import { useRef, useState } from "react";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiActionMenu, type UiActionMenuItem } from "@/shared/ui/menu/action-menu";

export function SettingsRowActions({ label, disabled, items, onSelect }: {
  label: string;
  disabled?: boolean;
  items: UiActionMenuItem[];
  onSelect: (value: string) => void;
}) {
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  return (
    <>
      <UiIconButton aria-label={label} aria-expanded={open && !disabled} aria-haspopup="menu" disabled={disabled} onClick={() => setOpen(!open)} ref={anchorRef} size="sm" variant="ghost">
        <MoreHorizontal className="h-4 w-4" />
      </UiIconButton>
      <UiActionMenu align="end" anchorRef={anchorRef} ariaLabel={label} isOpen={open && !disabled} items={items} onClose={() => setOpen(false)} onSelect={(value) => { setOpen(false); onSelect(value); }} />
    </>
  );
}

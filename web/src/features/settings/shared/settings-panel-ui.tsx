/**
 * INPUT: 设置项标题、说明、选项、当前值与设置目录动作。
 * OUTPUT: 设置卡片、具名开关行、可读标签/说明及共享 Button 目录导航。
 * POS: 设置域共享视图 Pattern；行级说明在窄屏仍用于解释选项影响。
 */

import { useId, type ButtonHTMLAttributes, type ReactNode } from "react";

import {
  UiButton,
  type UiButtonSize,
} from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export const SETTINGS_SECTION_TITLE_CLASS_NAME = cn(
  "px-1",
  getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
);
export const SETTINGS_CARD_CLASS_NAME = "overflow-hidden surface-radius-md border border-(--divider-subtle-color) bg-transparent";
export const SETTINGS_ROW_CLASS_NAME = "grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(180px,220px)] md:items-center";
export const SETTINGS_TEXT_ROW_CLASS_NAME = "flex min-w-0 items-start gap-3";
export const SETTINGS_ICON_CLASS_NAME = "flex h-7 w-7 shrink-0 items-center justify-center radius-control-sm bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary";
export const SETTINGS_ITEM_TITLE_CLASS_NAME = getUiTypographyClassName({
  role: "control",
  tone: "strong",
  weight: "semibold",
});
export const SETTINGS_ITEM_DESCRIPTION_CLASS_NAME = cn(
  "mt-1 max-w-[520px]",
  getUiTypographyClassName({ role: "supporting", tone: "muted" }),
);
export const SETTINGS_CONTROL_LABEL_CLASS_NAME = getUiTypographyClassName({
  role: "supporting",
  tone: "default",
  weight: "medium",
});

interface SettingsToggleRowProps {
  checked: boolean;
  description: string;
  disabled?: boolean;
  icon: ReactNode;
  onChange: (checked: boolean) => void;
  title: string;
}

export function SettingsToggleRow({
  checked, description, disabled, icon, onChange, title,
}: SettingsToggleRowProps) {
  const descriptionId = useId();
  return (
    <div className="flex min-w-0 items-center gap-3 px-4 py-3">
      <div className={cn(SETTINGS_TEXT_ROW_CLASS_NAME, "flex-1")}>
        <div aria-hidden="true" className={SETTINGS_ICON_CLASS_NAME}>{icon}</div>
        <div className="min-w-0 break-words">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>{title}</h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} id={descriptionId}>{description}</p>
        </div>
      </div>
      <GlassSwitch
        aria-describedby={descriptionId}
        aria-label={title}
        checked={checked}
        disabled={disabled}
        onChange={onChange}
        size="sm"
      />
    </div>
  );
}

interface SettingsNavigationButtonProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  active?: boolean;
  children: ReactNode;
  size?: UiButtonSize;
}

export function SettingsNavigationButton({
  "aria-current": ariaCurrent,
  active = false,
  children,
  className,
  size = "md",
  ...buttonProps
}: SettingsNavigationButtonProps) {
  return (
    <UiButton
      {...buttonProps}
      aria-current={ariaCurrent ?? (active ? "page" : undefined)}
      className={cn("w-full justify-start border-0 px-2 text-left", className)}
      size={size}
      variant="ghost"
    >
      {children}
    </UiButton>
  );
}

export function SettingsNavigationGroupLabel({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <p
      className={cn(
        "px-2 pb-1",
        getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }),
        className,
      )}
    >
      {children}
    </p>
  );
}

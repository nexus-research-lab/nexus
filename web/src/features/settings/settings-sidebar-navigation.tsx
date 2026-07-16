"use client";

import {
  ArrowLeft,
  Cable,
  Cpu,
  FolderKanban,
  Palette,
  Settings2,
  ShieldCheck,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";

import { canUseOperations } from "./operations/operations-access";
import {
  SETTINGS_NAVIGATION_GROUPS,
  type SettingsSectionKey,
} from "./settings-navigation-model";
import { useSettingsNavigation } from "./use-settings-navigation";

const SETTINGS_SECTION_ICONS: Record<SettingsSectionKey, LucideIcon> = {
  appearance: Palette,
  general: Settings2,
  runtime: Cpu,
  operations: ShieldCheck,
  permissions: ShieldCheck,
  personal: UserRound,
  providers: Cable,
  workspace: FolderKanban,
};

export function SettingsSidebarNavigation({
  variant,
}: {
  variant: "panel" | "rail";
}) {
  const { t } = useI18n();
  const { status } = useAuth();
  const { activeSection, backToWorkspace, selectSection } =
    useSettingsNavigation();
  const isRail = variant === "rail";
  const navigationGroups = SETTINGS_NAVIGATION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter(
      (item) =>
        item.key !== "operations" ||
        (!isDesktopRuntime() && canUseOperations(status?.role)),
    ),
  })).filter((group) => group.items.length > 0);

  if (isRail) {
    return (
      <nav
        aria-label={t("settings.title")}
        className="flex min-h-0 flex-1 flex-col items-center gap-1.5"
      >
        <SettingsRailButton
          icon={ArrowLeft}
          label={t("settings.back_to_workspace")}
          onClick={backToWorkspace}
        />
        <div className="my-1 h-px w-6 bg-(--divider-subtle-color)" />
        {navigationGroups.flatMap((group) => group.items).map(
          (item) => (
            <SettingsRailButton
              active={activeSection === item.key}
              icon={SETTINGS_SECTION_ICONS[item.key]}
              key={item.key}
              label={t(item.labelKey)}
              onClick={() => selectSection(item.key)}
            />
          ),
        )}
      </nav>
    );
  }

  return (
    <nav
      aria-label={t("settings.title")}
      className="soft-scrollbar flex min-h-0 flex-1 flex-col overflow-y-auto px-2 py-2.5"
    >
      <button
        className="mb-3 flex h-9 items-center gap-2 rounded-[8px] px-2 text-[12px] font-medium text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={backToWorkspace}
        type="button"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        <span>{t("settings.back_to_workspace")}</span>
      </button>

      <div className="flex flex-col gap-3">
        {navigationGroups.map((group) => (
          <section key={group.key}>
            <p className="px-2 pb-1 text-[12px] font-semibold uppercase tracking-[0.18em] text-(--text-soft)">
              {t(group.labelKey)}
            </p>
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const Icon = SETTINGS_SECTION_ICONS[item.key];
                const active = activeSection === item.key;
                return (
                  <button
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "flex h-9 w-full items-center gap-2.5 rounded-[8px] px-2 text-left text-[13px] font-medium transition-colors duration-(--motion-duration-fast)",
                      active
                        ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                        : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                    )}
                    key={item.key}
                    onClick={() => selectSection(item.key)}
                    type="button"
                  >
                    <Icon
                      className={cn(
                        "h-3.5 w-3.5 shrink-0",
                        active ? "text-(--primary)" : "text-(--icon-default)",
                      )}
                    />
                    <span className="truncate">{t(item.labelKey)}</span>
                  </button>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </nav>
  );
}

function SettingsRailButton({
  active = false,
  icon: Icon,
  label,
  onClick,
}: {
  active?: boolean;
  icon: LucideIcon;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      aria-current={active ? "page" : undefined}
      aria-label={label}
      className={cn(
        "flex h-9 w-9 items-center justify-center rounded-[9px] text-(--icon-default) transition-colors duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
        active &&
          "bg-(--surface-interactive-active-background) text-(--primary)",
      )}
      onClick={onClick}
      title={label}
      type="button"
    >
      <Icon className="h-4 w-4" />
    </button>
  );
}

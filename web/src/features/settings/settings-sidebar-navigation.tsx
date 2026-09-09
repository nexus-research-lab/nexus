// INPUT: 设置导航权限、查询与路由动作。
// OUTPUT: 返回与搜索同排、分组标题下选项统一缩进的宽侧栏，或紧凑导航轨。
// POS: 设置导航视图，复用公共输入与按钮，不执行设置写入。
"use client";

import { useState } from "react";

import {
  ArrowLeft,
  Cable,
  Chrome,
  Cpu,
  FolderKanban,
  Palette,
  Settings2,
  ShieldCheck,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { useAuth } from "@/shared/auth/auth-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";

import { canUseOperations } from "./operations/operations-access";
import {
  SETTINGS_NAVIGATION_GROUPS,
  type SettingsSectionKey,
} from "./settings-navigation-model";
import {
  SettingsNavigationButton,
  SettingsNavigationGroupLabel,
} from "./shared/settings-panel-ui";
import { SETTINGS_SEARCH_ITEMS } from "./settings-search-items";
import { useSettingsNavigation } from "./use-settings-navigation";

const SETTINGS_SECTION_ICONS: Record<SettingsSectionKey, LucideIcon> = {
  appearance: Palette,
  general: Settings2,
  runtime: Cpu,
  operations: ShieldCheck,
  permissions: ShieldCheck,
	browser: Chrome,
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
  const [query, setQuery] = useState("");
  const matcher = createUiSearchMatcher(isRail ? "" : query);
  const searchItems = (section: SettingsSectionKey) => SETTINGS_SEARCH_ITEMS[section].filter(
    (fields) => (isDesktopRuntime() || (fields[0] !== "settings.providers.ccswitch_title" && !fields[0].startsWith("settings.desktop.")))
      && !matcher.empty && matcher.matches(fields.map((key) => t(key))),
  );
  const navigationGroups = SETTINGS_NAVIGATION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter(
      (item) =>
        (matcher.matches([t(item.labelKey), t(group.labelKey)]) || searchItems(item.key).length > 0) &&
        (item.key !== "workspace" || isDesktopRuntime()) &&
				(item.key !== "browser" || isDesktopRuntime()) &&
        (item.key !== "operations" ||
          (!isDesktopRuntime() && canUseOperations(status?.role))),
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
      <div className="mb-4 flex min-w-0 shrink-0 items-center gap-2">
        <UiIconButton
          aria-label={t("settings.back_to_workspace")}
          tooltip={t("settings.back_to_workspace")}
          onClick={backToWorkspace}
          size="md"
        >
          <ArrowLeft aria-hidden="true" className="h-4 w-4" />
        </UiIconButton>
        <UiSearchInput
          aria-label={t("settings.search_navigation")}
          placeholder={t("settings.search_navigation")}
          className={cn(
            "min-w-0 flex-1 transition-[max-width] duration-200 motion-reduce:transition-none focus-within:max-w-full",
            query ? "max-w-full" : "max-w-[180px]",
          )}
          value={query}
          onChange={setQuery}
        />
      </div>
      {navigationGroups.length === 0 ? (
        <p role="status" className="ui-type-metadata px-2 text-(--text-muted)">
          {t("settings.search_no_results")}
        </p>
      ) : null}
      <div className="flex flex-col gap-3">
        {navigationGroups.map((group) => (
          <section key={group.key}>
            <SettingsNavigationGroupLabel>
              {t(group.labelKey)}
            </SettingsNavigationGroupLabel>
            <div className="ml-3 space-y-0.5">
              {group.items.map((item) => {
                const Icon = SETTINGS_SECTION_ICONS[item.key];
                const active = activeSection === item.key;
                return (
                  <div key={item.key}>
                  <SettingsNavigationButton
                    active={active}
                    onClick={() => selectSection(item.key)}
                  >
                    <Icon
                      className="h-3.5 w-3.5 shrink-0"
                    />
                    <span className="truncate">{t(item.labelKey)}</span>
                  </SettingsNavigationButton>
                  {searchItems(item.key).map((fields) => (
                    <SettingsNavigationButton
                      key={fields[0]}
                      className="pl-7 font-normal"
                      onClick={() => selectSection(item.key, fields[0])}
                    >
                      <span className="text-left whitespace-normal">{t(fields[0])}</span>
                    </SettingsNavigationButton>
                  ))}
                  </div>
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
    <UiIconButton
      aria-current={active ? "page" : undefined}
      aria-label={label}
      onClick={onClick}
      size="md"
      title={label}
      variant="ghost"
    >
      <Icon className="h-[18px] w-[18px]" />
    </UiIconButton>
  );
}

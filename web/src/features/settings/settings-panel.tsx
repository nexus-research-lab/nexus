// INPUT: 当前设置路由、宿主形态与账号访问范围。
// OUTPUT: 唯一设置侧栏/内容壳层及不重复页头的 Provider 管理内容。
// POS: 设置入口装配；各配置领域继续持有读取、草稿与写事务。

"use client";

import { useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Menu } from "lucide-react";
import { usePageHeaderActionsTarget } from "@/shared/lib/react/page-header-actions-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiDialogPortal, UiDialogBackdrop, UiDialogShell, UiDialogHeader } from "@/shared/ui/dialog/dialog";
import { Navigate } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { useProjectPermissionsEnabled } from "@/hooks/settings/use-project-permissions-enabled";
import { useAuth } from "@/shared/auth/auth-context";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { PersonalSettingsPanel } from "./personal/personal-settings-panel";
import { canUseOperations } from "./operations/operations-access";
import { OperationsPanel } from "./operations/operations-panel";
import { ProviderSettingsPanel } from "./provider-settings/provider-settings-panel";
import { SettingsGeneralSection } from "./general/settings-general-section";
import { SettingsRuntimeSection } from "./runtime/settings-runtime-section";
import { BrowserSettingsSection } from "./browser/browser-settings-section";
import { isOperationsSection, type SettingsSectionKey } from "./settings-navigation-model";
import { SettingsSidebarNavigation } from "./settings-sidebar-navigation";
import { useSettingsSearchTarget } from "./use-settings-search-target";
import { useSettingsNavigation } from "./use-settings-navigation";

export function SettingsPanel({ standalone = false }: { standalone?: boolean }) {
  const { status } = useAuth();
  const projectPermissionsEnabled = useProjectPermissionsEnabled();
  const { t } = useI18n();
  const headerActionsTarget = usePageHeaderActionsTarget();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const { activeSection } = useSettingsNavigation();
  const contentRef = useRef<HTMLDivElement>(null);
  useSettingsSearchTarget(contentRef, activeSection);
  const canViewOperations =
    !isDesktopRuntime() && canUseOperations(status?.role);
  const content = (
    <div ref={contentRef}>
      <SettingsSectionContent
        canViewOperations={canViewOperations && (activeSection !== "operations-projects" || projectPermissionsEnabled)}
        section={activeSection}
      />
    </div>
  );

  if (standalone) {
    return (
      <WorkspaceSurfaceScaffold bodyClassName="flex">
        <aside
          className="desktop-rail hidden h-full w-[224px] shrink-0 flex-col sm:flex"
          data-settings-navigation="panel"
        >
          <SettingsSidebarNavigation variant="panel" />
        </aside>
        <aside
          className="desktop-rail flex h-full w-14 shrink-0 flex-col px-1 py-2.5 sm:hidden"
          data-settings-navigation="rail"
        >
          <SettingsSidebarNavigation variant="rail" />
        </aside>
        <div className="soft-scrollbar scrollbar-stable-gutter min-h-0 min-w-0 flex-1 overflow-y-auto">
          {content}
        </div>
      </WorkspaceSurfaceScaffold>
    );
  }

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      stableGutter
    >
      {headerActionsTarget ? createPortal(
        <UiIconButton aria-label={t("settings.open_navigation")} aria-expanded={navigationOpen} aria-haspopup="dialog" onClick={() => setNavigationOpen(true)} size="md" variant="ghost"><Menu className="h-4 w-4" /></UiIconButton>,
        headerActionsTarget,
      ) : null}
      {navigationOpen && headerActionsTarget ? (
        <UiDialogPortal>
          <UiDialogBackdrop className="items-stretch justify-start" onClose={() => setNavigationOpen(false)}>
            <UiDialogShell className="h-full w-72 max-w-full" size="xs">
              <UiDialogHeader title={t("settings.title")} onClose={() => setNavigationOpen(false)} />
              <SettingsSidebarNavigation variant="panel" onNavigate={() => setNavigationOpen(false)} />
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}
      {content}
    </WorkspaceSurfaceScaffold>
  );
}

function SettingsSectionContent({
  canViewOperations,
  section,
}: {
  canViewOperations: boolean;
  section: SettingsSectionKey;
}) {
  if (isOperationsSection(section)) {
    return canViewOperations ? (
      <OperationsPanel section={section} />
    ) : (
      <Navigate replace to={AppRouteBuilders.settings()} />
    );
  }
  if (section === "personal") {
    return <PersonalSettingsPanel />;
  }
  if (section === "providers") {
    return <ProviderSettingsPanel />;
  }
  if (section === "runtime") {
    return <SettingsRuntimeSection />;
  }
	if (section === "browser") {
		return isDesktopRuntime() ? (
			<BrowserSettingsSection />
		) : (
			<Navigate replace to={AppRouteBuilders.settings()} />
		);
	}
  if (section === "workspace" && !isDesktopRuntime()) {
    return <Navigate replace to={AppRouteBuilders.settings()} />;
  }
  return <SettingsGeneralSection section={section} />;
}

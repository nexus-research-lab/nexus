"use client";

import { Navigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { useAuth } from "@/shared/auth/auth-context";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { PersonalSettingsPanel } from "./personal/personal-settings-panel";
import { canUseOperations } from "./operations/operations-access";
import { OperationsPanel } from "./operations/operations-panel";
import { ProviderSettingsPanel } from "./provider-settings/provider-settings-panel";
import { SettingsGeneralSection } from "./general/settings-general-section";
import { SettingsRuntimeSection } from "./runtime/settings-runtime-section";
import type { SettingsSectionKey } from "./settings-navigation-model";
import { SettingsSidebarNavigation } from "./settings-sidebar-navigation";
import { useSettingsNavigation } from "./use-settings-navigation";

export function SettingsPanel({ standalone = false }: { standalone?: boolean }) {
  const { status } = useAuth();
  const { activeSection } = useSettingsNavigation();
  const canViewOperations =
    !isDesktopRuntime() && canUseOperations(status?.role);
  const content = (
    <SettingsSectionContent
      canViewOperations={canViewOperations}
      section={activeSection}
    />
  );

  if (standalone) {
    return (
      <WorkspaceSurfaceScaffold bodyClassName="flex">
        <aside className="desktop-rail flex h-full w-[224px] shrink-0 flex-col border-r divider-subtle">
          <SettingsSidebarNavigation variant="panel" />
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
  if (section === "operations") {
    return canViewOperations ? (
      <OperationsPanel embedded />
    ) : (
      <Navigate replace to={AppRouteBuilders.settings()} />
    );
  }
  if (section === "personal") {
    return <PersonalSettingsPanel />;
  }
  if (section === "providers") {
    return <ProviderSettingsPanel embedded />;
  }
  if (section === "runtime") {
    return <SettingsRuntimeSection />;
  }
  return <SettingsGeneralSection section={section} />;
}

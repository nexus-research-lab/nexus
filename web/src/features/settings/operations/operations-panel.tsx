/**
 * INPUT: 当前运营标签、访问范围与返回动作。
 * OUTPUT: 运营设置页签和对应管理内容，移动端避免重复页面标题。
 * POS: 设置内嵌与独立运营入口共用的页面装配层。
 */
"use client";

import { ArrowLeft } from "lucide-react";
import { useCallback, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { ProviderSettingsPanel } from "@/features/settings/provider-settings/provider-settings-panel";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { ProjectAdminPanel } from "./project-admin/project-admin-panel";
import { SubscriptionAdminPanel } from "./subscription-admin/subscription-admin-panel";
import { ControlMembersPanel } from "./control-members-panel";

const OPERATIONS_TAB_KEYS = [
  "members",
  "userSubscriptions",
  "subscriptionPlans",
  "subscriptionProviders",
  "projects",
] as const;

type OperationsTabKey = (typeof OPERATIONS_TAB_KEYS)[number];
type OperationsTabLabelKey =
  | "operations.tabs.members"
  | "operations.tabs.user_subscriptions"
  | "operations.tabs.subscription_plans"
  | "operations.tabs.subscription_providers"
  | "operations.tabs.projects";

interface OperationsTabDefinition {
  labelKey: OperationsTabLabelKey;
  renderContent: () => ReactNode;
}

const OPERATIONS_TAB_DEFINITIONS: Record<
  OperationsTabKey,
  OperationsTabDefinition
> = {
  members: {
    labelKey: "operations.tabs.members",
    renderContent: () => <ControlMembersPanel />,
  },
  userSubscriptions: {
    labelKey: "operations.tabs.user_subscriptions",
    renderContent: () => <SubscriptionAdminPanel view="users" />,
  },
  subscriptionPlans: {
    labelKey: "operations.tabs.subscription_plans",
    renderContent: () => <SubscriptionAdminPanel view="plans" />,
  },
  subscriptionProviders: {
    labelKey: "operations.tabs.subscription_providers",
    renderContent: () => (
      <ProviderSettingsPanel
        embedded
        layout="section"
        visibilityScope="public"
      />
    ),
  },
  projects: {
    labelKey: "operations.tabs.projects",
    renderContent: () => <ProjectAdminPanel />,
  },
};

export function OperationsPanel({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<OperationsTabKey>("members");
  const activeTabConfig = OPERATIONS_TAB_DEFINITIONS[activeTab];

  const handleBackToWorkspace = useCallback(() => {
    navigate(APP_ROUTE_PATHS.home);
  }, [navigate]);

  const tabs = OPERATIONS_TAB_KEYS.map((key) => ({
    value: key,
    label: t(OPERATIONS_TAB_DEFINITIONS[key].labelKey),
  }));
  const content = activeTabConfig.renderContent();
  const page = (
    <div className={cn(
      WORKSPACE_CONTENT_PAGE_CLASS_NAME,
      "flex min-h-full flex-col",
    )}>
      <WorkspaceContentHeader
        actions={!embedded ? (
          <UiButton
            onClick={handleBackToWorkspace}
            size="2xs"
            variant="text"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            {t("settings.back_to_workspace")}
          </UiButton>
        ) : undefined}
        className="max-sm:hidden"
        description={t("operations.description")}
        title={t("operations.page_title")}
      />
      <UiTabs
        activeValue={activeTab}
        ariaLabel={t("operations.title")}
        className="shrink-0"
        density="compact"
        itemClassName="px-3"
        onChange={setActiveTab}
        options={tabs}
      />
      <div className="min-h-0 flex-1 pt-4">{content}</div>
    </div>
  );

  if (embedded) {
    return page;
  }

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      stableGutter
    >
      {page}
    </WorkspaceSurfaceScaffold>
  );
}

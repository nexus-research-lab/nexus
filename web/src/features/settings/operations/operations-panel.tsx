"use client";

import {
  ArrowLeft,
  Cable,
  ShieldCheck,
  UsersRound,
  type LucideIcon,
} from "lucide-react";
import { useCallback, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { ProviderSettingsPanel } from "@/features/settings/provider-settings/provider-settings-panel";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { SubscriptionAdminPanel } from "./subscription-admin/subscription-admin-panel";

const OPERATIONS_TAB_KEYS = [
  "userSubscriptions",
  "subscriptionPlans",
  "subscriptionProviders",
] as const;

type OperationsTabKey = (typeof OPERATIONS_TAB_KEYS)[number];
type OperationsTabLabelKey =
  | "operations.tabs.user_subscriptions"
  | "operations.tabs.subscription_plans"
  | "operations.tabs.subscription_providers";

interface OperationsTabDefinition {
  icon: LucideIcon;
  labelKey: OperationsTabLabelKey;
  renderContent: () => ReactNode;
}

const OPERATIONS_TAB_DEFINITIONS: Record<
  OperationsTabKey,
  OperationsTabDefinition
> = {
  userSubscriptions: {
    labelKey: "operations.tabs.user_subscriptions",
    icon: UsersRound,
    renderContent: () => <SubscriptionAdminPanel view="users" />,
  },
  subscriptionPlans: {
    labelKey: "operations.tabs.subscription_plans",
    icon: ShieldCheck,
    renderContent: () => <SubscriptionAdminPanel view="plans" />,
  },
  subscriptionProviders: {
    labelKey: "operations.tabs.subscription_providers",
    icon: Cable,
    renderContent: () => (
      <ProviderSettingsPanel embedded visibilityScope="public" />
    ),
  },
};

export function OperationsPanel({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<OperationsTabKey>("userSubscriptions");
  const activeTabConfig = OPERATIONS_TAB_DEFINITIONS[activeTab];
  const ActiveIcon = activeTabConfig.icon;

  const handleBackToWorkspace = useCallback(() => {
    navigate(APP_ROUTE_PATHS.home);
  }, [navigate]);

  const tabs = OPERATIONS_TAB_KEYS.map((key) => ({
    key,
    label: t(OPERATIONS_TAB_DEFINITIONS[key].labelKey),
    icon: OPERATIONS_TAB_DEFINITIONS[key].icon,
  }));
  const content = activeTabConfig.renderContent();

  if (embedded) {
    return (
      <>
        <WorkspaceSurfaceHeader
          activeTab={activeTab}
          onChangeTab={setActiveTab}
          tabs={tabs}
        />
        {content}
      </>
    );
  }

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      stableGutter
      header={(
        <WorkspaceSurfaceHeader
          activeTab={activeTab}
          leading={<ActiveIcon className="h-4 w-4" />}
          onChangeTab={setActiveTab}
          tabs={tabs}
          title={t("operations.title")}
          trailing={(
            <WorkspaceSurfaceToolbarAction onClick={handleBackToWorkspace}>
              <ArrowLeft className="h-3.5 w-3.5" />
              {t("settings.back_to_workspace")}
            </WorkspaceSurfaceToolbarAction>
          )}
        />
      )}
    >
      {content}
    </WorkspaceSurfaceScaffold>
  );
}

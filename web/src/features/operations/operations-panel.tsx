"use client";

import { ArrowLeft, Cable, ShieldCheck, UsersRound } from "lucide-react";
import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { ProviderSettingsPanel } from "@/features/settings/provider-settings/provider-settings-panel";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { SubscriptionAdminPanel } from "./subscription-admin/subscription-admin-panel";

type OperationsTabKey =
  | "userSubscriptions"
  | "subscriptionPlans"
  | "subscriptionProviders";

const OPERATIONS_TABS: {
  key: OperationsTabKey;
  labelKey:
    | "operations.tabs.user_subscriptions"
    | "operations.tabs.subscription_plans"
    | "operations.tabs.subscription_providers";
  icon: typeof ShieldCheck;
}[] = [
  {
    key: "userSubscriptions",
    labelKey: "operations.tabs.user_subscriptions",
    icon: UsersRound,
  },
  {
    key: "subscriptionPlans",
    labelKey: "operations.tabs.subscription_plans",
    icon: ShieldCheck,
  },
  {
    key: "subscriptionProviders",
    labelKey: "operations.tabs.subscription_providers",
    icon: Cable,
  },
];

export function OperationsPanel({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<OperationsTabKey>("userSubscriptions");
  const activeTabConfig =
    OPERATIONS_TABS.find((item) => item.key === activeTab) ?? OPERATIONS_TABS[0];
  const ActiveIcon = activeTabConfig.icon;

  const handleBackToWorkspace = useCallback(() => {
    navigate(APP_ROUTE_PATHS.home);
  }, [navigate]);

  const tabs = OPERATIONS_TABS.map((item) => ({
    key: item.key,
    label: t(item.labelKey),
    icon: item.icon,
  }));
  const content = (
    <>
      {activeTab === "userSubscriptions" ? (
        <SubscriptionAdminPanel view="users" />
      ) : null}
      {activeTab === "subscriptionPlans" ? (
        <SubscriptionAdminPanel view="plans" />
      ) : null}
      {activeTab === "subscriptionProviders" ? (
        <ProviderSettingsPanel embedded visibilityScope="public" />
      ) : null}
    </>
  );

  if (embedded) {
    return (
      <>
        <WorkspaceSurfaceHeader
          activeTab={activeTab}
          density="compact"
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
          density="compact"
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

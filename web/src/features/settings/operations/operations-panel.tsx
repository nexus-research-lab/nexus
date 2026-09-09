// INPUT: 已校验权限的运营子页 URL 分区。
// OUTPUT: 当前管理页面；导航由设置侧栏统一持有。
// POS: 运营内容装配，不持有本地页签状态或额外导航壳层。
"use client";

import type { ReactNode } from "react";
import { ProviderSettingsPanel } from "@/features/settings/provider-settings/provider-settings-panel";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { getSettingsSectionLabelKey, type OperationsSectionKey } from "../settings-navigation-model";
import { ProjectAdminPanel } from "./project-admin/project-admin-panel";
import { SubscriptionAdminPanel } from "./subscription-admin/subscription-admin-panel";
import { ControlMembersPanel } from "./control-members-panel";

const CONTENT: Record<OperationsSectionKey, () => ReactNode> = {
  "operations-members": () => <ControlMembersPanel />,
  "operations-subscriptions": () => <SubscriptionAdminPanel view="users" />,
  "operations-plans": () => <SubscriptionAdminPanel view="plans" />,
  "operations-providers": () => <ProviderSettingsPanel layout="section" visibilityScope="public" />,
  "operations-projects": () => <ProjectAdminPanel />,
};

export function OperationsPanel({ section }: { section: OperationsSectionKey }) {
  const { t } = useI18n();
  // 成员和项目页的标题与刷新动作由各自事务视图组合。
  const ownsHeader = section === "operations-members" || section === "operations-projects";
  return (
    <div className={WORKSPACE_CONTENT_PAGE_CLASS_NAME} data-operations-page={section}>
      {!ownsHeader ? (
        <WorkspaceContentHeader
          className="max-sm:hidden"
          title={t(getSettingsSectionLabelKey(section))}
        />
      ) : null}
      {CONTENT[section]()}
    </div>
  );
}

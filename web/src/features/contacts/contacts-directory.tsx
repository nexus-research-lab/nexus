/**
 * INPUT: Agent 目录与创建、详情、私聊、群聊导航命令。
 * OUTPUT: 直接复用公共搜索输入、共享具名筛选、可恢复空态和卡片/列表 Agent 目录。
 * POS: 联系人正文根目录；承载选择 Agent 所需的识别和能力概况。
 */
"use client";

import { LayoutGrid, List, Plus } from "lucide-react";
import { useMemo, useState } from "react";

import { AGENT_PERMISSION_MODES } from "@/lib/agent-options";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiFilterSelect } from "@/shared/ui/menu/filter-select";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import {
  WORKSPACE_CATALOG_GRID_CLASS_NAME,
  WORKSPACE_CONTENT_PAGE_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceCatalogGhostAction } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import {
  WorkspaceCatalogDescription,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-content";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { Agent } from "@/types/agent/agent";
import { formatProviderLabel } from "@/types/capability/provider";

import { ContactsAgentCard } from "./contacts-agent-card";
import {
  CONTACTS_DEFAULT_PROVIDER_FILTER,
  filterContactsAgents,
  getContactsDirectoryBusinessTags,
  getContactsDirectoryPermissionModes,
  getContactsDirectoryProviders,
} from "./contacts-directory-helpers";

type ContactsDirectoryView = "grid" | "list";

interface ContactsDirectoryProps {
  agents: Agent[];
  onOpenDirectRoom: (agentId: string) => void;
  onCreateAgent: () => void;
  onOpenAgent: (agentId: string) => void;
  onCreateTeam: (agentId: string) => void;
}

export function ContactsDirectory({
  agents,
  onOpenDirectRoom,
  onCreateAgent,
  onOpenAgent,
  onCreateTeam,
}: ContactsDirectoryProps) {
  const { t } = useI18n();
  const [searchQuery, setSearchQuery] = useState("");
  const [tagFilter, setTagFilter] = useState("");
  const [providerFilter, setProviderFilter] = useState("");
  const [permissionFilter, setPermissionFilter] = useState("");
  const [view, setView] = useState<ContactsDirectoryView>("grid");

  const filteredAgents = useMemo(
    () => filterContactsAgents(agents, {
      permissionMode: permissionFilter,
      provider: providerFilter,
      query: searchQuery,
      tag: tagFilter,
    }),
    [agents, permissionFilter, providerFilter, searchQuery, tagFilter],
  );
  const businessTags = useMemo(
    () => getContactsDirectoryBusinessTags(agents),
    [agents],
  );
  const providers = useMemo(() => getContactsDirectoryProviders(agents), [agents]);
  const permissionModes = useMemo(
    () => getContactsDirectoryPermissionModes(agents),
    [agents],
  );

  const tagOptions = [
    { label: t("contacts.filters.all_tags"), value: "" },
    ...businessTags.map((tag) => ({ label: tag, value: tag })),
  ];
  const providerOptions = [
    { label: t("contacts.filters.all_providers"), value: "" },
    ...providers.map((provider) => ({
      label: provider === CONTACTS_DEFAULT_PROVIDER_FILTER
        ? t("agent_options.identity.follow_default_provider")
        : formatProviderLabel(provider),
      value: provider,
    })),
  ];
  const permissionOptions = [
    { label: t("contacts.filters.all_permissions"), value: "" },
    ...permissionModes.map((permissionMode) => {
      const option = AGENT_PERMISSION_MODES.find(
        (candidate) => candidate.value === permissionMode,
      );
      return {
        label: option ? t(option.labelKey) : permissionMode,
        value: permissionMode,
      };
    }),
  ];

  const headerTrailing = (
    <UiSearchInput
      className="w-full sm:w-[240px]"
      onChange={setSearchQuery}
      placeholder={t("common.search_agents")}
      value={searchQuery}
    />
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="soft-scrollbar scrollbar-stable-gutter min-h-0 flex-1 overflow-y-auto">
        <div className={WORKSPACE_CONTENT_PAGE_CLASS_NAME}>
          <WorkspaceContentHeader
            actions={headerTrailing}
            description={t("contacts.description")}
            title={t("contacts.title")}
          />
          <div className="mb-3 space-y-2.5">
            <div className="flex flex-wrap items-center gap-2">
              <span className={cn(
                "mr-auto shrink-0",
                getUiTypographyClassName({ role: "metadata", tone: "muted" }),
              )}>
                {t("contacts.result_count", {
                  count: filteredAgents.length,
                  total: agents.length,
                })}
              </span>
              <UiFilterSelect
                ariaLabel={t("contacts.filters.tags")}
                className="w-full sm:w-[232px]"
                disabled={businessTags.length === 0}
                menuMinWidth={220}
                onChange={setTagFilter}
                options={tagOptions}
                value={tagFilter}
              />
              <UiFilterSelect
                ariaLabel={t("contacts.filters.providers")}
                className="w-full sm:w-[224px]"
                menuMinWidth={190}
                onChange={setProviderFilter}
                options={providerOptions}
                value={providerFilter}
              />
              <UiFilterSelect
                ariaLabel={t("contacts.filters.permissions")}
                className="w-full sm:w-[232px]"
                menuMinWidth={180}
                onChange={setPermissionFilter}
                options={permissionOptions}
                value={permissionFilter}
              />
              <UiSegmentedControl
                className="shrink-0"
                density="compact"
                onChange={setView}
                options={[
                  {
                    icon: LayoutGrid,
                    iconOnly: true,
                    label: t("contacts.views.grid"),
                    value: "grid",
                  },
                  {
                    icon: List,
                    iconOnly: true,
                    label: t("contacts.views.list"),
                    value: "list",
                  },
                ]}
                title={t("contacts.views.title")}
                value={view}
              />
            </div>
          </div>
          <UiPanel
            className={view === "grid"
              ? cn(WORKSPACE_CATALOG_GRID_CLASS_NAME, "gap-3.5 md:gap-4")
              : "divide-y divide-(--divider-subtle-color) overflow-hidden"}
            padding="none"
            radius="md"
            variant={view === "grid" ? "plain" : "card"}
          >
            {view === "grid" ? (
              <WorkspaceCatalogGhostAction aria-label={t("contacts.new_agent")} className="gap-3"
                onClick={onCreateAgent} size="comfort">
                <WorkspaceIconFrame shape="round" size="lg"><Plus aria-hidden className="h-6 w-6" /></WorkspaceIconFrame>
                <span className="min-w-0">
                  <WorkspaceCatalogTitle as="span" className="block [overflow-wrap:anywhere]" size="lg">
                    {t("contacts.new_agent")}
                  </WorkspaceCatalogTitle>
                  <WorkspaceCatalogDescription className="mt-1" lines={2}>
                    {t("contacts.new_agent_description")}
                  </WorkspaceCatalogDescription>
                </span>
              </WorkspaceCatalogGhostAction>
            ) : (
              <UiListRow aria-label={t("contacts.new_agent")} variant="flush" onClick={onCreateAgent}
                leading={<WorkspaceIconFrame shape="round" size="md"><Plus aria-hidden className="h-5 w-5" /></WorkspaceIconFrame>}>
                <div className="min-w-0">
                  <WorkspaceCatalogTitle className="[overflow-wrap:anywhere]" size="sm">{t("contacts.new_agent")}</WorkspaceCatalogTitle>
                  <WorkspaceCatalogDescription className="mt-1" lines={2}>{t("contacts.new_agent_description")}</WorkspaceCatalogDescription>
                </div>
              </UiListRow>
            )}
            {filteredAgents.map((agent) => (
              <ContactsAgentCard
                key={agent.agent_id}
                agent={agent}
                onCreateTeam={() => onCreateTeam(agent.agent_id)}
                onOpenProfile={() => onOpenAgent(agent.agent_id)}
                onOpenRoom={() => onOpenDirectRoom(agent.agent_id)}
                view={view}
              />
            ))}
            {agents.length > 0 && filteredAgents.length === 0 ? (
              <UiResourceState className="col-span-full" size="sm" state="empty" variant="plain"
                title={t("contacts.no_matches")} primaryAction={{ label: t("state.clear_filters"), onClick: () => {
                  setSearchQuery("");
                  setTagFilter("");
                  setProviderFilter("");
                  setPermissionFilter("");
                } }} />
            ) : null}
          </UiPanel>
        </div>
      </div>
    </div>
  );
}

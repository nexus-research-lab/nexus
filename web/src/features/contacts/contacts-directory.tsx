/**
 * INPUT: Agent 目录与创建、详情、私聊、群聊导航命令。
 * OUTPUT: 可搜索筛选并切换卡片/列表的 Agent 管理目录。
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
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
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
import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";
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
    <WorkspaceSearchInput
      className="hidden sm:inline-flex"
      inputClassName="w-[200px]"
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
            <WorkspaceSearchInput
              className="sm:hidden"
              onChange={setSearchQuery}
              placeholder={t("common.search_agents")}
              value={searchQuery}
            />
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
              <UiSelectMenu
                ariaLabel={t("contacts.filters.tags")}
                buttonClassName="gap-1 px-2.5 shadow-none"
                className="w-[140px] shrink-0"
                disabled={businessTags.length === 0}
                menuMinWidth={220}
                onChange={setTagFilter}
                options={tagOptions}
                placement="bottom"
                size="sm"
                value={tagFilter}
              />
              <UiSelectMenu
                ariaLabel={t("contacts.filters.providers")}
                buttonClassName="gap-1 px-2.5 shadow-none"
                className="w-[150px] shrink-0"
                menuMinWidth={190}
                onChange={setProviderFilter}
                options={providerOptions}
                placement="bottom"
                size="sm"
                value={providerFilter}
              />
              <UiSelectMenu
                ariaLabel={t("contacts.filters.permissions")}
                buttonClassName="gap-1 px-2.5 shadow-none"
                className="w-[140px] shrink-0"
                menuMinWidth={180}
                onChange={setPermissionFilter}
                options={permissionOptions}
                placement="bottom"
                size="sm"
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
              <>
                <WorkspaceCatalogGhostAction
                  className="flex-row justify-start gap-3 text-left md:hidden"
                  onClick={onCreateAgent}
                  size="compact"
                >
                  <WorkspaceIconFrame className="h-10 w-10 shrink-0" shape="round" size="md">
                    <Plus className="h-4.5 w-4.5 text-(--icon-default)" />
                  </WorkspaceIconFrame>
                  <span className="min-w-0">
                    <WorkspaceCatalogTitle as="span" className="block" size="sm" truncate>
                      {t("contacts.new_agent")}
                    </WorkspaceCatalogTitle>
                    <WorkspaceCatalogDescription className="mt-1" lines={2}>
                      {t("contacts.new_agent_description")}
                    </WorkspaceCatalogDescription>
                  </span>
                </WorkspaceCatalogGhostAction>
                <WorkspaceCatalogGhostAction
                  className="hidden py-8 md:flex"
                  onClick={onCreateAgent}
                  size="comfort"
                >
                  <WorkspaceIconFrame className="h-16 w-16" shape="round" size="lg">
                    <Plus className="h-7 w-7 text-(--icon-default)" />
                  </WorkspaceIconFrame>
                  <WorkspaceCatalogTitle as="p" className="mt-4" size="lg">
                    {t("contacts.new_agent")}
                  </WorkspaceCatalogTitle>
                  <WorkspaceCatalogDescription className="mt-2" minHeight={false}>
                    {t("contacts.new_agent_description")}
                  </WorkspaceCatalogDescription>
                </WorkspaceCatalogGhostAction>
              </>
            ) : (
              <UiListRow
                className="min-h-[76px] rounded-none px-3 py-2.5"
                description={t("contacts.new_agent_description")}
                leading={(
                  <WorkspaceIconFrame className="h-10 w-10 shrink-0" shape="round" size="md">
                    <Plus className="h-4.5 w-4.5 text-(--icon-default)" />
                  </WorkspaceIconFrame>
                )}
                onClick={onCreateAgent}
                title={t("contacts.new_agent")}
              />
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
              <p className={cn(
                "col-span-full px-4 py-10 text-center",
                getUiTypographyClassName({ role: "supporting", tone: "muted" }),
              )}>
                {t("contacts.no_matches")}
              </p>
            ) : null}
          </UiPanel>
        </div>
      </div>
    </div>
  );
}

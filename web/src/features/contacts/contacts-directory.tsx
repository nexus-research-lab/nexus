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
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import {
  WORKSPACE_CATALOG_GRID_CLASS_NAME,
  WORKSPACE_CONTENT_PAGE_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceCatalogGhostAction } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
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
              <span className="mr-auto shrink-0 text-compact text-(--text-muted)">
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
              <div className="flex shrink-0 items-center rounded-[8px] border border-(--surface-control-border) p-0.5">
                <UiIconButton
                  aria-label={t("contacts.views.grid")}
                  aria-pressed={view === "grid"}
                  onClick={() => setView("grid")}
                  size="sm"
                  title={t("contacts.views.grid")}
                  variant="ghost"
                >
                  <LayoutGrid className="h-3.5 w-3.5" />
                </UiIconButton>
                <UiIconButton
                  aria-label={t("contacts.views.list")}
                  aria-pressed={view === "list"}
                  onClick={() => setView("list")}
                  size="sm"
                  title={t("contacts.views.list")}
                  variant="ghost"
                >
                  <List className="h-3.5 w-3.5" />
                </UiIconButton>
              </div>
            </div>
          </div>
          <div className={view === "grid"
            ? cn(WORKSPACE_CATALOG_GRID_CLASS_NAME, "gap-3.5 md:gap-4")
            : "divide-y divide-(--divider-subtle-color) overflow-hidden rounded-[12px] border border-(--divider-subtle-color)"}
          >
            {view === "grid" ? (
              <>
                <button
                  className="flex min-h-[138px] w-full items-center gap-3 rounded-[12px] border border-dashed border-(--divider-subtle-color) bg-transparent px-4 py-4 text-left transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) md:hidden"
                  onClick={onCreateAgent}
                  type="button"
                >
                  <WorkspaceIconFrame className="h-10 w-10 shrink-0" shape="round" size="md">
                    <Plus className="h-4.5 w-4.5 text-(--icon-default)" />
                  </WorkspaceIconFrame>
                  <span className="min-w-0">
                    <span className="block truncate text-base font-semibold text-(--text-strong)">
                      {t("contacts.new_agent")}
                    </span>
                    <span className="mt-1 block line-clamp-2 text-xs leading-5 text-(--text-muted)">
                      {t("contacts.new_agent_description")}
                    </span>
                  </span>
                </button>
                <WorkspaceCatalogGhostAction
                  className="hidden py-8 md:flex"
                  onClick={onCreateAgent}
                  size="comfort"
                >
                  <WorkspaceIconFrame className="h-16 w-16" shape="round" size="lg">
                    <Plus className="h-7 w-7 text-(--icon-default)" />
                  </WorkspaceIconFrame>
                  <p className="mt-4 text-md font-semibold tracking-[-0.03em] text-(--text-strong)">
                    {t("contacts.new_agent")}
                  </p>
                  <p className="mt-2 text-sm leading-5 text-(--text-default)">
                    {t("contacts.new_agent_description")}
                  </p>
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
              <p className="col-span-full px-4 py-10 text-center text-sm text-(--text-muted)">
                {t("contacts.no_matches")}
              </p>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

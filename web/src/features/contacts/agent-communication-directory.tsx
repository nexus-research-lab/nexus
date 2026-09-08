// INPUT: 当前 Agent、联系人/候选目录、读取状态与增删选择命令。
// OUTPUT: 共享搜索与列表目录，以及保留当前选择、锁定提交的 Dialog/Form/Panel 添加流程。
// POS: Contacts 联络目录视图；不拥有聊天 Session、消息时间线或服务端 mutation 真相。
"use client";

import {
  Check,
  LoaderCircle,
  MessageCircle,
  UserRoundPlus,
  UsersRound,
} from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiField, UiInput, UiSearchInput } from "@/shared/ui/form/form-control";
import { SidebarSearchAction, SidebarSearchField } from "@/shared/ui/form/sidebar-search-field";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
import type { Agent, AgentContact } from "@/types/agent/agent";
import type { AgentCommunicationReadFailure } from "@/types/agent/communication";

import {
  filterCommunicationContacts,
  getCommunicationAgentName,
  getCommunicationContactLabel,
} from "./agent-communication-model";
import {
  AgentCommunicationEmptyState,
  AgentCommunicationReadFailureState,
} from "./agent-communication-status";

interface AgentCommunicationDirectoryProps {
  agent: Agent;
  agents: Agent[];
  contacts: AgentContact[];
  directoryFailure: AgentCommunicationReadFailure | null;
  isDirectoryLoading: boolean;
  pendingAgentId: string | null;
  selectedContactId: string | null;
  onAddContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  onRefresh: () => void;
  onSelectContact: (contactAgentId: string) => void;
}

export function AgentCommunicationDirectory({
  agent,
  agents,
  contacts: allContacts,
  directoryFailure,
  isDirectoryLoading,
  onAddContact,
  onRefresh,
  onSelectContact,
  pendingAgentId,
  selectedContactId,
}: AgentCommunicationDirectoryProps) {
  const { t } = useI18n();
  const [query, setQuery] = useResettableState("", agent.agent_id);
  const [addDialogOpen, setAddDialogOpen] = useResettableState(false, agent.agent_id);
  const hasQuery = !createUiSearchMatcher(query).empty;
  const contacts = useMemo(
    () => filterCommunicationContacts(allContacts, query),
    [allContacts, query],
  );
  const availableAgents = useMemo(() => {
    const contactIds = new Set(allContacts.map((contact) => contact.contact_agent_id));
    return agents.filter((candidate) => (
      candidate.agent_id !== agent.agent_id
      && !candidate.is_main
      && !contactIds.has(candidate.agent_id)
    ));
  }, [agent.agent_id, agents, allContacts]);

  return (
    <>
      <aside className={cn(
        "min-h-0 min-w-0 flex-col overflow-hidden bg-(--surface-shell-directory-background) md:flex",
        selectedContactId ? "hidden" : "flex",
      )}>
        <div className="shrink-0 pt-3">
          <SidebarSearchField
            action={(
              <SidebarSearchAction
                aria-label={t("agent_options.contact.add_friend")}
                onClick={() => setAddDialogOpen(true)}
                title={t("agent_options.contact.add_friend")}
              >
                <UserRoundPlus />
              </SidebarSearchAction>
            )}
            label={t("agent_options.contact.search_contacts")}
            onChange={setQuery}
            value={query}
          />
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-2">
          {isDirectoryLoading && allContacts.length === 0 && !directoryFailure ? (
            <AgentCommunicationEmptyState
              label={t("agent_options.contact.loading_address_book")}
              loading
            />
          ) : directoryFailure && !directoryFailure.stale ? (
            <AgentCommunicationReadFailureState
              failure={directoryFailure}
              onRetry={onRefresh}
            />
          ) : (
            <>
              {directoryFailure ? (
                <AgentCommunicationReadFailureState
                  compact
                  failure={directoryFailure}
                  onRetry={onRefresh}
                />
              ) : null}
              {contacts.length === 0 ? (
                <AgentCommunicationEmptyState
                  icon={hasQuery ? MessageCircle : UsersRound}
                  label={hasQuery
                    ? t("agent_options.contact.no_search_results")
                    : t("agent_options.contact.empty_directory")}
                  action={hasQuery ? {
                    label: t("common.clear"), onClick: () => setQuery(""),
                  } : {
                    label: t("agent_options.contact.add_friend"), onClick: () => setAddDialogOpen(true),
                  }}
                />
              ) : (
                <div className="space-y-0.5">
                  {contacts.map((contact) => (
                    <ContactRow
                      contact={contact}
                      isSelected={selectedContactId === contact.contact_agent_id}
                      key={contact.contact_agent_id}
                      onSelect={() => onSelectContact(contact.contact_agent_id)}
                    />
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </aside>

      {addDialogOpen ? (
        <AddContactDialog
          agentId={agent.agent_id}
          agents={availableAgents}
          isPending={Boolean(pendingAgentId)}
          key={agent.agent_id}
          onAdd={onAddContact}
          onClose={() => setAddDialogOpen(false)}
        />
      ) : null}
    </>
  );
}

function ContactRow({
  contact,
  isSelected,
  onSelect,
}: {
  contact: AgentContact;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const label = getCommunicationContactLabel(contact);
  return (
    <UiListRow
      active={isSelected}
      activeTone="sidebar"
      aria-label={label}
      aria-pressed={isSelected}
      density="compact"
      description={contact.alias?.trim()
        ? contact.display_name?.trim() || contact.name
        : undefined}
      leading={<UiAgentAvatar avatar={contact.avatar} name={label} size="md" />}
      onClick={onSelect}
      title={label}
      tooltip={label}
    />
  );
}

function AddContactDialog({
  agentId,
  agents,
  isPending,
  onAdd,
  onClose,
}: {
  agentId: string;
  agents: Agent[];
  isPending: boolean;
  onAdd: (contactAgentId: string, alias: string) => Promise<boolean>;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const titleId = useId();
  const searchRef = useRef<HTMLInputElement>(null);
  const submittingRef = useRef(false);
  const mountedRef = useRef(true);
  const [submitting, setSubmitting] = useState(false);
  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);
  const [query, setQuery] = useResettableState("", agentId);
  const [selectedAgentId, setSelectedAgentId] = useResettableState("", agentId);
  const [alias, setAlias] = useResettableState("", agentId);
  const search = createUiSearchMatcher(query);
  const candidates = agents.filter((candidate) => search.matches([
    getCommunicationAgentName(candidate),
  ]));
  const selectedAgent = agents.find((candidate) => candidate.agent_id === selectedAgentId);
  const busy = submitting || isPending;
  const selectedOutsideSearch = selectedAgent && !candidates.includes(selectedAgent);
  const close = () => {
    if (!busy && !submittingRef.current) onClose();
  };
  const submit = async () => {
    if (busy || submittingRef.current || !selectedAgent) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      if (await onAdd(selectedAgent.agent_id, alias) && mountedRef.current) onClose();
    } finally {
      submittingRef.current = false;
      if (mountedRef.current) setSubmitting(false);
    }
  };
  return (
    <UiDialogPortal>
      <UiDialogBackdrop initialFocusRef={searchRef} labelledBy={titleId} onClose={close}>
        <UiDialogFormShell
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          size="sm"
          viewport="compactMax"
        >
          <UiDialogHeader
            actions={<UiDialogCloseButton disabled={busy} onClose={close} />}
            appearance="plain"
            title={t("agent_options.contact.add_friend")}
            titleId={titleId}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <UiSearchInput
              aria-label={t("agent_options.contact.search_agents")}
              className="w-full"
              controlSize="md"
              disabled={busy}
              onChange={setQuery}
              placeholder={t("agent_options.contact.search_agents")}
              ref={searchRef}
              value={query}
              variant="dialog"
            />
            <UiPanel
              aria-label={t("agent_options.contact.search_agents")}
              className="soft-scrollbar max-h-72 min-h-36 space-y-0.5 overflow-y-auto p-1.5"
              padding="none"
              radius="md"
              role="group"
            >
              {candidates.length === 0 ? (
                <UiResourceState
                  className="min-h-32"
                  size="sm"
                  state="empty"
                  primaryAction={!search.empty ? { disabled: busy, label: t("common.clear"), onClick: () => setQuery("") } : undefined}
                  title={t(search.empty ? "agent_options.contact.no_available_agents" : "agent_options.contact.no_matching_agents")}
                  variant="plain"
                />
              ) : candidates.map((candidate) => {
                const candidateName = getCommunicationAgentName(candidate);
                const selected = selectedAgentId === candidate.agent_id;
                return (
                  <UiListRow
                    active={selected}
                    aria-label={candidateName}
                    aria-pressed={selected}
                    density="compact"
                    disabled={busy}
                    key={candidate.agent_id}
                    leading={(
                      <UiAgentAvatar
                        avatar={candidate.avatar}
                        name={candidateName}
                        size="md"
                      />
                    )}
                    onClick={() => setSelectedAgentId(candidate.agent_id)}
                    right={(
                      <Check
                        aria-hidden
                        className={cn(
                          "h-4 w-4 shrink-0 transition-opacity duration-(--motion-duration-fast)",
                          selected
                            ? "text-(--brand-action) opacity-100"
                            : "opacity-0",
                        )}
                      />
                    )}
                    title={candidateName}
                    tooltip={candidateName}
                  />
                );
              })}
            </UiPanel>
            <UiField
              description={selectedOutsideSearch
                ? t("agent_options.contact.selected_agent", { name: getCommunicationAgentName(selectedAgent) })
                : undefined}
              htmlFor={`${titleId}-alias`}
              label={t("agent_options.contact.alias")}
            >
              <UiInput
                controlSize="md"
                disabled={!selectedAgent || busy}
                id={`${titleId}-alias`}
                maxLength={128}
                onChange={(event) => setAlias(event.target.value)}
                placeholder={t("agent_options.contact.alias_placeholder")}
                value={alias}
                variant="dialog"
              />
            </UiField>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={close} type="button" variant="ghost">
              {t("common.cancel")}
            </UiButton>
            <UiButton aria-busy={busy || undefined} disabled={!selectedAgent || busy} tone="primary" type="submit">
              {busy ? (
                <LoaderCircle
                  aria-hidden
                  className={getUiSpinnerClassName({ size: "md" })}
                />
              ) : null}
              {t("agent_options.contact.add_friend")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

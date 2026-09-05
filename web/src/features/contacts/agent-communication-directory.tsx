// INPUT: 当前 Agent、联系人/候选目录、读取状态与增删选择命令。
// OUTPUT: 可搜索的共享列表行目录，以及使用统一 Dialog/Form/Panel 的添加联系人流程。
// POS: Contacts 联络目录视图；不拥有聊天 Session、消息时间线或服务端 mutation 真相。
"use client";

import {
  Check,
  LoaderCircle,
  MessageCircle,
  UserRoundPlus,
  UsersRound,
} from "lucide-react";
import { useMemo } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiField, UiInput, UiSearchInput } from "@/shared/ui/form/form-control";
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
        <div className="flex shrink-0 items-center gap-2 px-2 py-3">
          <UiSearchInput
            className="min-w-0 flex-1"
            controlSize="sm"
            onChange={setQuery}
            placeholder={t("agent_options.contact.search_contacts")}
            value={query}
          />
          <UiIconButton
            aria-label={t("agent_options.contact.add_friend")}
            onClick={() => setAddDialogOpen(true)}
            size="lg"
            title={t("agent_options.contact.add_friend")}
            variant="ghost"
          >
            <UserRoundPlus className="h-5 w-5" />
          </UiIconButton>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-2">
          {isDirectoryLoading && contacts.length === 0 && !directoryFailure ? (
            <AgentCommunicationEmptyState
              label={t("agent_options.contact.loading_address_book")}
              loading
            />
          ) : directoryFailure && !directoryFailure.stale ? (
            <AgentCommunicationReadFailureState
              failure={directoryFailure}
              onRetry={onRefresh}
            />
          ) : contacts.length === 0 ? (
            <>
              {directoryFailure ? (
                <AgentCommunicationReadFailureState
                  compact
                  failure={directoryFailure}
                  onRetry={onRefresh}
                />
              ) : null}
              <AgentCommunicationEmptyState
                icon={query ? MessageCircle : UsersRound}
                label={query
                  ? t("agent_options.contact.no_search_results")
                  : t("agent_options.contact.empty_directory")}
              />
            </>
          ) : (
            <>
              {directoryFailure ? (
                <AgentCommunicationReadFailureState
                  compact
                  failure={directoryFailure}
                  onRetry={onRefresh}
                />
              ) : null}
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
            </>
          )}
        </div>
      </aside>

      {addDialogOpen ? (
        <AddContactDialog
          agentId={agent.agent_id}
          agents={availableAgents}
          isPending={Boolean(pendingAgentId)}
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
  const [query, setQuery] = useResettableState("", agentId);
  const [selectedAgentId, setSelectedAgentId] = useResettableState("", agentId);
  const [alias, setAlias] = useResettableState("", agentId);
  const search = createUiSearchMatcher(query);
  const candidates = agents.filter((candidate) => search.matches([
    getCommunicationAgentName(candidate),
  ]));
  const titleId = `add-agent-contact-${agentId}`;
  const submit = async () => {
    if (selectedAgentId && await onAdd(selectedAgentId, alias)) {
      onClose();
    }
  };
  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={titleId} onClose={onClose}>
        <UiDialogFormShell
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          size="sm"
          viewport="compactMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("agent_options.contact.add_friend")}
            titleId={titleId}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <UiSearchInput
              className="w-full"
              controlSize="md"
              onChange={setQuery}
              placeholder={t("agent_options.contact.search_agents")}
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
                  title={t("agent_options.contact.no_available_agents")}
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
                  />
                );
              })}
            </UiPanel>
            <UiField
              htmlFor={`${titleId}-alias`}
              label={t("agent_options.contact.alias")}
            >
              <UiInput
                controlSize="md"
                disabled={!selectedAgentId || isPending}
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
            <UiButton onClick={onClose} type="button" variant="ghost">
              {t("common.cancel")}
            </UiButton>
            <UiButton disabled={!selectedAgentId || isPending} tone="primary" type="submit">
              {isPending ? (
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

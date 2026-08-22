/**
 * INPUT: Contacts 页面控制器、路由协调器与 Agent 联络资源。
 * OUTPUT: 可在目录、Agent 详情和创建/删除决策之间往返的页面装配。
 * POS: Contacts 路由页面；业务状态和导航动作分别下沉到 controller 与 orchestration。
 */
import type { ComponentProps } from "react";

import { AgentOptionsDialog } from "@/features/agents/options/dialog/agent-options-dialog";
import { ContactsAgentDetail } from "@/features/contacts/contacts-agent-detail";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

import {
  useAgentCommunication,
  type AgentCommunicationResource,
} from "./controller/use-agent-communication";
import { useContactsPageController } from "./controller/use-contacts-page-controller";
import {
  getContactsPagePresentation,
  type ContactsPageContentState,
} from "./contacts-page-model";
import { useContactsPageNavigation } from "./orchestration/use-contacts-page-navigation";

type ContactsAgentDetailActions = Omit<
  ComponentProps<typeof ContactsAgentDetail>,
  "agent" | "agents" | "communication"
>;

type ContactsDirectoryActions = Omit<
  ComponentProps<typeof ContactsDirectory>,
  "agents"
>;

interface ContactsPageActions extends
  ContactsAgentDetailActions,
  ContactsDirectoryActions {}

export function ContactsPage() {
  const controller = useContactsPageController();
  const navigation = useContactsPageNavigation({
    agents: controller.contactAgents,
    loading: controller.loading,
    confirmDeleteAgent: controller.confirmDeleteAgent,
    closeAgentEditor: controller.editor.close,
    openCreateAgent: controller.editor.openCreate,
  });
  const communication = useAgentCommunication(navigation.selectedAgent?.agent_id ?? null);

  const presentation = getContactsPagePresentation({
    contactCount: controller.contactAgents.length,
    loading: controller.loading,
    pendingDeleteAgent: controller.pendingDeleteAgent,
    selectedAgent: navigation.selectedAgent,
  });
  const actions: ContactsPageActions = {
    onAddContact: communication.addContact,
    onBackToAgentDirectory: navigation.openDirectory,
    onBackToCommunicationDirectory: communication.clearSelection,
    onCreateCommunicationConversation: communication.createConversation,
    onLoadOlderCommunicationMessages: communication.loadOlderMessages,
    onCreateAgent: controller.editor.openCreate,
    onCreateTeam: navigation.createTeam,
    onDeleteAgent: controller.requestDeleteAgent,
    onOpenAgent: navigation.openAgent,
    onOpenDirectRoom: navigation.openDirectRoom,
    onRefreshCommunication: communication.refresh,
    onRemoveCommunicationContact: communication.removeContact,
    onSaveAgentOptions: controller.saveAgentOptions,
    onSelectCommunicationContact: communication.selectContact,
    onSelectCommunicationConversation: communication.selectConversation,
    onSendCommunicationMessage: communication.sendMessage,
    onValidateAgentName: controller.validateAgentName,
  };

  if (presentation.content.kind === "loading") {
    return (
      <ContactsPageContent
        actions={actions}
        communication={communication}
        agents={controller.contactAgents}
        state={presentation.content}
      />
    );
  }

  return (
    <>
      <ContactsPageContent
        actions={actions}
        communication={communication}
        agents={controller.contactAgents}
        state={presentation.content}
      />

      <AgentOptionsDialog
        onClose={navigation.closeEditor}
        onDelete={controller.requestDeleteAgent}
        onSave={controller.editor.save}
        onValidateName={controller.editor.validateName}
        state={controller.editor.dialogState}
      />

      <ConfirmDialog
        confirmText="删除成员"
        isOpen={presentation.deleteDialog.isOpen}
        message={presentation.deleteDialog.message}
        onCancel={controller.cancelDeleteAgent}
        onConfirm={() => {
          void navigation.confirmDelete();
        }}
        title="删除成员"
        variant="danger"
      />
    </>
  );
}

function ContactsPageContent({
  actions,
  communication,
  agents,
  state,
}: {
  actions: ContactsPageActions;
  communication: AgentCommunicationResource;
  agents: ComponentProps<typeof ContactsDirectory>["agents"];
  state: ContactsPageContentState;
}) {
  switch (state.kind) {
    case "loading":
      return (
        <WorkspacePageFrame contentPaddingClassName="p-0">
          <WorkspaceLoadingState label="加载成员..." />
        </WorkspacePageFrame>
      );
    case "detail":
      return (
        <WorkspacePageFrame contentPaddingClassName="p-0">
          <ContactsAgentDetail
            agent={state.agent}
            agents={agents}
            communication={communication}
            onAddContact={actions.onAddContact}
            onBackToAgentDirectory={actions.onBackToAgentDirectory}
            onBackToCommunicationDirectory={actions.onBackToCommunicationDirectory}
            onCreateCommunicationConversation={actions.onCreateCommunicationConversation}
            onLoadOlderCommunicationMessages={actions.onLoadOlderCommunicationMessages}
            onCreateTeam={actions.onCreateTeam}
            onDeleteAgent={actions.onDeleteAgent}
            onOpenDirectRoom={actions.onOpenDirectRoom}
            onRefreshCommunication={actions.onRefreshCommunication}
            onRemoveCommunicationContact={actions.onRemoveCommunicationContact}
            onSaveAgentOptions={actions.onSaveAgentOptions}
            onSelectCommunicationContact={actions.onSelectCommunicationContact}
            onSelectCommunicationConversation={actions.onSelectCommunicationConversation}
            onSendCommunicationMessage={actions.onSendCommunicationMessage}
            onValidateAgentName={actions.onValidateAgentName}
          />
        </WorkspacePageFrame>
      );
    case "directory":
      return (
        <WorkspacePageFrame contentPaddingClassName="p-0">
          <ContactsDirectory
            agents={agents}
            onCreateAgent={actions.onCreateAgent}
            onCreateTeam={actions.onCreateTeam}
            onOpenAgent={actions.onOpenAgent}
            onOpenDirectRoom={actions.onOpenDirectRoom}
          />
        </WorkspacePageFrame>
      );
  }
}

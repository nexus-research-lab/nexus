import type { ComponentProps } from "react";

import { AgentOptionsDialog } from "@/features/agents/options/dialog/agent-options-dialog";
import { ContactsAgentDetail } from "@/features/contacts/contacts-agent-detail";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

import { useContactsPageController } from "./controller/use-contacts-page-controller";
import {
  getContactsPagePresentation,
  type ContactsPageContentState,
} from "./contacts-page-model";
import { useContactsPageNavigation } from "./orchestration/use-contacts-page-navigation";

type ContactsAgentDetailActions = Omit<
  ComponentProps<typeof ContactsAgentDetail>,
  "agent"
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
  });

  const presentation = getContactsPagePresentation({
    contactCount: controller.contactAgents.length,
    editingAgent: controller.editor.editingAgent,
    loading: controller.loading,
    pendingDeleteAgent: controller.pendingDeleteAgent,
    selectedAgent: navigation.selectedAgent,
  });
  const actions: ContactsPageActions = {
    onBack: navigation.backToDirectory,
    onCreateAgent: controller.editor.openCreate,
    onCreateTeam: navigation.createTeam,
    onDeleteAgent: controller.requestDeleteAgent,
    onEditAgent: controller.editor.openEdit,
    onOpenDirectRoom: navigation.openDirectRoom,
    onSaveAgentOptions: controller.saveAgentOptions,
    onValidateAgentName: controller.validateAgentName,
  };

  if (presentation.content.kind === "loading") {
    return <ContactsPageContent actions={actions} agents={controller.contactAgents} state={presentation.content} />;
  }

  return (
    <>
      <ContactsPageContent
        actions={actions}
        agents={controller.contactAgents}
        state={presentation.content}
      />

      <AgentOptionsDialog
        agentId={presentation.editor.agentId}
        initialOptions={controller.editor.initialOptions}
        initialAvatar={presentation.editor.initialAvatar}
        initialDescription={presentation.editor.initialDescription}
        initialTitle={presentation.editor.initialTitle}
        initialVibeTags={presentation.editor.initialVibeTags}
        isOpen={controller.editor.isOpen}
        mode={controller.editor.mode}
        onClose={controller.editor.close}
        onDelete={controller.requestDeleteAgent}
        onSave={controller.editor.save}
        onValidateName={controller.editor.validateName}
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
  agents,
  state,
}: {
  actions: ContactsPageActions;
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
            onBack={actions.onBack}
            onCreateTeam={actions.onCreateTeam}
            onDeleteAgent={actions.onDeleteAgent}
            onOpenDirectRoom={actions.onOpenDirectRoom}
            onSaveAgentOptions={actions.onSaveAgentOptions}
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
            onEditAgent={actions.onEditAgent}
            onOpenDirectRoom={actions.onOpenDirectRoom}
          />
        </WorkspacePageFrame>
      );
  }
}

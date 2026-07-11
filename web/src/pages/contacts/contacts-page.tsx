import { AgentOptionsDialog } from "@/features/agents/options/dialog/agent-options-dialog";
import { ContactsAgentDetail } from "@/features/contacts/contacts-agent-detail";
import { ContactsDirectory } from "@/features/contacts/contacts-directory";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

import { useContactsPageController } from "./controller/use-contacts-page-controller";
import { useContactsPageNavigation } from "./orchestration/use-contacts-page-navigation";

export function ContactsPage() {
  const controller = useContactsPageController();
  const navigation = useContactsPageNavigation({
    agents: controller.contactAgents,
    loading: controller.loading,
    confirmDeleteAgent: controller.confirmDeleteAgent,
  });

  if (controller.loading && controller.contactAgents.length === 0) {
    return (
      <WorkspacePageFrame contentPaddingClassName="p-0">
        <WorkspaceLoadingState label="加载成员..." />
      </WorkspacePageFrame>
    );
  }

  const editorAgent = controller.editor.editingAgent;
  const pendingDeleteAgent = controller.pendingDeleteAgent;

  return (
    <>
      <WorkspacePageFrame contentPaddingClassName="p-0">
        {navigation.selectedAgent ? (
          <ContactsAgentDetail
            agent={navigation.selectedAgent}
            onBack={navigation.backToDirectory}
            onCreateTeam={navigation.createTeam}
            onDeleteAgent={controller.requestDeleteAgent}
            onOpenDirectRoom={navigation.openDirectRoom}
            onSaveAgentOptions={controller.saveAgentOptions}
            onValidateAgentName={controller.validateAgentName}
          />
        ) : (
          <ContactsDirectory
            agents={controller.contactAgents}
            onCreateAgent={controller.editor.openCreate}
            onCreateTeam={navigation.createTeam}
            onEditAgent={controller.editor.openEdit}
            onOpenDirectRoom={navigation.openDirectRoom}
          />
        )}
      </WorkspacePageFrame>

      <AgentOptionsDialog
        agentId={editorAgent?.agent_id}
        initialOptions={controller.editor.initialOptions}
        initialAvatar={editorAgent?.avatar ?? ""}
        initialDescription={editorAgent?.description ?? ""}
        initialTitle={editorAgent?.name}
        initialVibeTags={editorAgent?.vibe_tags ?? []}
        isOpen={controller.editor.isOpen}
        mode={controller.editor.mode}
        onClose={controller.editor.close}
        onDelete={controller.requestDeleteAgent}
        onSave={controller.editor.save}
        onValidateName={controller.editor.validateName}
      />

      <ConfirmDialog
        confirmText="删除成员"
        isOpen={Boolean(pendingDeleteAgent)}
        message={`删除「${pendingDeleteAgent?.name ?? "该 Agent"}」后，该成员将不再出现在 Contacts 中。已有历史协作不会自动删除。`}
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

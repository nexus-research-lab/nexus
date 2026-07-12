import type { Agent } from "@/types/agent/agent";

export type ContactsPageContentState =
  | { agent: Agent; kind: "detail" }
  | { kind: "directory" }
  | { kind: "loading" };

interface ContactsEditorPresentation {
  agentId: string | undefined;
  initialAvatar: string;
  initialDescription: string;
  initialTitle: string | undefined;
  initialVibeTags: string[];
}

interface ContactsDeleteDialogPresentation {
  isOpen: boolean;
  message: string;
}

interface ContactsPagePresentation {
  content: ContactsPageContentState;
  deleteDialog: ContactsDeleteDialogPresentation;
  editor: ContactsEditorPresentation;
}

function getContactsContentState({
  contactCount,
  loading,
  selectedAgent,
}: {
  contactCount: number;
  loading: boolean;
  selectedAgent: Agent | null;
}): ContactsPageContentState {
  if (loading && contactCount === 0) {
    return { kind: "loading" };
  }
  return selectedAgent
    ? { agent: selectedAgent, kind: "detail" }
    : { kind: "directory" };
}

function getContactsEditorPresentation(
  editingAgent: Agent | null,
): ContactsEditorPresentation {
  if (!editingAgent) {
    return {
      agentId: undefined,
      initialAvatar: "",
      initialDescription: "",
      initialTitle: undefined,
      initialVibeTags: [],
    };
  }
  return {
    agentId: editingAgent.agent_id,
    initialAvatar: editingAgent.avatar ?? "",
    initialDescription: editingAgent.description ?? "",
    initialTitle: editingAgent.name,
    initialVibeTags: editingAgent.vibe_tags ?? [],
  };
}

function getContactsDeleteDialogPresentation(
  pendingDeleteAgent: { name: string } | null,
): ContactsDeleteDialogPresentation {
  const agentName = pendingDeleteAgent?.name ?? "该 Agent";
  return {
    isOpen: pendingDeleteAgent !== null,
    message: `删除「${agentName}」后，该成员将不再出现在 Contacts 中。已有历史协作不会自动删除。`,
  };
}

export function getContactsPagePresentation({
  contactCount,
  editingAgent,
  loading,
  pendingDeleteAgent,
  selectedAgent,
}: {
  contactCount: number;
  editingAgent: Agent | null;
  loading: boolean;
  pendingDeleteAgent: { name: string } | null;
  selectedAgent: Agent | null;
}): ContactsPagePresentation {
  return {
    content: getContactsContentState({ contactCount, loading, selectedAgent }),
    deleteDialog: getContactsDeleteDialogPresentation(pendingDeleteAgent),
    editor: getContactsEditorPresentation(editingAgent),
  };
}

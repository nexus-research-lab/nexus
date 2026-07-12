import type { Agent } from "@/types/agent/agent";

export type ContactsPageContentState =
  | { agent: Agent; kind: "detail" }
  | { kind: "directory" }
  | { kind: "loading" };

interface ContactsDeleteDialogPresentation {
  isOpen: boolean;
  message: string;
}

interface ContactsPagePresentation {
  content: ContactsPageContentState;
  deleteDialog: ContactsDeleteDialogPresentation;
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
  loading,
  pendingDeleteAgent,
  selectedAgent,
}: {
  contactCount: number;
  loading: boolean;
  pendingDeleteAgent: { name: string } | null;
  selectedAgent: Agent | null;
}): ContactsPagePresentation {
  return {
    content: getContactsContentState({ contactCount, loading, selectedAgent }),
    deleteDialog: getContactsDeleteDialogPresentation(pendingDeleteAgent),
  };
}

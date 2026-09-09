// INPUT: Contacts 资源状态与删除操作的领域证据。
// OUTPUT: 页面内容和准确回答结果、影响、下一步的删除弹窗文案。
// POS: Contacts 页纯展示模型；不得把内部 effect、code 或诊断 ID 暴露给用户。
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { AgentDeletionFailure } from "./controller/use-contacts-page-controller";

export type ContactsPageContentState =
  | { agent: Agent; kind: "detail" }
  | { kind: "directory" }
  | { kind: "loading" };

interface ContactsDeleteDialogPresentation {
  confirmText: string;
  failure: {
    impact: string;
    nextStep: string;
    title: string;
  } | null;
  isOpen: boolean;
  message: string;
  variant: "danger" | "default";
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
  deleteFailure: AgentDeletionFailure | null,
  t: I18nContextValue["t"],
): ContactsDeleteDialogPresentation {
  const agentName = pendingDeleteAgent?.name ?? t("contacts.delete.agent_fallback");
  const failure = getAgentDeleteFailurePresentation(deleteFailure, t);
  const needsDirectoryCheck = deleteFailure !== null && deleteFailure.kind !== "not_applied";
  return {
    confirmText: needsDirectoryCheck ? t("contacts.delete.refresh") : t("contacts.delete.confirm"),
    failure,
    isOpen: pendingDeleteAgent !== null,
    message: t("contacts.delete.message", { name: agentName }),
    variant: needsDirectoryCheck ? "default" : "danger",
  };
}

function getAgentDeleteFailurePresentation(
  failure: AgentDeletionFailure | null,
  t: I18nContextValue["t"],
): ContactsDeleteDialogPresentation["failure"] {
  if (!failure) {
    return null;
  }
  if (failure.kind === "not_applied") {
    return {
      title: t("contacts.delete.not_applied_title"),
      impact: t("contacts.delete.not_applied_impact"),
      nextStep: t("contacts.delete.not_applied_next"),
    };
  }
  if (failure.kind === "committed_cleanup_incomplete") {
    return failure.directoryCheck === "failed"
      ? {
          title: t("contacts.delete.committed_failed_title"),
          impact: t("contacts.delete.committed_failed_impact"),
          nextStep: t("contacts.delete.refresh_network_no_delete"),
        }
      : failure.directoryCheck === "target_present"
        ? {
            title: t("contacts.delete.committed_present_title"),
            impact: t("contacts.delete.committed_present_impact"),
            nextStep: t("contacts.delete.refresh_later_no_delete"),
          }
        : {
          title: t("contacts.delete.committed_title"),
          impact: t("contacts.delete.committed_impact"),
          nextStep: t("contacts.delete.committed_next"),
        };
  }
  if (failure.kind === "resource_absent") {
    return failure.directoryCheck === "failed"
      ? {
          title: t("contacts.delete.absent_failed_title"),
          impact: t("contacts.delete.absent_failed_impact"),
          nextStep: t("contacts.delete.refresh_network"),
        }
      : failure.directoryCheck === "target_present"
        ? {
            title: t("contacts.delete.absent_present_title"),
            impact: t("contacts.delete.absent_present_impact"),
            nextStep: t("contacts.delete.absent_present_next"),
          }
        : {
          title: t("contacts.delete.absent_title"),
          impact: t("contacts.delete.absent_impact"),
          nextStep: t("contacts.delete.absent_next"),
        };
  }
  return failure.directoryCheck === "failed"
    ? {
        title: t("contacts.delete.unknown_failed_title"),
        impact: t("contacts.delete.unknown_failed_impact"),
        nextStep: t("contacts.delete.refresh_network_no_delete"),
      }
    : failure.directoryCheck === "target_present"
      ? {
          title: t("contacts.delete.unknown_present_title"),
          impact: t("contacts.delete.unknown_present_impact"),
          nextStep: t("contacts.delete.unknown_present_next"),
        }
      : {
        title: t("contacts.delete.unknown_title"),
        impact: t("contacts.delete.unknown_impact"),
        nextStep: t("contacts.delete.unknown_next"),
      };
}

export function getContactsPagePresentation({
  t,
  contactCount,
  deleteFailure,
  loading,
  pendingDeleteAgent,
  selectedAgent,
}: {
  contactCount: number;
  deleteFailure: AgentDeletionFailure | null;
  loading: boolean;
  pendingDeleteAgent: { name: string } | null;
  selectedAgent: Agent | null;
  t: I18nContextValue["t"];
}): ContactsPagePresentation {
  return {
    content: getContactsContentState({ contactCount, loading, selectedAgent }),
    deleteDialog: getContactsDeleteDialogPresentation(
      pendingDeleteAgent,
      deleteFailure,
      t,
    ),
  };
}

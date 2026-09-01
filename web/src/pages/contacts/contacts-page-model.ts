// INPUT: Contacts 资源状态与删除操作的领域证据。
// OUTPUT: 页面内容和准确回答结果、影响、下一步的删除弹窗文案。
// POS: Contacts 页纯展示模型；不得把内部 effect、code 或诊断 ID 暴露给用户。
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
): ContactsDeleteDialogPresentation {
  const agentName = pendingDeleteAgent?.name ?? "该 Agent";
  const failure = getAgentDeleteFailurePresentation(deleteFailure);
  const needsDirectoryCheck = deleteFailure !== null && deleteFailure.kind !== "not_applied";
  return {
    confirmText: needsDirectoryCheck ? "刷新成员列表" : "删除成员",
    failure,
    isOpen: pendingDeleteAgent !== null,
    message: `删除「${agentName}」后，该智能体及其会话、工作文件、目标和定时任务会被清理，相关绑定会失效。此操作无法从本页恢复。`,
    variant: needsDirectoryCheck ? "default" : "danger",
  };
}

function getAgentDeleteFailurePresentation(
  failure: AgentDeletionFailure | null,
): ContactsDeleteDialogPresentation["failure"] {
  if (!failure) {
    return null;
  }
  if (failure.kind === "not_applied") {
    return {
      title: "成员没有删除",
      impact: "这次删除没有生效；该智能体及其会话、工作文件和任务仍然保留。",
      nextStep: "可以再次删除，或取消后继续保留。",
    };
  }
  if (failure.kind === "committed_cleanup_incomplete") {
    return failure.directoryCheck === "failed"
      ? {
          title: "成员已删除，列表暂时无法更新",
          impact: "删除已经提交，但部分关联内容没有完全清理；当前页面仍显示上次内容。",
          nextStep: "检查网络连接后重新刷新成员列表，不要再次删除。",
        }
      : failure.directoryCheck === "target_present"
        ? {
            title: "成员仍在列表中，删除状态存在冲突",
            impact: "服务端曾确认删除已经提交，但当前列表仍包含这个成员；当前页面不能判断哪些关联内容已完成清理。",
            nextStep: "稍后再次刷新成员列表，不要再次删除。",
          }
        : {
          title: "成员已删除，关联内容未完全清理",
          impact: "删除已经提交；部分会话、工作文件或任务仍需处理。",
          nextStep: "刷新成员列表确认最新状态，不要再次删除。",
        };
  }
  if (failure.kind === "resource_absent") {
    return failure.directoryCheck === "failed"
      ? {
          title: "成员列表暂时无法更新",
          impact: "服务端已确认该成员不存在，但当前页面仍显示上次内容；这次请求没有删除其他内容。",
          nextStep: "检查网络连接后重新刷新成员列表。",
        }
      : failure.directoryCheck === "target_present"
        ? {
            title: "成员状态存在冲突",
            impact: "删除接口没有找到这个成员，但当前列表仍包含它；这次请求没有删除其他成员。",
            nextStep: "稍后再次刷新成员列表，不要重新删除。",
          }
        : {
          title: "成员已经不存在",
          impact: "服务端没有找到这个成员；这次请求没有删除其他内容，当前列表尚未核对。",
          nextStep: "刷新成员列表同步最新状态。",
        };
  }
  return failure.directoryCheck === "failed"
    ? {
        title: "成员状态仍无法确认",
        impact: "删除结果未确认，当前页面显示上次内容；重复删除有重复清理风险。",
        nextStep: "检查网络连接后重新刷新成员列表，不要再次删除。",
      }
    : failure.directoryCheck === "target_present"
      ? {
          title: "成员目前仍在列表中，但删除结果没有确认",
          impact: "当前列表仍有这个成员，但一次读取不能证明先前删除结果；确认前保持保护。",
          nextStep: "稍后再次刷新成员列表，确认前不要重新删除。",
        }
      : {
        title: "还无法确认成员是否已删除",
        impact: "未收到完整结果；成员删除结果待核对，系统没有自动再次删除。",
        nextStep: "先刷新成员列表确认状态，不要再次删除。",
      };
}

export function getContactsPagePresentation({
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
}): ContactsPagePresentation {
  return {
    content: getContactsContentState({ contactCount, loading, selectedAgent }),
    deleteDialog: getContactsDeleteDialogPresentation(
      pendingDeleteAgent,
      deleteFailure,
    ),
  };
}

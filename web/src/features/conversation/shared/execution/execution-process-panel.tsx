/**
 * INPUT: 当前托管 Execution、Agent 目录、打开完整工作图与精确 Agent round 导航动作。
 * OUTPUT: Composer 上方只包含一级 Agent 的实时活动 Dock；不复制完整图、Tool、Gate 或 Subagent。
 * POS: DM 与 Room 共用的 WorkGraph 快速入口；完整节点关系与详情只在右侧 WorkGraph Surface 展示。
 */
"use client";

import { Workflow } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getConversationActivityToolbarClassName } from "@/shared/ui/workspace/surface/conversation-activity-chip-styles";
import type { ExecutionView } from "@/types/conversation/execution";

import {
  EXECUTION_STATUS_LABEL_KEY,
  resolveExecutionGraphNodeAgent,
  resolveExecutionGraphNodeItem,
  resolveExecutionGraphNodeStatus,
  resolveExecutionNodeSummary,
  resolveExecutionPrimaryAgentNodes,
  type ExecutionAgentDirectory,
  WORK_ITEM_STATUS_LABEL_KEY,
} from "./execution-process-model";
import { ExecutionNodeAvatar } from "./execution-node-avatar";

export function ExecutionProcessPanel({
  className,
  directory,
  execution,
  onNavigateToRound,
  onOpenGraph,
}: {
  className?: string;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  onNavigateToRound?: (roundId: string) => void;
  onOpenGraph?: () => void;
}) {
  const { t } = useI18n();
  const nodeSummary = resolveExecutionNodeSummary(execution);
  const agentNodes = resolveExecutionPrimaryAgentNodes(execution);
  const nodeProgressLabel = nodeSummary.totalCount > 0
    ? t("execution.node_progress", {
        current: nodeSummary.currentStep,
        total: nodeSummary.totalCount,
      })
    : t(EXECUTION_STATUS_LABEL_KEY[execution.status]);

  return (
    <aside
      aria-label={t("execution.agent_activity")}
      aria-live="polite"
      className={cn(
        "pointer-events-none relative flex w-full min-w-0 max-w-[460px] justify-center",
        className,
      )}
      data-execution-process-panel
      data-execution-status={execution.status}
    >
      <div
        className={getConversationActivityToolbarClassName("pointer-events-auto flex max-w-full items-center overflow-hidden")}
        data-execution-agent-activity-dock
      >
        {agentNodes.map((node, index) => {
          const item = resolveExecutionGraphNodeItem(execution, node);
          const owner = resolveExecutionGraphNodeAgent(directory, node, item);
          const status = resolveExecutionGraphNodeStatus(node, item);
          const live = status === "running";
          const statusLabel = t(WORK_ITEM_STATUS_LABEL_KEY[status]);
          const subject = item?.subject.trim()
            || node.description?.trim()
            || owner?.name
            || t("execution.owner_unassigned");
          const title = `${owner?.name ?? t("execution.owner_unassigned")} · ${subject} · ${statusLabel}`;
          const canNavigate = Boolean(node.agent_round_id && onNavigateToRound);
          return (
            <span className="inline-flex shrink-0 items-center gap-1" key={node.id}>
              {index > 0 ? (
                <span
                  aria-hidden="true"
                  className="h-px w-2.5 bg-(--divider-subtle-color)"
                  data-execution-agent-connection
                />
              ) : null}
              <UiIconButton
                aria-label={canNavigate
                  ? t("execution.jump_to_agent_output", {
                      agent: owner?.name ?? t("execution.owner_unassigned"),
                    })
                  : `${t("execution.open_workgraph")} · ${title}`}
                className="shrink-0 transition-transform hover:scale-[1.03]"
                data-execution-agent-activity={owner?.id ?? node.id}
                data-execution-agent-live={live ? "true" : undefined}
                data-execution-agent-round-id={node.agent_round_id || undefined}
                onClick={() => {
                  if (node.agent_round_id && onNavigateToRound) {
                    onNavigateToRound(node.agent_round_id);
                    return;
                  }
                  onOpenGraph?.();
                }}
                size="md"
                tooltip={title}
              >
                <ExecutionNodeAvatar
                  agent={owner}
                  current={live}
                  kind="agent"
                  size="dock"
                  status={status}
                  title={title}
                  tone="activity"
                />
              </UiIconButton>
            </span>
          );
        })}

        <span
          aria-hidden="true"
          className="mx-1 h-5 w-px shrink-0 bg-(--divider-subtle-color)"
        />
        <UiIconButton
          aria-label={t("execution.open_workgraph")}
          className="shrink-0"
          data-execution-open-workgraph
          onClick={onOpenGraph}
          size="md"
          tooltip={`${t("execution.open_workgraph")} · ${nodeSummary.summary} · ${nodeProgressLabel}`}
        >
          <Workflow aria-hidden="true" className="h-[18px] w-[18px]" />
        </UiIconButton>
      </div>
    </aside>
  );
}

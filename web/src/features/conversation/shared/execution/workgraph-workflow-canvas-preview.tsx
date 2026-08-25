/**
 * INPUT: durable 命名工作图或临时 WorkGraph Preview，以及消费面的可选尺寸 class。
 * OUTPUT: 统一投影并以标题、目标和关键标记摘要节点完整复用 ExecutionWorkGraphCanvas 的只读预览 Surface。
 * POS: 对话确认、能力详情等非运行态工作图查看面的唯一完整画布入口；尺寸由外层消费面决定。
 */
"use client";

import { useMemo } from "react";

import { cn } from "@/shared/ui/class-name";
import type {
  WorkGraphWorkflow,
  WorkGraphWorkflowPreview,
} from "@/types/conversation/workgraph-workflow";

import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";
import { projectWorkGraphWorkflowCanvasExecution } from "./workgraph-workflow-canvas-model";

export function WorkGraphWorkflowCanvasPreview({
  className,
  revision,
  workflow,
}: {
  className?: string;
  revision?: number;
  workflow: WorkGraphWorkflow | WorkGraphWorkflowPreview;
}) {
  const resolvedRevision = revision ?? ("version" in workflow ? workflow.version : 1);
  const execution = useMemo(
    () => projectWorkGraphWorkflowCanvasExecution(workflow, resolvedRevision),
    [resolvedRevision, workflow],
  );

  return (
    <div
      className={cn("flex min-h-0 min-w-0", className)}
      data-workgraph-workflow-canvas-preview
    >
      <ExecutionWorkGraphCanvas
        currentId={null}
        directory={{}}
        execution={execution}
        nodePresentation="summary"
        taskRuns={[]}
      />
    </div>
  );
}

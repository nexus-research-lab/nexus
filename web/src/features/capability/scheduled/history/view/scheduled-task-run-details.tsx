// INPUT: 单次运行的规范化输出、持久 Session 身份、诊断行与复制动作。
// OUTPUT: 绑定历史执行 Agent 的结果预览、错误与可折叠诊断详情。
// POS: Scheduled 历史详情消费侧；不猜测历史资源归属，不决定重跑或投递恢复行为。

"use client";

import { Copy } from "lucide-react";

import { useWorkspaceMarkdown } from "@/hooks/agent/use-workspace-markdown";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";

import {
  getRunDiagnosticRows,
  getRunOutputSections,
  getRunWorkspaceAgentID,
  type RunOutputSection,
} from "../scheduled-task-run-diagnostic-model";

interface ScheduledTaskRunDetailsProps {
  isCopied: boolean;
  onCopyDiagnostic: () => void | Promise<void>;
  run: ScheduledTaskRunItem;
}

export function ScheduledTaskRunDetails({
  isCopied,
  onCopyDiagnostic,
  run,
}: ScheduledTaskRunDetailsProps) {
  const diagnosticRows = getRunDiagnosticRows(run);
  const outputSections = getRunOutputSections(run);
  const workspaceAgentId = getRunWorkspaceAgentID(run);
  return (
    <>
      {outputSections.map((section, index) => (
        <RunOutput key={`${section.label ?? section.tone}:${index}`} section={section} workspaceAgentId={workspaceAgentId} />
      ))}
      <UiDisclosure
        className="mt-4"
        label="诊断详情"
        summaryRole="caption"
        variant="inline"
      >
        <UiPanel className="space-y-1.5" padding="sm" radius="sm">
          {diagnosticRows.map((row) => (
            <p className={cn(row.breakAll && "break-all")} key={row.label}>
              {row.label} {row.value}
            </p>
          ))}
          <UiButton
            className="mt-1"
            onClick={() => void onCopyDiagnostic()}
            size="xs"
            variant="text"
          >
            <Copy className="h-3.5 w-3.5" />
            {isCopied ? "已复制" : "复制诊断"}
          </UiButton>
        </UiPanel>
      </UiDisclosure>
    </>
  );
}

function RunOutput({ section, workspaceAgentId }: {
  section: RunOutputSection;
  workspaceAgentId: string | null;
}) {
  const { getFilePreviewUrl, resolveFilePath } = useWorkspaceMarkdown(workspaceAgentId);
  if (section.tone === "default") {
    return (
      <div className="mt-3 min-w-0">
        {section.label ? (
          <p className={getUiTypographyClassName({ role: "overline", tone: "muted" })}>
            {section.label}
          </p>
        ) : null}
        <UiMarkdownContent
          className={cn(
            getUiTypographyClassName({ role: "supporting", tone: "default" }),
            section.label && "mt-2",
          )}
          content={section.content}
          getFilePreviewUrl={getFilePreviewUrl}
          mermaidShowHeader={false}
          resolveFilePath={resolveFilePath}
        />
      </div>
    );
  }
  return (
    <UiPanel
      className="mt-3 min-w-0 border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)]"
      padding="sm"
      radius="sm"
    >
      {section.label ? (
        <p className={getUiTypographyClassName({ role: "overline", tone: "muted" })}>
          {section.label}
        </p>
      ) : null}
      <p className={cn(
        "whitespace-pre-wrap break-words",
        getUiTypographyClassName({ role: "supporting", tone: "danger" }),
      )}>{section.content}</p>
    </UiPanel>
  );
}

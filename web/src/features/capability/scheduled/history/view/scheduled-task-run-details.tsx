// INPUT: 单次运行的规范化输出、持久 Session 身份、诊断行与复制动作。
// OUTPUT: 绑定历史执行 Agent 的结果/错误摘要，完整错误仅进入折叠诊断，分组边界由公共 Panel 承载。
// POS: Scheduled 历史详情消费侧；不猜测历史资源归属，不决定重跑或投递恢复行为。

"use client";

import { Copy } from "lucide-react";

import { useWorkspaceMarkdown } from "@/hooks/agent/use-workspace-markdown";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  const { locale, t } = useI18n();
  const diagnosticRows = getRunDiagnosticRows(run, { locale, t });
  const outputSections = getRunOutputSections(run, t);
  const workspaceAgentId = getRunWorkspaceAgentID(run);
  return (
    <>
      {outputSections.map((section, index) => (
        <RunOutput key={`${section.tone}:${index}`} section={section} workspaceAgentId={workspaceAgentId} />
      ))}
      <UiDisclosure
        className="mt-4"
        label={t("capability.scheduled_history_diagnostics")}
        summaryRole="caption"
        variant="inline"
      >
        <UiPanel className="space-y-1.5" padding="sm" radius="sm">
          {diagnosticRows.map((row) => (
            <p className={cn("whitespace-pre-wrap", row.breakAll && "break-all")} key={row.label}>
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
            {t(isCopied ? "capability.scheduled_history_copied" : "capability.scheduled_history_copy_diagnostics")}
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
        <UiMarkdownContent
          className={getUiTypographyClassName({ role: "supporting", tone: "default" })}
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
      className="mt-3 min-w-0"
      padding="sm"
      radius="sm"
    >
      <p className={cn(
        "whitespace-pre-wrap break-words",
        getUiTypographyClassName({ role: "supporting", tone: "danger" }),
      )}>{section.content}</p>
    </UiPanel>
  );
}

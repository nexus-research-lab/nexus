/**
 * INPUT: 单个命名 WorkGraph、返回/复制/继续编辑动作。
 * OUTPUT: 对象说明、目标摘要、共享动作与只读完整画布。
 * POS: WorkGraph 能力详情纯视图；不读取路由、资源或命令状态。
 */
"use client";

import type { ReactNode } from "react";

import { CapabilityDetailPage } from "@/features/capability/shared/capability-page-layout";
import { WorkGraphWorkflowCanvasPreview } from "@/features/conversation/shared/execution/workgraph-workflow-canvas-preview";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

interface WorkGraphDistillationDetailProps {
  item: WorkGraphWorkflow;
  notice?: ReactNode;
  onBack: () => void;
  onCopy: () => void;
  onEdit: () => void;
}

export function WorkGraphDistillationDetail({
  item,
  notice,
  onBack,
  onCopy,
  onEdit,
}: WorkGraphDistillationDetailProps) {
  const { t } = useI18n();
  return (
    <CapabilityDetailPage
      backLabel={t("capability.workgraph_distillations")}
      className="flex min-h-full flex-1 flex-col"
      currentTitle={`/${item.slash_name}`}
      onBack={onBack}
    >
      {notice}
      <div className="mt-3 flex min-h-0 flex-1 flex-col gap-4">
        <div className="flex shrink-0 flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 flex-1">
            <h2 className={cn(
              "break-words",
              getUiTypographyClassName({ role: "objectTitle", tone: "strong" }),
            )}>
              /{item.slash_name}
            </h2>
            {item.description ? (
              <p className={cn(
                "mt-1",
                getUiTypographyClassName({ role: "supporting", tone: "muted" }),
              )}>
                {item.description}
              </p>
            ) : null}
          </div>
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {!item.built_in ? (
              <UiButton onClick={onEdit} size="sm" variant="surface">
                {t("capability.workgraph_edit")}
              </UiButton>
            ) : null}
            <UiButton onClick={onCopy} size="sm" tone="primary" variant="solid">
              {t("capability.workgraph_copy")}
            </UiButton>
          </div>
        </div>
        <UiPanel
          className={cn(
            "shrink-0",
            getUiTypographyClassName({ role: "body", tone: "default" }),
          )}
          padding="sm"
          radius="md"
        >
          {item.objective}
        </UiPanel>
        <WorkGraphWorkflowCanvasPreview
          className="min-h-[360px] flex-1 overflow-hidden surface-radius-md border border-(--divider-subtle-color) bg-(--surface-canvas-background)"
          workflow={item}
        />
      </div>
    </CapabilityDetailPage>
  );
}

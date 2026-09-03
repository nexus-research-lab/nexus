/**
 * INPUT: 单个命名 WorkGraph、返回/复制/继续编辑动作。
 * OUTPUT: 共享对象身份区中的说明与动作、目标摘要及只读完整画布。
 * POS: WorkGraph 能力详情纯视图；身份几何归 capability/shared，不读取路由、资源或命令状态。
 */
"use client";

import type { ReactNode } from "react";

import {
  CapabilityDetailIdentity,
  CapabilityDetailPage,
} from "@/features/capability/shared/capability-page-layout";
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
        <CapabilityDetailIdentity
          actions={(
            <>
              {!item.built_in ? (
                <UiButton onClick={onEdit} size="sm" variant="surface">
                  {t("capability.workgraph_edit")}
                </UiButton>
              ) : null}
              <UiButton onClick={onCopy} size="sm" tone="primary" variant="solid">
                {t("capability.workgraph_copy")}
              </UiButton>
            </>
          )}
          description={item.description}
          title={`/${item.slash_name}`}
        />
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

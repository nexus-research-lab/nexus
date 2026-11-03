/**
 * INPUT: 单个命名 WorkGraph、返回/复制/继续编辑动作。
 * OUTPUT: 共享对象身份区中的统一方法说明与动作，参数输入示例、按版本统计与只读完整画布。
 * POS: WorkGraph 能力详情纯视图；身份几何归 capability/shared，不读取路由、资源或命令状态。
 */
"use client";

import type { ReactNode } from "react";

import {
  CapabilityDetailIdentity,
  CapabilityDetailPage,
} from "@/features/capability/shared/capability-page-layout";
import { WorkGraphWorkflowCanvasPreview } from "@/features/conversation/shared/execution/workgraph-workflow-canvas-preview";
import { workgraphInvocationExample } from "@/features/conversation/shared/execution/workgraph-invocation";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

interface WorkGraphDistillationDetailProps {
  item: WorkGraphWorkflow;
  editDisabled?: boolean;
  notice?: ReactNode;
  onBack: () => void;
  onCopy: () => void;
  onEdit: () => void;
}

export function WorkGraphDistillationDetail({
  item,
  editDisabled = false,
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
      <div className="flex min-h-0 flex-1 flex-col gap-4">
        <CapabilityDetailIdentity
          actions={(
            <>
              {!item.built_in ? (
                <UiButton disabled={editDisabled} onClick={onEdit} size="sm" variant="surface">
                  {t("capability.workgraph_edit")}
                </UiButton>
              ) : null}
              <UiButton onClick={onCopy} size="sm" tone="primary" variant="solid">
                {t("capability.workgraph_copy")}
              </UiButton>
            </>
          )}
          description={item.description}
          leading={<UiSeededAvatar seed={item.slash_name} size="lg" />}
          title={`/${item.slash_name}`}
        />
        {item.parameters?.length ? (
          <UiPanel padding="sm" variant="plain" className="space-y-2">
            <h2 className={getUiTypographyClassName({ role: "sectionTitle" })}>{t("capability.workgraph_parameters")}</h2>
            <dl className={getUiTypographyClassName({ role: "body" })}>
              {item.parameters.map((parameter) => (
                <div className="mb-2" key={parameter.name}>
                  <dt>{parameter.name} · {t(`capability.parameter_${parameter.type}`)} · {t(parameter.required ? "capability.parameter_required" : "capability.parameter_optional")}</dt>
                  <dd>{parameter.description}</dd>
                  {parameter.default !== undefined ? <dd>{t("capability.parameter_default")}: {JSON.stringify(parameter.default)}</dd> : null}
                  {parameter.enum?.length ? <dd>{parameter.enum.join(" / ")}</dd> : null}
                  {parameter.minimum !== undefined || parameter.maximum !== undefined ? <dd>{parameter.minimum ?? "−∞"} … {parameter.maximum ?? "∞"}</dd> : null}
                </div>
              ))}
            </dl>
            <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>{t("capability.parameter_example_hint")}</p>
            <pre className="whitespace-pre-wrap break-words">{workgraphInvocationExample(item)}</pre>
          </UiPanel>
        ) : null}
        {item.statistics?.length ? (
          <UiPanel padding="sm" variant="plain" className="space-y-2">
            <h2 className={getUiTypographyClassName({ role: "sectionTitle" })}>{t("capability.workgraph_statistics")}</h2>
            {item.statistics.map((stat) => (
              <p key={stat.version} className={getUiTypographyClassName({ role: "body" })}>
                {t("capability.workgraph_version_stats", { version: stat.version, runs: stat.runs, completed: stat.completed, closed: stat.closed_without_completion, active: stat.active, failed: stat.failed_attempts, attempts: stat.attempts })}
                {stat.elapsed_samples > 0 ? ` · ${t("capability.workgraph_elapsed_stats", { seconds: Math.round(stat.elapsed_milliseconds / stat.elapsed_samples / 1000), samples: stat.elapsed_samples })}` : ""}
              </p>
            ))}
            <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>{t("capability.workgraph_statistics_hint")}</p>
          </UiPanel>
        ) : null}
        <WorkGraphWorkflowCanvasPreview
          className="min-h-[360px] flex-1 overflow-hidden surface-radius-md border border-(--divider-subtle-color) bg-(--surface-canvas-background)"
          workflow={item}
        />
      </div>
    </CapabilityDetailPage>
  );
}

/**
 * INPUT: Loop slug、本地化资源与返回动作。
 * OUTPUT: Loop 步骤、退出条件、护栏和启动指令详情。
 * POS: 工作循环长内容详情；目录统计不在此重复堆叠。
 */
"use client";

import { useEffect, useState } from "react";
import { ArrowLeft, Check, Copy, RotateCcw } from "lucide-react";

import { getLoopApi } from "@/lib/api/capability/loop-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import {
  WorkspaceContentDetailHeader,
  WorkspaceContentHeader,
} from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { buildLoopMetadataPresentation } from "./loop-presentation";

interface LoopDetailViewProps {
  slug: string;
  onBack: () => void;
}

interface LoopDetailState {
  error: ResourceFailure | null;
  loading: boolean;
  loop: LoopCatalogItem | null;
}

export function LoopDetailView({ slug, onBack: onBack }: LoopDetailViewProps) {
  const { locale, t } = useI18n();
  const [state, setState] = useResettableState<LoopDetailState>(
    { error: null, loading: true, loop: null },
    `${slug}\x1f${locale}`,
  );
  const [copied, setCopied] = useState(false);
  const [loadRevision, setLoadRevision] = useState(0);
  const { error, loading, loop } = state;
  const metadata = loop
    ? buildLoopMetadataPresentation(loop, locale, t)
    : null;

  useEffect(() => {
    let cancelled = false;
    setState({ error: null, loading: true, loop: null });
    getLoopApi(slug, locale)
      .then((item) => {
        if (!cancelled) {
          setState({ error: null, loading: false, loop: item });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            error: getResourceFailure(err, t("capability.loops_loading_failed")),
            loading: false,
            loop: null,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [loadRevision, locale, setState, slug, t]);

  const copyPrompt = async () => {
    if (!loop) {
      return;
    }
    if (await writeTextToClipboard(loop.kickoff_prompt)) {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    }
  };

  return (
    <div className={WORKSPACE_CONTENT_PAGE_CLASS_NAME}>
      <WorkspaceContentDetailHeader>
        <UiButton size="sm" variant="text" onClick={onBack}>
          <ArrowLeft className="h-4 w-4" />
          {t("common.back")}
        </UiButton>
      </WorkspaceContentDetailHeader>

      {loading ? (
        <UiResourceState
          className="min-h-[320px]"
          size="md"
          state="loading"
          title={t("capability.connectors_loading")}
          variant="plain"
        />
      ) : error ? (
        <UiResourceState
          className="min-h-[320px]"
          description={error.message}
          impact={t(error.access
            ? "state.access_failure_impact"
            : "state.read_failure_impact")}
          nextStep={t(error.access
            ? "state.permission_next_step"
            : "state.retry_next_step")}
          primaryAction={{
            icon: <RotateCcw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: () => setLoadRevision((current) => current + 1),
          }}
          size="md"
          state="error"
          title={t(error.access
            ? "state.permission_title"
            : "capability.loops_loading_failed")}
          variant="plain"
        />
      ) : loop ? (
        <div className="mt-3 space-y-5">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="radius-control-xs border border-(--divider-subtle-color) px-1.5 py-0.5 text-2xs font-medium text-(--text-muted)">
              {loop.category}
            </span>
            <span className="radius-control-xs border border-(--divider-subtle-color) px-1.5 py-0.5 text-2xs text-(--text-soft)">
              {metadata?.triggerLabel}
            </span>
          </div>
          <WorkspaceContentHeader
            className="mb-0"
            description={loop.description}
            title={loop.title}
          />

          <section>
            <h2 className="text-base font-medium text-(--text-strong)">{t("capability.loops_steps")}</h2>
            <div className="mt-2 space-y-2">
              {loop.steps.map((step, index) => (
                <div className="rounded-[8px] border border-(--divider-subtle-color) bg-transparent p-3" key={`${loop.slug}:${step.name}`}>
                  <div className="flex gap-3">
                    <div className="flex h-7 w-7 shrink-0 items-center justify-center radius-control-sm bg-(--surface-interactive-hover-background) text-compact font-semibold text-(--text-muted)">
                      {index + 1}
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-sm font-medium text-(--text-strong)">{step.name}</h3>
                      <p className="mt-0.5 text-compact leading-5 text-(--text-muted)">{step.prompt}</p>
                      {step.shell_check ? (
                        <code className="mt-2 block overflow-x-auto radius-control-sm bg-(--surface-code-background) px-3 py-2 text-compact text-(--text-default)">
                          {step.shell_check}
                        </code>
                      ) : null}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section className="rounded-[8px] border border-(--divider-subtle-color) bg-transparent p-3">
            <h2 className="text-base font-medium text-(--text-strong)">{t("capability.loops_exit")}</h2>
            <p className="mt-1 text-compact leading-5 text-(--text-muted)">{loop.exit_condition.description}</p>
            {loop.exit_condition.command ? (
              <code className="mt-2 block overflow-x-auto radius-control-sm bg-(--surface-code-background) px-3 py-2 text-compact text-(--text-default)">
                {loop.exit_condition.command}
              </code>
            ) : null}
            {loop.exit_condition.max_iterations ? (
              <p className="mt-2 text-compact text-(--text-soft)">
                {t("capability.loops_max_iterations")}: {loop.exit_condition.max_iterations}
              </p>
            ) : null}
          </section>

          {loop.guardrails.length > 0 ? (
            <section>
              <h2 className="text-base font-medium text-(--text-strong)">{t("capability.loops_guardrails")}</h2>
              <ul className="mt-2 space-y-1.5">
                {loop.guardrails.map((item) => (
                  <li className="rounded-[8px] border border-(--divider-subtle-color) bg-transparent px-3 py-2 text-compact leading-5 text-(--text-muted)" key={item}>
                    {item}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <section>
            <div className="mb-3 flex items-center justify-between gap-3">
              <h2 className="text-base font-medium text-(--text-strong)">{t("capability.loops_kickoff_prompt")}</h2>
              <UiButton size="sm" variant="surface" onClick={() => void copyPrompt()}>
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                {t("capability.loops_copy_prompt")}
              </UiButton>
            </div>
            <pre className="soft-scrollbar max-h-[360px] overflow-auto rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-code-background) p-3 text-xs leading-5 text-(--text-default)">
              {loop.kickoff_prompt}
            </pre>
          </section>

          <section>
            <h2 className="text-base font-medium text-(--text-strong)">{t("capability.loops_related")}</h2>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {loop.tags.map((tag) => (
                  <span className="radius-control-xs border border-(--divider-subtle-color) px-1.5 py-0.5 text-xs text-(--text-muted)" key={tag}>
                  {tag}
                </span>
              ))}
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}

/**
 * INPUT: Loop slug、本地化资源与返回动作。
 * OUTPUT: Loop 步骤、退出条件、护栏和启动指令详情。
 * POS: 工作循环长内容详情；目录统计不在此重复堆叠。
 */
"use client";

import { useEffect, useState } from "react";
import { Check, Copy, RotateCcw } from "lucide-react";

import { CapabilityDetailPage } from "@/features/capability/shared/capability-page-layout";
import { getLoopApi } from "@/lib/api/capability/loop-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
    <CapabilityDetailPage
      backLabel={t("capability.loops")}
      currentTitle={loop?.title}
      onBack={onBack}
    >
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
          impact={t(error.access
            ? "state.access_failure_impact"
            : "state.read_failure_impact")}
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
            <UiBadge size="xs">
              {loop.category}
            </UiBadge>
            <UiBadge size="xs" tone="idle">
              {metadata?.triggerLabel}
            </UiBadge>
          </div>
          <WorkspaceContentHeader
            className="mb-0"
            description={loop.description}
            title={loop.title}
          />

          <section>
            <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
              {t("capability.loops_steps")}
            </h2>
            <div className="mt-2 space-y-2">
              {loop.steps.map((step, index) => (
                <UiPanel padding="sm" radius="sm" key={`${loop.slug}:${step.name}`}>
                  <div className="flex gap-3">
                    <div className={cn(
                      "flex h-7 w-7 shrink-0 items-center justify-center radius-control-sm bg-(--surface-interactive-hover-background)",
                      getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "semibold" }),
                    )}>
                      {index + 1}
                    </div>
                    <div className="min-w-0">
                      <h3 className={getUiTypographyClassName({ role: "control", tone: "strong" })}>
                        {step.name}
                      </h3>
                      <p className={cn(
                        "mt-0.5",
                        getUiTypographyClassName({ role: "supporting", tone: "muted" }),
                      )}>
                        {step.prompt}
                      </p>
                      {step.shell_check ? (
                        <code className={cn(
                          "mt-2 block overflow-x-auto radius-control-sm bg-(--surface-code-background) px-3 py-2",
                          getUiTypographyClassName({ role: "code", tone: "default" }),
                        )}>
                          {step.shell_check}
                        </code>
                      ) : null}
                    </div>
                  </div>
                </UiPanel>
              ))}
            </div>
          </section>

          <UiPanel padding="sm" radius="sm">
            <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
              {t("capability.loops_exit")}
            </h2>
            <p className={cn(
              "mt-1",
              getUiTypographyClassName({ role: "supporting", tone: "muted" }),
            )}>
              {loop.exit_condition.description}
            </p>
            {loop.exit_condition.command ? (
              <code className={cn(
                "mt-2 block overflow-x-auto radius-control-sm bg-(--surface-code-background) px-3 py-2",
                getUiTypographyClassName({ role: "code", tone: "default" }),
              )}>
                {loop.exit_condition.command}
              </code>
            ) : null}
            {loop.exit_condition.max_iterations ? (
              <p className={cn(
                "mt-2",
                getUiTypographyClassName({ role: "metadata", tone: "soft" }),
              )}>
                {t("capability.loops_max_iterations")}: {loop.exit_condition.max_iterations}
              </p>
            ) : null}
          </UiPanel>

          {loop.guardrails.length > 0 ? (
            <section>
              <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
                {t("capability.loops_guardrails")}
              </h2>
              <ul className="mt-2 space-y-1.5">
                {loop.guardrails.map((item) => (
                  <li className={cn(
                    "surface-radius-sm border border-(--divider-subtle-color) bg-transparent px-3 py-2",
                    getUiTypographyClassName({ role: "supporting", tone: "muted" }),
                  )} key={item}>
                    {item}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <section>
            <div className="mb-3 flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
              <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
                {t("capability.loops_kickoff_prompt")}
              </h2>
              <UiButton className="shrink-0" size="sm" variant="surface" onClick={() => void copyPrompt()}>
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                {t("capability.loops_copy_prompt")}
              </UiButton>
            </div>
            <pre className={cn(
              "soft-scrollbar max-h-[360px] overflow-auto surface-radius-sm border border-(--divider-subtle-color) bg-(--surface-code-background) p-3",
              getUiTypographyClassName({ role: "code", tone: "default" }),
            )}>
              {loop.kickoff_prompt}
            </pre>
          </section>

          <section>
            <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
              {t("capability.loops_related")}
            </h2>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {loop.tags.map((tag) => (
                <UiBadge size="sm" key={tag}>
                  {tag}
                </UiBadge>
              ))}
            </div>
          </section>
        </div>
      ) : null}
    </CapabilityDetailPage>
  );
}

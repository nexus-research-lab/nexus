"use client";

import { useEffect, useState } from "react";
import { ArrowLeft, Check, Copy } from "lucide-react";

import { getLoopApi } from "@/lib/api/loop-api";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button";
import type { LoopCatalogItem } from "@/types/capability/loop";

interface LoopDetailViewProps {
  slug: string;
  onBack: () => void;
}

interface LoopDetailState {
  error: string | null;
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
  const { error, loading, loop } = state;

  useEffect(() => {
    let cancelled = false;
    getLoopApi(slug, locale)
      .then((item) => {
        if (!cancelled) {
          setState({ error: null, loading: false, loop: item });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            error: err instanceof Error ? err.message : t("capability.loops_loading_failed"),
            loading: false,
            loop: null,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [locale, setState, slug, t]);

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
    <div className="mx-auto w-full max-w-[960px] px-4 py-5 sm:px-6 lg:px-8">
      <UiButton size="sm" variant="text" onClick={onBack}>
        <ArrowLeft className="h-4 w-4" />
        {t("common.back")}
      </UiButton>

      {loading ? (
        <div className="py-10 text-[13px] text-(--text-muted)">{t("capability.connectors_loading")}</div>
      ) : error ? (
        <div className="py-10 text-[13px] text-(--destructive)">{error}</div>
      ) : loop ? (
        <div className="mt-4 space-y-6">
          <header className="border-b border-(--divider-subtle-color) pb-5">
            <div className="mb-3 flex flex-wrap items-center gap-1.5">
              <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] font-medium text-(--text-muted)">
                {loop.category}
              </span>
              <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
                {loop.trigger_type}
              </span>
              <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
                {loop.views.toLocaleString()} views
              </span>
              <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
                {loop.installs.toLocaleString()} installs
              </span>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <h1 className="text-[28px] font-semibold tracking-[-0.04em] text-(--text-strong)">
                  {loop.title}
                </h1>
                <p className="mt-2 max-w-[760px] text-[14px] leading-6 text-(--text-muted)">
                  {loop.description}
                </p>
              </div>
              <UiButton className="shrink-0" size="sm" onClick={() => void copyPrompt()}>
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                {t("capability.loops_copy_prompt")}
              </UiButton>
            </div>
          </header>

          <section>
            <h2 className="text-[18px] font-semibold text-(--text-strong)">{t("capability.loops_steps")}</h2>
            <div className="mt-3 space-y-3">
              {loop.steps.map((step, index) => (
                <div className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-4" key={`${loop.slug}:${step.name}`}>
                  <div className="flex gap-3">
                    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] bg-(--surface-interactive-hover-background) text-[12px] font-semibold text-(--text-muted)">
                      {index + 1}
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-[14px] font-semibold text-(--text-strong)">{step.name}</h3>
                      <p className="mt-1 text-[13px] leading-5 text-(--text-muted)">{step.prompt}</p>
                      {step.shell_check ? (
                        <code className="mt-2 block overflow-x-auto rounded-[7px] bg-(--surface-code-background) px-3 py-2 text-[12px] text-(--text-default)">
                          {step.shell_check}
                        </code>
                      ) : null}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-4">
            <h2 className="text-[18px] font-semibold text-(--text-strong)">{t("capability.loops_exit")}</h2>
            <p className="mt-2 text-[13px] leading-5 text-(--text-muted)">{loop.exit_condition.description}</p>
            {loop.exit_condition.command ? (
              <code className="mt-2 block overflow-x-auto rounded-[7px] bg-(--surface-code-background) px-3 py-2 text-[12px] text-(--text-default)">
                {loop.exit_condition.command}
              </code>
            ) : null}
            {loop.exit_condition.max_iterations ? (
              <p className="mt-2 text-[12px] text-(--text-soft)">
                {t("capability.loops_max_iterations")}: {loop.exit_condition.max_iterations}
              </p>
            ) : null}
          </section>

          {loop.guardrails.length > 0 ? (
            <section>
              <h2 className="text-[18px] font-semibold text-(--text-strong)">{t("capability.loops_guardrails")}</h2>
              <ul className="mt-3 space-y-2">
                {loop.guardrails.map((item) => (
                  <li className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) px-4 py-3 text-[13px] leading-5 text-(--text-muted)" key={item}>
                    {item}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <section>
            <div className="mb-3 flex items-center justify-between gap-3">
              <h2 className="text-[18px] font-semibold text-(--text-strong)">{t("capability.loops_kickoff_prompt")}</h2>
              <UiButton size="sm" variant="surface" onClick={() => void copyPrompt()}>
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                {t("capability.loops_copy_prompt")}
              </UiButton>
            </div>
            <pre className="soft-scrollbar max-h-[360px] overflow-auto rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-code-background) p-4 text-[12px] leading-5 text-(--text-default)">
              {loop.kickoff_prompt}
            </pre>
          </section>

          <section>
            <h2 className="text-[18px] font-semibold text-(--text-strong)">{t("capability.loops_related")}</h2>
            <div className="mt-3 flex flex-wrap gap-2">
              {loop.tags.map((tag) => (
                <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-1 text-[12px] text-(--text-muted)" key={tag}>
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

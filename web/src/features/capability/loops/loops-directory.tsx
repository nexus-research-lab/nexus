"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Copy, Repeat2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
import { list_loops_api } from "@/lib/api/loop-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { LoopCatalogItem } from "@/types/capability/loop";
import { write_text_to_clipboard } from "@/hooks/ui/clipboard";

import { LoopDetailView } from "./loop-detail-view";

const ALL_CATEGORIES = "__all__";

function matches_loop(loop: LoopCatalogItem, query: string): boolean {
  if (!query) {
    return true;
  }
  const haystack = [
    loop.title,
    loop.description,
    loop.category,
    loop.trigger_type,
    ...loop.tags,
    ...loop.compatible_agents,
  ].join(" ").toLowerCase();
  return haystack.includes(query);
}

export function LoopsDirectory() {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const { slug } = useParams<{ slug?: string }>();
  const [loops, set_loops] = useState<LoopCatalogItem[]>([]);
  const [query, set_query] = useState("");
  const [category, set_category] = useState(ALL_CATEGORIES);
  const [loading, set_loading] = useState(true);
  const [error, set_error] = useState<string | null>(null);
  const [copied_slug, set_copied_slug] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    set_loading(true);
    set_error(null);
    list_loops_api(locale)
      .then((items) => {
        if (!cancelled) {
          set_loops(items);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          set_error(err instanceof Error ? err.message : t("capability.loops_loading_failed"));
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [locale, t]);

  const category_options = useMemo(() => {
    const categories = Array.from(new Set(loops.map((loop) => loop.category))).sort();
    return [
      { value: ALL_CATEGORIES, label: t("capability.category_all") },
      ...categories.map((item) => ({ value: item, label: item })),
    ];
  }, [loops, t]);

  const filtered_loops = useMemo(() => {
    const normalized_query = query.trim().toLowerCase();
    return loops.filter((loop) =>
      (category === ALL_CATEGORIES || loop.category === category) &&
      matches_loop(loop, normalized_query),
    );
  }, [category, loops, query]);

  const copy_prompt = async (loop: LoopCatalogItem) => {
    await write_text_to_clipboard(loop.kickoff_prompt);
    set_copied_slug(loop.slug);
    window.setTimeout(() => set_copied_slug((current) => current === loop.slug ? null : current), 1800);
  };

  if (slug) {
    return (
      <LoopDetailView
        slug={slug}
        on_back={() => navigate(AppRouteBuilders.loops())}
      />
    );
  }

  return (
    <WorkspaceSurfaceScaffold body_scrollable stable_gutter>
      <CapabilityPageLayout
        description={t("capability.loops_intro_description")}
        title={t("capability.loops_intro_title")}
      >
        <CapabilityFilterBar>
          <CapabilityFilterSearchInput
            on_change={set_query}
            placeholder={t("capability.loops_search_placeholder")}
            value={query}
          />
          <CapabilityFilterSelect
            aria_label={t("capability.loops_filter_aria")}
            on_change={set_category}
            options={category_options}
            value={category}
          />
        </CapabilityFilterBar>

        <CapabilitySectionHeader
          count={t("capability.loops_badge", { count: filtered_loops.length })}
          title={t("capability.loops")}
        />

        {loading ? (
          <div className="py-10 text-[13px] text-(--text-muted)">{t("capability.connectors_loading")}</div>
        ) : error ? (
          <div className="py-10 text-[13px] text-(--destructive)">{error}</div>
        ) : filtered_loops.length === 0 ? (
          <div className="py-10 text-[13px] text-(--text-muted)">{t("capability.loops_empty")}</div>
        ) : (
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
            {filtered_loops.map((loop) => (
              <div
                className="cursor-pointer rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-4 transition-colors hover:bg-(--surface-interactive-hover-background)"
                key={loop.slug}
                onClick={() => navigate(AppRouteBuilders.loop_detail(loop.slug))}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    navigate(AppRouteBuilders.loop_detail(loop.slug));
                  }
                }}
                role="button"
                tabIndex={0}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="mb-2 flex flex-wrap items-center gap-1.5">
                      <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] font-medium text-(--text-muted)">
                        {loop.category}
                      </span>
                      <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
                        {loop.trigger_type}
                      </span>
                    </div>
                    <h3 className="text-[15px] font-semibold text-(--text-strong)">{loop.title}</h3>
                    <p className="mt-1 line-clamp-2 text-[13px] leading-5 text-(--text-muted)">
                      {loop.description}
                    </p>
                  </div>
                  <UiIconButton
                    aria-label={t("capability.loops_copy_prompt")}
                    class_name="shrink-0"
                    onClick={(event) => {
                      event.stopPropagation();
                      void copy_prompt(loop);
                    }}
                    size="md"
                    variant="ghost"
                  >
                    {copied_slug === loop.slug ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </UiIconButton>
                </div>

                <div className="mt-3 space-y-2">
                  {loop.steps.slice(0, 3).map((step) => (
                    <div className="flex gap-2 text-[12px] leading-5 text-(--text-muted)" key={`${loop.slug}:${step.name}`}>
                      <Repeat2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                      <span className="min-w-0">
                        <span className="font-medium text-(--text-default)">{step.name}</span>
                        <span>：{step.prompt}</span>
                      </span>
                    </div>
                  ))}
                </div>

                <p className="mt-3 border-t border-(--divider-subtle-color) pt-3 text-[12px] leading-5 text-(--text-soft)">
                  {t("capability.loops_exit")}: {loop.exit_condition.description}
                </p>
              </div>
            ))}
          </div>
        )}
      </CapabilityPageLayout>
    </WorkspaceSurfaceScaffold>
  );
}

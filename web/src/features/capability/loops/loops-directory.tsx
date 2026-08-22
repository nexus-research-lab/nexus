/**
 * INPUT: Loop 目录、筛选、复制动作与可选详情路由。
 * OUTPUT: 展示用途、触发方式与步骤规模的工作循环目录或当前 Loop 详情。
 * POS: “能力 > 工作循环”的唯一页面入口。
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Copy, Repeat2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import { listLoopsApi } from "@/lib/api/capability/loop-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiListRow } from "@/shared/ui/list/list-row";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { LoopCatalogItem } from "@/types/capability/loop";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";

import { LoopDetailView } from "./loop-detail-view";
import { getLoopTriggerLabel } from "./loop-presentation";

const ALL_CATEGORIES = "__all__";

function matchesLoop(loop: LoopCatalogItem, query: string): boolean {
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
  const [loops, setLoops] = useState<LoopCatalogItem[]>([]);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState(ALL_CATEGORIES);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copiedSlug, setCopiedSlug] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    listLoopsApi(locale)
      .then((items) => {
        if (!cancelled) {
          setLoops(items);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : t("capability.loops_loading_failed"));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [locale, t]);

  const categoryOptions = useMemo(() => {
    const categories = Array.from(new Set(loops.map((loop) => loop.category))).sort();
    return [
      { value: ALL_CATEGORIES, label: t("capability.category_all") },
      ...categories.map((item) => ({ value: item, label: item })),
    ];
  }, [loops, t]);

  const filteredLoops = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return loops.filter((loop) =>
      (category === ALL_CATEGORIES || loop.category === category) &&
      matchesLoop(loop, normalizedQuery),
    );
  }, [category, loops, query]);

  const copyPrompt = async (loop: LoopCatalogItem) => {
    await writeTextToClipboard(loop.kickoff_prompt);
    setCopiedSlug(loop.slug);
    window.setTimeout(() => setCopiedSlug((current) => current === loop.slug ? null : current), 1800);
  };

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      stableGutter
    >
      {slug ? (
        <LoopDetailView
          slug={slug}
          onBack={() => navigate(AppRouteBuilders.loops())}
        />
      ) : (
        <CapabilityPageLayout
          description={t("capability.loops_intro_description")}
          title={t("capability.loops_intro_title")}
        >
          <CapabilityFilterBar>
            <CapabilityFilterSearchInput
              onChange={setQuery}
              placeholder={t("capability.loops_search_placeholder")}
              value={query}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.loops_filter_aria")}
              label={t("capability.category_label")}
              onChange={setCategory}
              options={categoryOptions}
              value={category}
            />
          </CapabilityFilterBar>

          {loading ? (
            <div className="py-10 text-sm text-(--text-muted)">
              {t("capability.loops_loading")}
            </div>
          ) : error ? (
            <div className="py-10 text-sm text-(--destructive)">{error}</div>
          ) : filteredLoops.length === 0 ? (
            <UiStateBlock
              className="min-h-48"
              description={t("capability.loops_empty_description")}
              icon={<Repeat2 className="h-5 w-5 text-(--icon-default)" />}
              size="sm"
              title={t("capability.loops_empty")}
            />
          ) : (
            <div className={CAPABILITY_DIRECTORY_GRID_CLASS_NAME}>
              {filteredLoops.map((loop) => (
                <UiListRow
                  className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
                  key={loop.slug}
                  onClick={() => navigate(AppRouteBuilders.loopDetail(loop.slug))}
                  leading={(
                    <UiSeededAvatar seed={loop.slug} size="sm" />
                  )}
                  right={(
                    <UiIconButton
                      aria-label={t("capability.loops_copy_prompt")}
                      className="shrink-0"
                      onClick={(event) => {
                        event.stopPropagation();
                        void copyPrompt(loop);
                      }}
                      size="md"
                      variant="ghost"
                    >
                      {copiedSlug === loop.slug
                        ? <Check className="h-4 w-4" />
                        : <Copy className="h-4 w-4" />}
                    </UiIconButton>
                  )}
                >
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-base font-medium text-(--text-strong)">
                      {loop.title}
                    </h3>
                    <p className="mt-0.5 truncate text-compact leading-[1.125rem] text-(--text-muted)">
                      {loop.description}
                    </p>
                    <div className="mt-0.5 flex min-w-0 items-center gap-1.5 overflow-hidden text-2xs leading-4 text-(--text-soft)">
                      <span className="truncate">{loop.category}</span>
                      <span aria-hidden="true">·</span>
                      <span className="shrink-0">
                        {getLoopTriggerLabel(loop.trigger_type, t)}
                      </span>
                      <span aria-hidden="true">·</span>
                      <span className="shrink-0">
                        {t("capability.loops_step_count", {
                          count: loop.steps.length,
                        })}
                      </span>
                    </div>
                  </div>
                </UiListRow>
              ))}
            </div>
          )}
        </CapabilityPageLayout>
      )}
    </WorkspaceSurfaceScaffold>
  );
}

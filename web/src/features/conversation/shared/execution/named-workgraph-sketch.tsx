/**
 * INPUT: 后台模型抽取或已保存的抽象 WorkGraph 节点与依赖。
 * OUTPUT: 不含运行身份和工具事实的只读蓝图式结构草图。
 * POS: 草图确认、Composer 复用目录与能力详情共用的唯一结构预览。
 */
"use client";

import { ArrowRight, CheckCircle2, GitBranch } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type {
  WorkGraphWorkflowDependency,
  WorkGraphWorkflowNode,
} from "@/types/conversation/workgraph-workflow";

interface SketchLayer {
  depth: number;
  nodes: WorkGraphWorkflowNode[];
}

export function NamedWorkGraphSketch({
  className,
  dependencies = [],
  nodes,
}: {
  className?: string;
  dependencies?: readonly WorkGraphWorkflowDependency[];
  nodes: readonly WorkGraphWorkflowNode[];
}) {
  const { t } = useI18n();
  const layers = buildSketchLayers(nodes, dependencies);

  return (
    <div
      aria-label={t("execution.workflow_sketch_label")}
      className={cn(
        "soft-scrollbar overflow-x-auto rounded-[12px] border border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--surface-muted-background)_88%,transparent)] p-4",
        className,
      )}
      data-workgraph-sketch
    >
      <div className="flex min-w-max items-stretch gap-2">
        {layers.map((layer, layerIndex) => (
          <div className="flex items-center gap-2" key={layer.depth}>
            {layerIndex > 0 ? (
              <div className="flex w-7 shrink-0 items-center" aria-hidden="true">
                <span className="h-px flex-1 border-t border-dashed border-[color:color-mix(in_srgb,var(--primary)_45%,transparent)]" />
                <ArrowRight className="-ml-1 h-3.5 w-3.5 text-(--primary)" />
              </div>
            ) : null}
            <div className="flex w-52 flex-col justify-center gap-2" data-workgraph-sketch-layer={layer.depth}>
              {layer.nodes.map((node) => {
                const upstream = dependencies
                  .filter((edge) => edge.logical_key === node.logical_key)
                  .map((edge) => edge.depends_on_logical_key);
                return (
                  <article
                    className="relative rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-2.5 shadow-[0_1px_0_color-mix(in_srgb,var(--text-strong)_4%,transparent)]"
                    data-workgraph-sketch-node={node.logical_key}
                    key={node.logical_key}
                  >
                    <div className="flex items-start gap-2">
                      <span className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-[6px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)">
                        {node.terminal
                          ? <CheckCircle2 className="h-3 w-3" />
                          : <GitBranch className="h-3 w-3" />}
                      </span>
                      <div className="min-w-0">
                        <h4 className="text-xs font-semibold leading-4 text-(--text-strong)">{node.subject}</h4>
                        <p className="mt-1 line-clamp-3 text-[11px] leading-4 text-(--text-muted)">{node.objective}</p>
                      </div>
                    </div>
                    {node.terminal ? (
                      <div className="mt-2 border-t border-dashed border-(--divider-subtle-color) pt-1.5 text-right text-[9px] uppercase tracking-[0.08em] text-(--text-soft)">
                        {t("execution.workflow_terminal_short")}
                      </div>
                    ) : null}
                    {upstream.length > 0 ? (
                      <div className="sr-only">{upstream.join(", ")}</div>
                    ) : null}
                  </article>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function buildSketchLayers(
  nodes: readonly WorkGraphWorkflowNode[],
  dependencies: readonly WorkGraphWorkflowDependency[],
): SketchLayer[] {
  const nodeByKey = new Map(nodes.map((node) => [node.logical_key, node]));
  const parentsByKey = new Map<string, string[]>();
  nodes.forEach((node) => parentsByKey.set(node.logical_key, []));
  dependencies.forEach((edge) => {
    if (nodeByKey.has(edge.logical_key) && nodeByKey.has(edge.depends_on_logical_key)) {
      parentsByKey.get(edge.logical_key)?.push(edge.depends_on_logical_key);
    }
  });
  nodes.forEach((node) => {
    if (node.parent_logical_key && nodeByKey.has(node.parent_logical_key)) {
      parentsByKey.get(node.logical_key)?.push(node.parent_logical_key);
    }
  });

  const depthByKey = new Map<string, number>();
  const resolveDepth = (key: string, visiting: Set<string>): number => {
    const cached = depthByKey.get(key);
    if (cached !== undefined) return cached;
    if (visiting.has(key)) return 0;
    const nextVisiting = new Set(visiting).add(key);
    const parents = parentsByKey.get(key) ?? [];
    const depth = parents.length === 0
      ? 0
      : Math.max(...parents.map((parent) => resolveDepth(parent, nextVisiting))) + 1;
    depthByKey.set(key, depth);
    return depth;
  };

  const grouped = new Map<number, WorkGraphWorkflowNode[]>();
  [...nodes]
    .sort((left, right) => left.position - right.position)
    .forEach((node) => {
      const depth = resolveDepth(node.logical_key, new Set());
      grouped.set(depth, [...(grouped.get(depth) ?? []), node]);
    });
  return [...grouped.entries()]
    .sort(([left], [right]) => left - right)
    .map(([depth, layerNodes]) => ({ depth, nodes: layerNodes }));
}

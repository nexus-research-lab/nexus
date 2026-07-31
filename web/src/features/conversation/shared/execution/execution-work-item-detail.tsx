/**
 * INPUT: 选中 Work Item、完整 Execution 与 Agent 目录。
 * OUTPUT: 交付契约、依赖、责任、Attempt/子智能体、Submission、证据和 Review 详情。
 * POS: Execution 展开面板右侧验收流。
 */
"use client";

import {
  AlertTriangle,
  Bot,
  Check,
  Circle,
  FileCheck2,
  Flag,
  GitBranch,
  Link2,
  PackageCheck,
  ShieldCheck,
  Target,
  UserRound,
} from "lucide-react";
import type { ReactNode } from "react";

import type { TranslationKey } from "@/shared/i18n/messages";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import type {
  ExecutionAttemptView,
  ExecutionView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import {
  resolveExecutionAgent,
  type ExecutionAgentDirectory,
  WORK_ITEM_KIND_LABEL_KEY,
  WORK_ITEM_STATUS_LABEL_KEY,
} from "./execution-process-model";

const ATTEMPT_STATUS_LABEL_KEY: Record<
  ExecutionAttemptView["status"],
  TranslationKey
> = {
  pending: "execution.attempt_pending",
  running: "execution.attempt_running",
  succeeded: "execution.attempt_succeeded",
  failed: "execution.attempt_failed",
  interrupted: "execution.attempt_interrupted",
  cancelled: "execution.attempt_cancelled",
  timed_out: "execution.attempt_timed_out",
};

export function ExecutionWorkItemDetail({
  directory,
  execution,
  item,
}: {
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  item: ExecutionWorkItemView;
}) {
  const { t } = useI18n();
  const owner = resolveExecutionAgent(directory, item.owner_agent_id);
  const itemById = new Map(
    (execution.work_items ?? []).map((candidate) => [candidate.id, candidate]),
  );
  const dependencies = (item.dependency_ids ?? [])
    .map((id) => itemById.get(id))
    .filter((value): value is ExecutionWorkItemView => Boolean(value));

  return (
    <article className="soft-scrollbar min-h-0 overflow-y-auto px-3 pb-4 pt-3 md:px-4">
      <header className="mb-3">
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="rounded-full border border-(--surface-control-border) bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[10px] font-medium text-(--text-muted)">
                {t(WORK_ITEM_KIND_LABEL_KEY[item.kind])}
              </span>
              <WorkItemStatusBadge item={item} />
              {item.required ? (
                <span className="text-[10px] text-(--text-soft)">
                  {t("execution.required")}
                </span>
              ) : null}
            </div>
            <h3 className="mt-1.5 text-sm font-semibold leading-5 text-(--text-strong)">
              {item.subject}
            </h3>
            <p className="mt-0.5 font-mono text-[10px] text-(--text-soft)">
              {item.logical_key}
            </p>
          </div>
          {owner ? (
            <span
              className="flex max-w-[10rem] shrink-0 items-center gap-1.5"
              title={owner.name}
            >
              <UiAgentAvatar
                avatar={owner.avatar}
                name={owner.name}
                size="xs"
              />
              <span className="truncate text-xs font-medium text-(--text-default)">
                {owner.name}
              </span>
            </span>
          ) : (
            <span className="text-xs text-(--text-soft)">
              {t("execution.owner_unassigned")}
            </span>
          )}
        </div>
      </header>

      {item.block_reason || item.needed_input ? (
        <div className="mb-3 rounded-[10px] border border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-3 py-2 text-xs leading-5 text-(--warning)">
          <div className="flex items-center gap-1.5 font-semibold">
            <AlertTriangle className="h-3.5 w-3.5" />
            {t("execution.blocker")}
          </div>
          {item.block_reason ? <p className="mt-1">{item.block_reason}</p> : null}
          {item.needed_input ? (
            <p className="mt-1">
              <span className="font-semibold">{t("execution.needed_input")}：</span>
              {item.needed_input}
            </p>
          ) : null}
        </div>
      ) : null}

      <DetailSection
        icon={<Target className="h-3.5 w-3.5" />}
        title={t("execution.contract")}
      >
        <DetailField label={t("execution.objective")} value={item.objective} />
        <DetailField label={t("execution.deliverable")} value={item.deliverable} />
      </DetailSection>

      <DetailSection
        icon={<GitBranch className="h-3.5 w-3.5" />}
        title={t("execution.dependencies")}
      >
        {dependencies.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {dependencies.map((dependency) => (
              <span
                className="inline-flex max-w-full items-center gap-1 rounded-full border border-(--surface-control-border) bg-(--surface-interactive-hover-background) px-2 py-1 text-xs text-(--text-muted)"
                key={dependency.id}
              >
                <span
                  className={cn(
                    "h-1.5 w-1.5 shrink-0 rounded-full",
                    dependency.status === "accepted"
                      ? "bg-(--success)"
                      : dependency.status === "running"
                        ? "bg-(--primary)"
                        : "bg-(--icon-muted)",
                  )}
                />
                <span className="truncate">{dependency.subject}</span>
              </span>
            ))}
          </div>
        ) : (
          <p className="text-xs text-(--text-soft)">
            {t("execution.no_dependencies")}
          </p>
        )}
      </DetailSection>

      <AcceptanceCriteria item={item} />
      <WorkInputsAndOutputs item={item} />
      <AttemptTimeline directory={directory} item={item} />
      <SubmissionAndReview directory={directory} item={item} />
      <ExecutionCompletionBoundary execution={execution} />
    </article>
  );
}

function ExecutionCompletionBoundary({
  execution,
}: {
  execution: ExecutionView;
}) {
  const { t } = useI18n();
  const criteria = execution.completion_criteria ?? [];
  const blockers = execution.completion_blockers ?? [];
  if (criteria.length === 0 && blockers.length === 0) {
    return null;
  }
  return (
    <>
      {criteria.length > 0 ? (
        <DetailSection
          icon={<Flag className="h-3.5 w-3.5" />}
          title={t("execution.completion_criteria")}
        >
          <ul className="space-y-1.5">
            {criteria.map((criterion) => (
              <li className="flex items-start gap-2 text-xs leading-5" key={criterion}>
                <Circle className="mt-1 h-2.5 w-2.5 shrink-0 text-(--icon-muted)" />
                <span className="text-(--text-default)">{criterion}</span>
              </li>
            ))}
          </ul>
        </DetailSection>
      ) : null}
      {blockers.length > 0 ? (
        <DetailSection
          icon={<AlertTriangle className="h-3.5 w-3.5" />}
          title={t("execution.completion_blockers")}
        >
          <ul className="space-y-1 text-xs leading-5 text-(--warning)">
            {blockers.map((blocker) => <li key={blocker}>{blocker}</li>)}
          </ul>
        </DetailSection>
      ) : null}
    </>
  );
}

function WorkItemStatusBadge({ item }: { item: ExecutionWorkItemView }) {
  const { t } = useI18n();
  const warning = item.status === "blocked"
    || item.status === "failed"
    || item.status === "changes_requested";
  const success = item.status === "accepted";
  return (
    <span
      className={cn(
        "rounded-full border px-2 py-0.5 text-[10px] font-semibold",
        success
          ? "border-[color:color-mix(in_srgb,var(--success)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_9%,transparent)] text-(--success)"
          : warning
            ? "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_9%,transparent)] text-(--warning)"
            : "border-[color:color-mix(in_srgb,var(--primary)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)] text-(--primary)",
      )}
    >
      {t(WORK_ITEM_STATUS_LABEL_KEY[item.status])}
    </span>
  );
}

function AcceptanceCriteria({ item }: { item: ExecutionWorkItemView }) {
  const { t } = useI18n();
  const criteria = item.acceptance_criteria ?? [];
  if (criteria.length === 0) {
    return null;
  }
  const resultByCriterion = new Map(
    (item.acceptance?.criteria_results ?? [])
      .map((result) => [result.criterion, result]),
  );
  return (
    <DetailSection
      icon={<ShieldCheck className="h-3.5 w-3.5" />}
      title={t("execution.acceptance")}
    >
      <ul className="space-y-1.5">
        {criteria.map((criterion) => {
          const result = resultByCriterion.get(criterion);
          return (
            <li className="flex items-start gap-2 text-xs leading-5" key={criterion}>
              <span className="mt-1 grid h-3.5 w-3.5 shrink-0 place-items-center">
                {result?.passed ? (
                  <Check className="h-3.5 w-3.5 text-(--success)" />
                ) : (
                  <Circle className="h-2.5 w-2.5 text-(--icon-muted)" />
                )}
              </span>
              <span className="min-w-0 text-(--text-default)">
                {criterion}
                {result?.note ? (
                  <span className="mt-0.5 block text-(--text-soft)">
                    {result.note}
                  </span>
                ) : null}
              </span>
            </li>
          );
        })}
      </ul>
    </DetailSection>
  );
}

function WorkInputsAndOutputs({ item }: { item: ExecutionWorkItemView }) {
  const { t } = useI18n();
  const inputs = item.input_refs ?? [];
  const outputs = item.output_scopes ?? [];
  if (inputs.length === 0 && outputs.length === 0) {
    return null;
  }
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      {inputs.length > 0 ? (
        <DetailSection
          icon={<Link2 className="h-3.5 w-3.5" />}
          title={t("execution.inputs")}
        >
          <ReferenceList values={inputs} />
        </DetailSection>
      ) : null}
      {outputs.length > 0 ? (
        <DetailSection
          icon={<PackageCheck className="h-3.5 w-3.5" />}
          title={t("execution.outputs")}
        >
          <ul className="space-y-1">
            {outputs.map((output) => (
              <li
                className="flex min-w-0 items-center justify-between gap-2 text-xs"
                key={`${output.mode}:${output.scope}`}
              >
                <span className="min-w-0 truncate font-mono text-[11px] text-(--text-default)">
                  {output.scope}
                </span>
                <span className="shrink-0 text-[10px] text-(--text-soft)">
                  {t(
                    output.mode === "exclusive"
                      ? "execution.output_exclusive"
                      : "execution.output_shared",
                  )}
                </span>
              </li>
            ))}
          </ul>
        </DetailSection>
      ) : null}
    </div>
  );
}

function AttemptTimeline({
  directory,
  item,
}: {
  directory: ExecutionAgentDirectory;
  item: ExecutionWorkItemView;
}) {
  const { t } = useI18n();
  const attempts = item.attempts ?? [];
  if (attempts.length === 0) {
    return null;
  }
  return (
    <DetailSection
      icon={<UserRound className="h-3.5 w-3.5" />}
      title={t("execution.attempts")}
    >
      <ol className="space-y-1.5">
        {attempts.map((attempt) => {
          const executor = resolveExecutionAgent(
            directory,
            attempt.executor_agent_id,
          );
          return (
            <li
              className="flex items-start gap-2 rounded-[8px] bg-(--surface-interactive-hover-background) px-2.5 py-2"
              key={attempt.id}
            >
              <span className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center text-(--icon-muted)">
                {attempt.executor_kind === "subagent"
                  ? <Bot className="h-4 w-4" />
                  : <UserRound className="h-4 w-4" />}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-center gap-1.5 text-xs">
                  <span className="font-medium text-(--text-default)">
                    {attempt.executor_kind === "subagent"
                      ? t("execution.attempt_subagent")
                      : t("execution.attempt_agent")}
                  </span>
                  {executor ? (
                    <span className="text-(--text-soft)">· {executor.name}</span>
                  ) : null}
                  <span className="text-(--text-soft)">
                    · {t(ATTEMPT_STATUS_LABEL_KEY[attempt.status])}
                  </span>
                </span>
                {attempt.failure_reason ? (
                  <span className="mt-0.5 block text-xs leading-4.5 text-(--warning)">
                    {attempt.failure_reason}
                  </span>
                ) : null}
              </span>
            </li>
          );
        })}
      </ol>
    </DetailSection>
  );
}

function SubmissionAndReview({
  directory,
  item,
}: {
  directory: ExecutionAgentDirectory;
  item: ExecutionWorkItemView;
}) {
  const { t } = useI18n();
  if (!item.submission && !item.acceptance) {
    return null;
  }
  const submitter = resolveExecutionAgent(
    directory,
    item.submission?.submitter_agent_id,
  );
  const reviewer = resolveExecutionAgent(
    directory,
    item.acceptance?.reviewer_id,
  );
  return (
    <>
      {item.submission ? (
        <DetailSection
          icon={<FileCheck2 className="h-3.5 w-3.5" />}
          title={t("execution.submission")}
        >
          {submitter ? (
            <p className="mb-1 text-[10px] text-(--text-soft)">
              {submitter.name}
            </p>
          ) : null}
          <p className="text-xs leading-5 text-(--text-default)">
            {item.submission.result_summary}
          </p>
          {(item.submission.result_refs?.length ?? 0) > 0 ? (
            <DetailField
              label={t("execution.result_refs")}
              value={<ReferenceList values={item.submission.result_refs ?? []} />}
            />
          ) : null}
          {(item.submission.evidence?.length ?? 0) > 0 ? (
            <DetailField
              label={t("execution.evidence")}
              value={<ReferenceList values={item.submission.evidence ?? []} />}
            />
          ) : null}
        </DetailSection>
      ) : null}
      {item.acceptance ? (
        <DetailSection
          icon={<ShieldCheck className="h-3.5 w-3.5" />}
          title={t("execution.review")}
        >
          <div className="flex items-center gap-2 text-xs">
            <span
              className={cn(
                "font-semibold",
                item.acceptance.decision === "accepted"
                  ? "text-(--success)"
                  : "text-(--warning)",
              )}
            >
              {t(
                item.acceptance.decision === "accepted"
                  ? "execution.review_accepted"
                  : item.acceptance.decision === "rejected"
                    ? "execution.review_rejected"
                    : "execution.review_changes_requested",
              )}
            </span>
            {reviewer ? (
              <span className="text-(--text-soft)">· {reviewer.name}</span>
            ) : null}
          </div>
          {item.acceptance.feedback ? (
            <p className="mt-1 text-xs leading-5 text-(--text-default)">
              {item.acceptance.feedback}
            </p>
          ) : null}
        </DetailSection>
      ) : null}
    </>
  );
}

function DetailSection({
  children,
  icon,
  title,
}: {
  children: ReactNode;
  icon: ReactNode;
  title: string;
}) {
  return (
    <section className="mb-2.5 rounded-[10px] border border-(--surface-control-border) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_84%,transparent)] px-3 py-2.5">
      <h4 className="mb-1.5 flex items-center gap-1.5 text-xs font-semibold text-(--text-muted)">
        <span className="text-(--icon-muted)">{icon}</span>
        {title}
      </h4>
      {children}
    </section>
  );
}

function DetailField({
  label,
  value,
}: {
  label: string;
  value: ReactNode;
}) {
  return (
    <div className="mt-1.5 first:mt-0">
      <p className="text-[10px] font-medium uppercase tracking-[0.06em] text-(--text-soft)">
        {label}
      </p>
      <div className="mt-0.5 text-xs leading-5 text-(--text-default)">
        {value}
      </div>
    </div>
  );
}

function ReferenceList({ values }: { values: string[] }) {
  return (
    <ul className="space-y-1">
      {values.map((value) => (
        <li
          className="break-all font-mono text-[11px] leading-4.5 text-(--text-muted)"
          key={value}
        >
          {value}
        </li>
      ))}
    </ul>
  );
}

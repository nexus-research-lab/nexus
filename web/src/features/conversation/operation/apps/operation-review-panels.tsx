import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  ClipboardList,
  Clock3,
  FileText,
  Globe2,
  LockKeyhole,
  Play,
  RefreshCw,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import type {
  UserQuestion,
  UserQuestionAnswer,
} from "@/types/conversation/interaction/ask-user-question";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import {
  buildOperationInputRows,
  PHASE_LABELS,
  resolveOperationToolProfile,
} from "../operation-tool-catalog";
import { formatOperationTime, safeJsonStringify } from "../operation-preview";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
} from "../operation-types";

interface AskUserQuestionInput {
  questions: UserQuestion[];
}

export function PermissionCheckpointPanel({
  compact = false,
  event,
  evidence: payload_evidence,
  onPermissionResponse,
  snapshot,
}: {
  compact?: boolean;
  event: NexusOperationEvent;
  evidence?: OperationEvidence[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  snapshot: NexusOperationSnapshot | null;
}) {
  const profile = resolveOperationToolProfile(event.tool_name, event.kind, event.surface);
  const rows = buildOperationInputRows(event.input_preview, profile.target_keys, compact ? 4 : 8);
  const evidence = dedupe_evidence([
    ...(payload_evidence ?? []),
    ...(event.evidence ?? []),
    ...(snapshot?.recent_evidence ?? []),
  ]).slice(0, compact ? 4 : 7);
  const lead = event.summary ?? event.target ?? event.title ?? event.tool_name ?? "等待用户确认";
  const request_target = event.target ?? rows[0]?.value ?? event.tool_name ?? "待确认请求";
  const request_id = event.permission_request_id ?? null;
  const is_question = event.permission_interaction_mode === "question";
  const question_input = useMemo(
    () => parse_ask_user_question_input(event.input_preview),
    [event.input_preview],
  );
  const questions = question_input?.questions ?? [];
  const [submitted_decision, set_submitted_decision] = useState<PermissionDecisionPayload["decision"] | null>(null);
  const [question_answers, set_question_answers] = useState<Record<number, string[]>>({});
  const can_send_response = Boolean(request_id && onPermissionResponse);
  const can_allow = can_send_response && !is_question && !submitted_decision;
  const can_deny = can_send_response && !submitted_decision;
  const can_submit_question_answer = Boolean(
    is_question &&
    can_send_response &&
    !submitted_decision &&
    questions.length > 0 &&
    questions.every((_, index) => (question_answers[index] ?? []).length > 0),
  );
  const response_state_label = !request_id
    ? "缺少请求"
    : submitted_decision === "allow"
        ? is_question ? "已提交" : "已允许"
        : submitted_decision === "deny"
          ? "已拒绝"
          : !onPermissionResponse
            ? "等待连接"
            : is_question
              ? "需回答"
              : "待确认";
  const header_state_label = submitted_decision ? response_state_label : PHASE_LABELS[event.phase];

  useEffect(() => {
    set_submitted_decision(null);
    set_question_answers({});
  }, [request_id]);

  const submit_permission_response = (decision: PermissionDecisionPayload["decision"]) => {
    if (!request_id || !onPermissionResponse) {
      return false;
    }
    const submitted = onPermissionResponse({
      request_id,
      decision,
      message: decision === "deny" ? "User denied permission from Operation Stage" : undefined,
    });
    if (submitted) {
      set_submitted_decision(decision);
    }
    return submitted;
  };
  const submit_question_answers = () => {
    if (!request_id || !onPermissionResponse || !can_submit_question_answer) {
      return false;
    }
    const user_answers: UserQuestionAnswer[] = questions.map((_, index) => ({
      question_index: index,
      selected_options: question_answers[index] ?? [],
    }));
    const submitted = onPermissionResponse({
      request_id,
      decision: "allow",
      user_answers,
    });
    if (submitted) {
      set_submitted_decision("allow");
    }
    return submitted;
  };

  return (
    <div className="flex h-full min-h-[320px] min-w-0 max-w-full flex-col overflow-hidden bg-[linear-gradient(180deg,rgba(255,255,255,0.94),rgba(248,250,252,0.88))]">
      <div className="border-b border-(--divider-subtle-color) px-4 py-3">
        <div className="flex min-w-0 items-start justify-between gap-3">
          <div className="flex min-w-0 items-start gap-3">
            <span className="grid h-10 w-10 shrink-0 place-items-center rounded-[13px] bg-[rgba(223,157,46,0.13)] text-[color:var(--warning)] shadow-[inset_0_1px_0_rgba(255,255,255,0.78)]">
              <LockKeyhole className="h-5 w-5" />
            </span>
            <div className="min-w-0">
              <h3 className="mt-1 truncate text-[15px] font-black tracking-[-0.03em] text-(--text-strong)">
                系统设置
              </h3>
              <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-(--text-muted)">
                自动化权限需要确认后才能继续。
              </p>
            </div>
          </div>
          <span className="shrink-0 rounded-full bg-[rgba(223,157,46,0.12)] px-2.5 py-1 text-[10px] font-black text-[color:var(--warning)]">
            {header_state_label}
          </span>
        </div>
      </div>

      <div className="grid min-h-0 min-w-0 flex-1 grid-cols-[170px_minmax(0,1fr)] max-md:grid-cols-1">
        <aside className="border-r border-(--divider-subtle-color) bg-white/46 p-2.5 max-md:border-b max-md:border-r-0">
          <div className="mb-2 flex items-center gap-1.5 rounded-[9px] border border-(--divider-subtle-color) bg-white/70 px-2 py-1.5 text-[10px] text-(--text-soft)">
            <Search className="h-3 w-3 shrink-0" />
            <span className="truncate">搜索设置</span>
          </div>
            {[
              { label: "隐私", Icon: ShieldCheck },
              { label: "自动化", Icon: Settings2 },
              { label: "文件与文件夹", Icon: FileText },
            { label: "网络", Icon: Globe2 },
          ].map((item, index) => (
            <div
              className={cn(
                "mb-1 flex items-center gap-2 rounded-[9px] px-2.5 py-2 text-[11px] font-bold",
                index === 1 ? "bg-white/82 text-(--text-strong)" : "text-(--text-soft)",
              )}
              key={item.label}
            >
              <item.Icon className="h-3.5 w-3.5 shrink-0" />
              <span className="truncate">{item.label}</span>
            </div>
          ))}
        </aside>

        <section className="soft-scrollbar min-h-0 min-w-0 overflow-auto p-4">
          <div className="rounded-[14px] border border-(--divider-subtle-color) bg-white/76">
            <div className="flex items-start justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
              <div className="min-w-0">
                <p className="text-[13px] font-black text-(--text-strong)">自动化权限</p>
                <p className="mt-1 text-[11px] leading-5 text-(--text-muted)">
                  “Nexus”想要控制这个桌面上的应用或文件。
                </p>
              </div>
              <span className="relative h-5 w-9 shrink-0 rounded-full bg-[rgba(47,184,132,0.22)] p-0.5 shadow-[inset_0_1px_2px_rgba(18,28,42,0.10)]">
                <span className="block h-4 w-4 translate-x-4 rounded-full bg-white shadow-[0_2px_6px_rgba(18,28,42,0.20)]" />
              </span>
            </div>
            <div className="px-4 py-3">
              <div className="flex min-w-0 items-center gap-3 rounded-[12px] bg-[rgba(248,250,252,0.82)] px-3 py-2">
                <span className="grid h-9 w-9 shrink-0 place-items-center rounded-[11px] bg-[rgba(91,114,255,0.10)] text-[color:var(--primary)]">
                  <Settings2 className="h-4 w-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12px] font-black text-(--text-strong)">{profile.title}</p>
                  <p className="truncate text-[10px] text-(--text-soft)">{request_target}</p>
                </div>
                <span className="shrink-0 rounded-full bg-[rgba(223,157,46,0.12)] px-2 py-1 text-[10px] font-black text-[color:var(--warning)]">
                  {response_state_label}
                </span>
              </div>
              <div className="mt-3 rounded-[10px] border border-(--divider-subtle-color) bg-white/76 px-3 py-2">
                <p className="text-[10px] font-black uppercase tracking-[0.12em] text-(--text-soft)">请求内容</p>
                <p className="mt-1 break-words font-mono text-[11px] leading-5 text-(--text-strong)">{request_target}</p>
                <p className="mt-1 line-clamp-2 text-[10px] leading-4 text-(--text-muted)">{lead}</p>
              </div>
              {is_question ? (
                questions.length ? (
                  <StageQuestionAnswerList
                    disabled={Boolean(submitted_decision)}
                    onChange={set_question_answers}
                    questions={questions}
                    selections={question_answers}
                  />
                ) : (
                  <div className="mt-3 rounded-[10px] border border-(--divider-subtle-color) bg-white/72 px-3 py-2 text-[10px] leading-4 text-(--text-muted)">
                    舞台没有收到可渲染的问题选项，请回到聊天输入区处理这个请求。
                  </div>
                )
              ) : null}
              <div className="mt-3 flex items-center justify-end gap-2 border-t border-(--divider-subtle-color) pt-3">
                <button
                  className={cn(
                    "h-7 rounded-[7px] border border-(--divider-subtle-color) bg-white/80 px-3 text-[11px] font-semibold text-(--text-strong) shadow-[inset_0_1px_0_rgba(255,255,255,0.82)] transition hover:bg-white",
                    !can_deny && "cursor-not-allowed opacity-45 hover:bg-white/80",
                  )}
                  disabled={!can_deny}
                  onClick={() => submit_permission_response("deny")}
                  type="button"
                >
                  {is_question ? "取消请求" : "拒绝"}
                </button>
                {is_question ? (
                  <button
                    className={cn(
                      "h-7 rounded-[7px] border border-[rgba(91,114,255,0.28)] bg-[rgba(91,114,255,0.92)] px-3 text-[11px] font-semibold text-white shadow-[0_8px_18px_rgba(91,114,255,0.20)] transition hover:bg-[color:var(--primary)]",
                      !can_submit_question_answer && "cursor-not-allowed opacity-45 hover:bg-[rgba(91,114,255,0.92)]",
                    )}
                    disabled={!can_submit_question_answer}
                    onClick={submit_question_answers}
                    type="button"
                  >
                    提交回答
                  </button>
                ) : (
                  <button
                    className={cn(
                      "h-7 rounded-[7px] border border-[rgba(91,114,255,0.28)] bg-[rgba(91,114,255,0.92)] px-3 text-[11px] font-semibold text-white shadow-[0_8px_18px_rgba(91,114,255,0.20)] transition hover:bg-[color:var(--primary)]",
                      !can_allow && "cursor-not-allowed opacity-45 hover:bg-[rgba(91,114,255,0.92)]",
                    )}
                    disabled={!can_allow}
                    onClick={() => submit_permission_response("allow")}
                    type="button"
                  >
                    允许
                  </button>
                )}
              </div>
            </div>
          </div>

          <div className="mt-3 grid grid-cols-3 gap-2 text-[10px] max-sm:grid-cols-1">
            {[
              { label: "暂停点", value: "权限确认", Icon: Clock3 },
              { label: "工具", value: profile.title, Icon: Play },
              { label: "更新", value: formatOperationTime(event.updated_at), Icon: RefreshCw },
            ].map((item) => (
              <div className="min-w-0 rounded-[11px] border border-white/64 bg-white/62 px-2.5 py-2" key={item.label}>
                <div className="flex items-center gap-1.5 text-(--text-soft)">
                  <item.Icon className="h-3.5 w-3.5 shrink-0" />
                  <span className="font-black">{item.label}</span>
                </div>
                <p className="mt-1 truncate font-mono text-[10px] text-(--text-strong)">{item.value}</p>
              </div>
            ))}
          </div>

          <p className="mt-4 text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">请求详情</p>
          <div className="mt-2 space-y-1.5">
            {rows.length ? rows.map((row) => (
              <div className="rounded-[10px] border border-white/62 bg-white/70 px-2.5 py-2 text-[10px]" key={row.key}>
                <p className="font-black text-(--text-strong)">{row.label}</p>
                <p className="mt-0.5 break-words font-mono leading-4 text-(--text-muted)">{row.value}</p>
              </div>
            )) : (
              <div className="rounded-[10px] border border-white/62 bg-white/70 px-2.5 py-2 text-[10px] text-(--text-muted)">
                {event.target ?? event.tool_name ?? "没有更多请求详情"}
              </div>
            )}
          </div>

          <p className="mt-3 text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">最近活动</p>
          <div className="mt-2 space-y-1.5">
            {(evidence.length ? evidence : [{
              type: "permission",
              label: "等待确认",
              value: lead,
            } satisfies OperationEvidence]).map((item, index) => {
              const Icon = icon_for_evidence(item.type);
              return (
                <div
                  className="flex min-w-0 items-start gap-2 rounded-[10px] border border-white/62 bg-white/68 px-2.5 py-2 text-[10px]"
                  key={`${item.type}:${item.label}:${item.value ?? ""}:${index}`}
                >
                  <span className="grid h-5 w-5 shrink-0 place-items-center rounded-[7px] bg-[rgba(223,157,46,0.12)] text-[color:var(--warning)]">
                    <Icon className="h-3 w-3" />
                  </span>
                  <div className="min-w-0">
                    <p className="font-black text-(--text-strong)">{item.label}</p>
                    <p className="mt-0.5 line-clamp-2 break-words text-(--text-muted)">{item.value ?? item.type}</p>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </div>
    </div>
  );
}

function StageQuestionAnswerList({
  disabled,
  onChange,
  questions,
  selections,
}: {
  disabled: boolean;
  onChange: (next: Record<number, string[]>) => void;
  questions: UserQuestion[];
  selections: Record<number, string[]>;
}) {
  const toggle_option = (question: UserQuestion, question_index: number, option_label: string) => {
    if (disabled) {
      return;
    }
    const selected = selections[question_index] ?? [];
    const multi_select = question.multi_select ?? false;
    const next_selected = multi_select
      ? selected.includes(option_label)
        ? selected.filter((item) => item !== option_label)
        : [...selected, option_label]
      : [option_label];
    onChange({
      ...selections,
      [question_index]: next_selected,
    });
  };

  return (
    <div className="mt-3 space-y-2">
      {questions.map((question, question_index) => {
        const selected = selections[question_index] ?? [];
        const multi_select = question.multi_select ?? false;
        return (
          <div
            className="rounded-[11px] border border-(--divider-subtle-color) bg-white/74 px-3 py-2"
            key={`${question_index}:${question.question}`}
          >
            <div className="flex min-w-0 items-start justify-between gap-2">
              <div className="min-w-0">
                {question.header ? (
                  <p className="text-[9px] font-black uppercase tracking-[0.14em] text-[color:var(--primary)]">
                    {question.header}
                  </p>
                ) : null}
                <p className="mt-0.5 break-words text-[11px] font-black leading-4 text-(--text-strong)">
                  {question.question}
                </p>
              </div>
              <span className="shrink-0 rounded-full bg-[rgba(91,114,255,0.08)] px-2 py-1 text-[9px] font-black text-[color:var(--primary)]">
                {multi_select ? "多选" : "单选"}
              </span>
            </div>
            <div className="mt-2 grid gap-1.5">
              {question.options.map((option) => {
                const is_selected = selected.includes(option.label);
                return (
                  <button
                    className={cn(
                      "min-h-8 rounded-[9px] border px-2.5 py-1.5 text-left text-[10px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
                      is_selected
                        ? "border-[rgba(91,114,255,0.32)] bg-[rgba(91,114,255,0.10)] text-(--text-strong)"
                        : "border-white/70 bg-white/64 text-(--text-muted) hover:bg-white",
                      disabled && "cursor-not-allowed opacity-60",
                    )}
                    disabled={disabled}
                    key={option.label}
                    onClick={() => toggle_option(question, question_index, option.label)}
                    type="button"
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      <span className={cn(
                        "grid h-4 w-4 shrink-0 place-items-center rounded-full border text-[8px] font-black",
                        is_selected
                          ? "border-[rgba(91,114,255,0.44)] bg-[rgba(91,114,255,0.92)] text-white"
                          : "border-(--divider-subtle-color) bg-white/70 text-transparent",
                      )}>
                        {multi_select ? "✓" : "•"}
                      </span>
                      <span className="min-w-0">
                        <span className="block break-words font-black">{option.label}</span>
                        {option.description ? (
                          <span className="mt-0.5 block break-words leading-4 text-(--text-soft)">
                            {option.description}
                          </span>
                        ) : null}
                      </span>
                    </span>
                  </button>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function OperationReviewPanel({
  compact = false,
  event,
  evidence: payload_evidence,
  mode,
  snapshot,
}: {
  compact?: boolean;
  event: NexusOperationEvent;
  evidence?: OperationEvidence[];
  mode: "evidence" | "permission";
  snapshot: NexusOperationSnapshot | null;
}) {
  const profile = resolveOperationToolProfile(event.tool_name, event.kind, event.surface);
  const evidence = dedupe_evidence([
    ...(payload_evidence ?? []),
    ...(event.evidence ?? []),
    ...(snapshot?.recent_evidence ?? []),
  ]).slice(0, compact ? 4 : 8);
  const rows = buildOperationInputRows(event.input_preview, profile.target_keys, compact ? 3 : 6);
  const waiting = event.phase === "waiting" || mode === "permission";
  const lead = event.summary ?? event.title ?? event.target ?? event.tool_name ?? "诊断信息";

  return (
    <div className="flex h-full min-h-[260px] min-w-0 max-w-full flex-col overflow-hidden rounded-[13px] border border-(--divider-subtle-color) bg-white/76">
      <div className={cn(
        "border-b border-(--divider-subtle-color) px-3 py-3",
        waiting
          ? "bg-[linear-gradient(135deg,rgba(223,157,46,0.13),rgba(255,255,255,0.76))]"
          : "bg-[linear-gradient(135deg,rgba(91,114,255,0.10),rgba(255,255,255,0.78))]",
      )}>
        <div className="flex min-w-0 items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">
              {waiting ? "安全请求" : "诊断"}
            </p>
            <h3 className="mt-1 truncate text-[14px] font-black tracking-[-0.03em] text-(--text-strong)">
              {waiting ? "等待确认" : "系统日志"}
            </h3>
          </div>
          <span className={cn(
            "shrink-0 rounded-full px-2 py-1 text-[10px] font-black",
            waiting
              ? "bg-[rgba(223,157,46,0.14)] text-[color:var(--warning)]"
              : "bg-[rgba(47,184,132,0.12)] text-[color:var(--success)]",
          )}>
            {PHASE_LABELS[event.phase]}
          </span>
        </div>
        <p className="mt-2 line-clamp-2 text-[11px] leading-4 text-(--text-muted)">{lead}</p>
      </div>

      <div className="grid min-h-0 min-w-0 flex-1 grid-cols-[minmax(0,1fr)_180px] gap-0 max-md:grid-cols-1">
        <div className="soft-scrollbar min-h-0 min-w-0 overflow-auto p-3">
          <div className="space-y-2">
            {(evidence.length ? evidence : [{
              type: waiting ? "permission" : "status",
              label: waiting ? "请求" : "状态",
              value: lead,
            } satisfies OperationEvidence]).map((item, index) => {
              const Icon = icon_for_evidence(item.type);
              return (
                <div
                  className="flex min-w-0 gap-2 rounded-[11px] border border-(--divider-subtle-color) bg-white/70 px-2.5 py-2 text-[11px]"
                  key={`${item.type}:${item.label}:${item.value ?? ""}:${index}`}
                >
                  <span className={cn(
                    "mt-0.5 grid h-6 w-6 shrink-0 place-items-center rounded-[8px]",
                    item.type === "error" && "bg-[rgba(223,93,98,0.10)] text-[color:var(--destructive)]",
                    item.type === "permission" && "bg-[rgba(223,157,46,0.12)] text-[color:var(--warning)]",
                    item.type !== "error" && item.type !== "permission" && "bg-[rgba(91,114,255,0.09)] text-[color:var(--primary)]",
                  )}>
                    <Icon className="h-3.5 w-3.5" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="shrink-0 font-black text-(--text-strong)">{item.label}</span>
                      <span className="min-w-0 flex-1 truncate text-(--text-muted)">{item.value ?? item.type}</span>
                    </div>
                    {item.preview != null ? (
                      <pre className="mt-1 max-h-16 overflow-hidden whitespace-pre-wrap break-words rounded-[8px] bg-[rgba(18,28,42,0.05)] px-2 py-1.5 font-mono text-[10px] leading-4 text-(--text-soft)">
                        {safeJsonStringify(item.preview)}
                      </pre>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <aside className="soft-scrollbar min-h-0 overflow-auto border-l border-(--divider-subtle-color) bg-white/45 p-3 max-md:max-h-[220px] max-md:border-l-0 max-md:border-t">
          <p className="text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">详情</p>
          <div className="mt-2 space-y-1.5">
            {rows.length ? rows.map((row) => (
              <div className="rounded-[9px] bg-white/70 px-2 py-1.5 text-[10px]" key={row.key}>
                <p className="font-black text-(--text-strong)">{row.label}</p>
                <p className="mt-0.5 break-words text-(--text-muted)">{row.value}</p>
              </div>
            )) : (
              <div className="rounded-[9px] bg-white/70 px-2 py-1.5 text-[10px] text-(--text-muted)">
                {event.target ?? event.tool_name ?? "没有更多详情"}
              </div>
            )}
          </div>
          {waiting ? (
            <div className="mt-2 grid grid-cols-2 gap-1.5 text-[10px] font-semibold">
              <span className="rounded-[7px] border border-(--divider-subtle-color) bg-white/80 px-2 py-1.5 text-center text-(--text-strong)">
                拒绝
              </span>
              <span className="rounded-[7px] border border-[rgba(91,114,255,0.26)] bg-[rgba(91,114,255,0.92)] px-2 py-1.5 text-center text-white">
                允许
              </span>
            </div>
          ) : null}
          <div className="mt-2 rounded-[9px] bg-white/70 px-2 py-1.5 text-[10px] text-(--text-muted)">
            更新于 {formatOperationTime(event.updated_at)}
          </div>
        </aside>
      </div>
    </div>
  );
}

function parse_ask_user_question_input(value: unknown): AskUserQuestionInput | null {
  if (!is_record(value) || !Array.isArray(value.questions)) {
    return null;
  }
  const questions = value.questions.flatMap((item) => {
    const question = parse_user_question(item);
    return question ? [question] : [];
  });
  return questions.length ? { questions } : null;
}

function parse_user_question(value: unknown): UserQuestion | null {
  if (!is_record(value) || typeof value.question !== "string") {
    return null;
  }
  const options_source = Array.isArray(value.options) ? value.options : [];
  const options = options_source.flatMap((item) => {
    if (!is_record(item) || typeof item.label !== "string") {
      return [];
    }
    return [{
      label: item.label,
      description: typeof item.description === "string" ? item.description : undefined,
    }];
  });
  if (!options.length) {
    return null;
  }
  return {
    question: value.question,
    header: typeof value.header === "string" ? value.header : undefined,
    multi_select: typeof value.multi_select === "boolean"
      ? value.multi_select
      : typeof value.multiSelect === "boolean"
        ? value.multiSelect
        : undefined,
    options,
  };
}

function is_record(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

function dedupe_evidence(items: OperationEvidence[]): OperationEvidence[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = `${item.type}:${item.label}:${item.value ?? ""}`;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function icon_for_evidence(type: OperationEvidence["type"]): LucideIcon {
  if (type === "file" || type === "diff") {
    return FileText;
  }
  if (type === "terminal") {
    return Play;
  }
  if (type === "url") {
    return Globe2;
  }
  if (type === "task") {
    return ClipboardList;
  }
  if (type === "permission") {
    return CircleHelp;
  }
  if (type === "error") {
    return AlertTriangle;
  }
  if (type === "skill") {
    return Sparkles;
  }
  return CheckCircle2;
}

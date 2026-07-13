import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";

export type OperationStageExperiencePhase =
  | "idle"
  | "awakening"
  | "running"
  | "settling"
  | "completed";

const MAX_MERGED_EVENTS = 24;
const MAX_MERGED_WORKSPACE_EVENTS = 8;
const MAX_MERGED_EVIDENCE = 8;

export interface OperationContinuationBrief {
  status_label: string;
  status_detail: string;
  resume_prompt: string;
  primary_artifact: string;
  checkpoints: Array<{
    label: string;
    value: string;
    tone: "neutral" | "success" | "warning";
  }>;
}

export interface OperationLiveEpisode {
  status_label: string;
  status_detail: string;
  active_index: number;
  total_count: number;
  settled_count: number;
  active_tool_label: string;
  active_target: string;
  previous_label: string;
  next_label: string;
  progress_label: string;
  checkpoints: Array<{
    label: string;
    value: string;
    tone: "neutral" | "success" | "warning";
  }>;
}

export function deriveOperationStageExperiencePhase(
  event: NexusOperationEvent | null,
  snapshot: NexusOperationSnapshot | null,
): OperationStageExperiencePhase {
  if (!event) {
    return "idle";
  }
  if (
    event.phase === "queued" ||
    (
      event.surface === "conversation" &&
      (event.phase === "running" || event.phase === "waiting") &&
      countRoundEvents(event, snapshot) <= 1
    )
  ) {
    return "awakening";
  }
  if (event.phase === "running" || event.phase === "waiting") {
    return "running";
  }
  if (event.phase === "done" || event.phase === "cancelled") {
    return countRoundEvents(event, snapshot) > 1 ? "completed" : "settling";
  }
  return "settling";
}

export function countRoundEvents(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
): number {
  const round_events = snapshot?.events.filter((item) => item.round_id === event.round_id) ?? [];
  return round_events.some((item) => item.id === event.id)
    ? round_events.length
    : round_events.length + 1;
}

export function buildOperationContinuationBrief(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
): OperationContinuationBrief {
  const round_events = events.length
    ? events
    : snapshot?.events.filter((item) => item.round_id === event.round_id) ?? [event];
  const failed_count = round_events.filter((item) => item.phase === "error" || item.phase === "cancelled").length;
  const completed_count = round_events.filter((item) => item.phase === "done").length;
  const running_count = round_events.filter((item) => item.phase === "running" || item.phase === "waiting").length;
  const workspace_items = collect_continuation_workspace_items(event, round_events, snapshot);
  const evidence_count = round_events.reduce((total, item) => total + (item.evidence?.length ?? 0), 0)
    + (snapshot?.recent_evidence.length ?? 0);
  const primary_artifact = select_continuation_primary_artifact(event, round_events, workspace_items);

  return {
    status_label: failed_count
      ? "需要回看"
      : running_count
        ? "执行中"
        : "可继续",
    status_detail: failed_count
      ? "本轮存在异常，现场保留了失败步骤、输入和证据。"
      : running_count
        ? "本轮还有未收束步骤，工作台会继续等待后续工具事件。"
        : "本轮工具轨迹、窗口现场和关键产物已经沉淀。",
    resume_prompt: failed_count
      ? `继续排查本轮失败：回看 ${primary_artifact} 的执行现场和错误证据。`
      : `基于本轮产物继续：打开 ${primary_artifact}，按执行记录继续迭代或验证。`,
    primary_artifact,
    checkpoints: [
      {
        label: failed_count ? "异常" : "步骤",
        value: failed_count ? `${failed_count} issue` : `${completed_count}/${round_events.length}`,
        tone: failed_count ? "warning" : "success",
      },
      {
        label: "产物",
        value: workspace_items.length ? `${workspace_items.length} 个文件` : primary_artifact,
        tone: workspace_items.length ? "success" : "neutral",
      },
      {
        label: "证据",
        value: evidence_count ? `${evidence_count} 条证据` : "窗口状态",
        tone: evidence_count ? "success" : "neutral",
      },
      {
        label: running_count ? "现场" : "继续",
        value: running_count ? `${running_count} 个活动` : "就绪",
        tone: running_count || failed_count ? "warning" : "neutral",
      },
    ],
  };
}

export function buildOperationLiveEpisode(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
): OperationLiveEpisode {
  const round_events = events.length
    ? events
    : snapshot?.events.filter((item) => item.round_id === event.round_id) ?? [event];
  const ordered_events = [...round_events].sort((left, right) => left.updated_at - right.updated_at);
  const active_index = Math.max(0, ordered_events.findIndex((item) => item.id === event.id));
  const active_event = ordered_events[active_index] ?? event;
  const previous_event = ordered_events
    .slice(0, active_index)
    .reverse()
    .find((item) => item.phase === "done" || item.phase === "cancelled" || item.phase === "error")
    ?? null;
  const settled_count = ordered_events.filter((item) => (
    item.phase === "done" || item.phase === "cancelled" || item.phase === "error"
  )).length;
  const active_target = active_event.target
    ?? active_event.summary
    ?? active_event.title;
  const is_waiting = active_event.phase === "waiting";
  const is_queued = active_event.phase === "queued";
  const is_terminal = active_event.surface === "terminal";
  const is_handoff = active_event.surface === "conversation";
  const is_api_retry = is_runtime_retry_event(active_event);

  return {
    status_label: is_queued
      ? "桌面唤醒"
      : is_waiting
        ? "等待确认"
        : is_api_retry
          ? "API 重试中"
          : is_handoff
            ? "桌面待命"
            : "现场执行",
    status_detail: is_queued
      ? "字符场正在展开为第一层桌面。"
      : is_waiting
        ? "当前工具停在权限检查点，确认后会继续回到执行现场。"
        : is_api_retry
          ? "模型 API 暂未返回可执行事件，Nexus 桌面正在保留现场。"
          : is_handoff
            ? "Nexus 桌面保持空场，等待第一个工具打开应用窗口。"
          : is_terminal
            ? "命令窗口正在接收真实 stdout、stderr 和退出状态。"
            : "当前应用窗口已成为焦点，前序步骤沉淀在桌面轨迹里。",
    active_index,
    total_count: ordered_events.length,
    settled_count,
    active_tool_label: active_event.tool_name ?? active_event.title,
    active_target,
    previous_label: previous_event
      ? `${previous_event.tool_name ?? previous_event.title} · ${previous_event.target ?? previous_event.summary ?? "已沉淀"}`
      : "从 Nexus 桌面进入",
    next_label: is_waiting
      ? "等待用户确认后继续执行"
      : is_terminal
        ? "等待命令退出并沉淀结果"
        : is_api_retry
          ? "等待模型响应恢复或返回错误"
        : is_handoff
          ? "等待第一个应用窗口"
          : "等待下一个工具事件或本轮收束",
    progress_label: `${active_index + 1}/${ordered_events.length}`,
    checkpoints: [
      {
        label: "上一步",
        value: previous_event ? "沉淀" : "桌面",
        tone: previous_event ? "success" : "neutral",
      },
      {
        label: "当前",
        value: active_event.phase === "waiting"
          ? "确认"
          : active_event.phase === "queued"
            ? "显影"
            : is_api_retry
              ? "重试"
              : "执行",
        tone: active_event.phase === "waiting" || is_api_retry ? "warning" : "success",
      },
      {
        label: "焦点",
        value: active_event.surface,
        tone: "neutral",
      },
      {
        label: "进度",
        value: `${settled_count}/${ordered_events.length}`,
        tone: settled_count > 0 ? "success" : "neutral",
      },
    ],
  };
}

function is_runtime_retry_event(event: NexusOperationEvent): boolean {
  return event.surface === "conversation"
    && (event.evidence ?? []).some((item) => item.label === "api_retry");
}

export function mergeOperationStageSnapshotsForRestore(
  current: NexusOperationSnapshot | null | undefined,
  next: NexusOperationSnapshot,
): NexusOperationSnapshot {
  if (!current || current.key !== next.key) {
    return next;
  }
  if (current.session_key && next.session_key && current.session_key !== next.session_key) {
    return next;
  }

  const active_round_id = next.active_event?.round_id ?? next.events.at(-1)?.round_id ?? null;
  if (!active_round_id) {
    return next;
  }

  const terminal_round_summary = [...next.events]
    .reverse()
    .find((event) => (
      event.round_id === active_round_id &&
      event.kind === "round_summary" &&
      (event.phase === "done" || event.phase === "error" || event.phase === "cancelled")
    )) ?? null;
  const current_round_events = current.events
    .filter((event) => event.round_id === active_round_id)
    .map((event) => settle_stale_live_event_for_round_summary(event, terminal_round_summary));
  if (!current_round_events.length) {
    return next;
  }

  const next_event_ids = new Set(next.events.map((event) => event.id));
  const merged_events = [
    ...current_round_events.filter((event) => !next_event_ids.has(event.id)),
    ...next.events,
  ]
    .sort((left, right) => left.updated_at - right.updated_at)
    .slice(-MAX_MERGED_EVENTS);
  const merged_runtime_events = merge_runtime_events_for_round(
    current.runtime_events ?? [],
    next.runtime_events ?? [],
    active_round_id,
  );

  return {
    ...next,
    active_event: next.active_event ?? current.active_event,
    events: merged_events,
    runtime_events: merged_runtime_events,
    recent_evidence: merge_operation_evidence(current.recent_evidence, next.recent_evidence),
    workspace_events: merge_workspace_events_for_round(
      current.workspace_events,
      next.workspace_events,
      merged_events,
    ),
    updated_at: Math.max(current.updated_at, next.updated_at),
  };
}

function merge_runtime_events_for_round(
  current: NexusOperationSnapshot["runtime_events"],
  next: NexusOperationSnapshot["runtime_events"],
  round_id: string,
): NexusOperationSnapshot["runtime_events"] {
  const next_ids = new Set(next.map((event) => event.id));
  return [
    ...current.filter((event) => event.round_id === round_id && !next_ids.has(event.id)),
    ...next,
  ]
    .sort((left, right) => left.timestamp - right.timestamp)
    .slice(-MAX_MERGED_EVENTS);
}

function collect_continuation_workspace_items(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
): NexusOperationSnapshot["workspace_events"] {
  const workspace_items = snapshot?.workspace_events ?? [];
  if (!workspace_items.length) {
    return [];
  }

  const tool_use_ids = new Set(
    events
      .map((item) => item.tool_use_id)
      .filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id)),
  );
  const targets = new Set(
    events
      .map((item) => item.target)
      .filter((target): target is string => Boolean(target)),
  );

  const scoped_items = workspace_items.filter((item) => (
    Boolean(item.tool_use_id && tool_use_ids.has(item.tool_use_id)) ||
    targets.has(item.path)
  ));
  if (scoped_items.length) {
    return scoped_items;
  }

  const event_target_item = event.target
    ? workspace_items.find((item) => item.path === event.target)
    : null;
  return event_target_item ? [event_target_item] : [];
}

function select_continuation_primary_artifact(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  workspace_items: NexusOperationSnapshot["workspace_events"],
): string {
  const workspace_path = workspace_items.find((item) => !is_low_signal_continuation_value(item.path))?.path;
  if (workspace_path) {
    return workspace_path;
  }

  const file_target = events.find((item) => (
    item.kind !== "round_summary" &&
    item.target &&
    !is_low_signal_continuation_value(item.target) &&
    (
      item.surface === "workspace" ||
      item.surface === "editor" ||
      looks_like_continuation_file_artifact(item.target)
    )
  ))?.target;
  if (file_target) {
    return file_target;
  }

  const semantic_target = events.find((item) => (
    item.kind !== "round_summary" &&
    item.surface !== "terminal" &&
    item.surface !== "conversation" &&
    item.target &&
    !is_low_signal_continuation_value(item.target)
  ))?.target;
  if (semantic_target) {
    return semantic_target;
  }

  if (event.target && !is_low_signal_continuation_value(event.target)) {
    return event.target;
  }
  return event.summary ?? event.title;
}

function is_low_signal_continuation_value(value: string | null | undefined): value is string {
  if (!value) {
    return true;
  }
  const normalized = value.trim().toLowerCase();
  return !normalized || /^\d+\s+turns?$/.test(normalized) || normalized === "0";
}

function looks_like_continuation_file_artifact(value: string): boolean {
  const normalized = value.trim();
  if (!normalized || normalized.endsWith("/")) {
    return false;
  }
  const basename = normalized.split(/[\\/]/).filter(Boolean).at(-1) ?? normalized;
  return /\.[a-z0-9]{1,12}$/i.test(basename);
}

function settle_stale_live_event_for_round_summary(
  event: NexusOperationEvent,
  summary: NexusOperationEvent | null,
): NexusOperationEvent {
  if (
    !summary ||
    (event.phase !== "running" && event.phase !== "waiting" && event.phase !== "queued") ||
    event.id === summary.id
  ) {
    return event;
  }

  const settled_phase = summary.phase === "cancelled"
    ? "cancelled"
    : summary.phase === "error"
      ? "error"
      : "done";
  return {
    ...event,
    phase: settled_phase,
    summary: event.summary ?? summary.summary,
    ended_at: summary.ended_at ?? summary.updated_at,
    updated_at: Math.max(event.updated_at, summary.updated_at),
    evidence: [
      ...(event.evidence ?? []),
      {
        type: summary.phase === "error" ? "error" : "status",
        label: summary.phase === "error" ? "round_error" : "round_settled",
        value: summary.summary ?? summary.title,
      },
    ],
  };
}

function merge_workspace_events_for_round(
  current: NexusOperationSnapshot["workspace_events"],
  next: NexusOperationSnapshot["workspace_events"],
  events: NexusOperationSnapshot["events"],
): NexusOperationSnapshot["workspace_events"] {
  const round_tool_use_ids = new Set(
    events
      .map((event) => event.tool_use_id)
      .filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id)),
  );
  const round_targets = new Set(
    events
      .map((event) => event.target)
      .filter((target): target is string => Boolean(target)),
  );
  const merged_by_id = new Map<string, NexusOperationSnapshot["workspace_events"][number]>();

  for (const item of current) {
    if (
      (item.tool_use_id && round_tool_use_ids.has(item.tool_use_id)) ||
      round_targets.has(item.path)
    ) {
      merged_by_id.set(item.id, item);
    }
  }
  for (const item of next) {
    merged_by_id.set(item.id, item);
  }

  return Array.from(merged_by_id.values())
    .sort((left, right) => right.updated_at - left.updated_at)
    .slice(0, MAX_MERGED_WORKSPACE_EVENTS);
}

function merge_operation_evidence(
  current: NexusOperationSnapshot["recent_evidence"],
  next: NexusOperationSnapshot["recent_evidence"],
): NexusOperationSnapshot["recent_evidence"] {
  const merged = new Map<string, NexusOperationSnapshot["recent_evidence"][number]>();
  for (const item of [...current, ...next]) {
    merged.set(`${item.type}:${item.label}:${item.value ?? ""}`, item);
  }
  return Array.from(merged.values()).slice(-MAX_MERGED_EVIDENCE);
}

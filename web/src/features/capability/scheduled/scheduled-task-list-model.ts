import type {
  ScheduledTaskDeliveryTarget,
  ScheduledTaskItem,
  ScheduledTaskSchedule,
  ScheduledTaskSource,
  ScheduledTaskSessionTarget,
} from "@/types/capability/scheduled-task";
import { format_scheduled_datetime } from "./scheduled-formatters";

function format_interval(seconds: number): string {
  if (seconds % 86400 === 0) {
    return `${seconds / 86400} 天`;
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600} 小时`;
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60} 分钟`;
  }
  return `${seconds} 秒`;
}

export function get_schedule_summary(schedule: ScheduledTaskSchedule): string {
  if (schedule.kind === "every") {
    return `每 ${format_interval(schedule.interval_seconds)}`;
  }
  if (schedule.kind === "cron") {
    return `Cron · ${schedule.cron_expression}`;
  }
  return `单次 · ${format_scheduled_datetime(new Date(schedule.run_at).getTime(), { empty_label: "未安排" })}`;
}

function get_session_target_summary(target: ScheduledTaskSessionTarget): string {
  if (target.kind === "main") {
    return "主会话";
  }
  if (target.kind === "bound") {
    return "使用现有会话";
  }
  if (target.kind === "named") {
    return `专用长期会话 · ${target.named_session_key}`;
  }
  return "每次新建临时会话";
}

export function get_source_kind_label(source: ScheduledTaskSource | null | undefined): string {
  if (!source) {
    return "未知来源";
  }
  if (source.kind === "user_page") {
    return "页面创建";
  }
  if (source.kind === "agent") {
    return "智能体创建";
  }
  if (source.kind === "cli") {
    return "CLI 创建";
  }
  return "系统创建";
}

export function get_delivery_summary(
  delivery: ScheduledTaskDeliveryTarget,
  source: ScheduledTaskSource | null | undefined,
): string {
  if (delivery.mode === "none") {
    return "不回传";
  }
  if (delivery.mode === "last") {
    return "回到最近会话";
  }
  if (delivery.channel === "websocket") {
    if (delivery.to && source?.session_key && delivery.to === source.session_key) {
      return "回到当前选择的会话";
    }
    return "回到指定会话";
  }
  return "回到指定位置";
}

export function get_context_summary(task: ScheduledTaskItem): string {
  const source = task.source;
  if (source?.context_type === "room" && source.context_label) {
    return `Room：${source.context_label}`;
  }
  if (source?.context_type === "agent" && source.context_label) {
    return `智能体：${source.context_label}`;
  }
  return `智能体：${task.agent_id}`;
}

export function get_session_summary(task: ScheduledTaskItem): string {
  if (task.execution_kind === "script") {
    return "脚本执行";
  }
  const source = task.source;
  if (source?.session_label) {
    return source.session_label;
  }
  return get_session_target_summary(task.session_target);
}

function is_same_session_loop(task: ScheduledTaskItem): boolean {
  return Boolean(
    task.session_target.kind === "bound"
      && task.delivery.mode === "explicit"
      && task.delivery.channel === "websocket"
      && task.delivery.to
      && task.source?.session_key
      && task.delivery.to === task.source.session_key,
  );
}

export function get_behavior_summary(task: ScheduledTaskItem): string {
  if (task.execution_kind === "script") {
    return "直接在工作区执行脚本，不占用 Agent 会话；运行输出会写入产物。";
  }
  if (is_same_session_loop(task)) {
    return "在当前会话里持续执行，并直接回到这条会话。";
  }
  if (task.session_target.kind === "bound") {
    return "复用一个已有会话执行；回复位置可单独指定。";
  }
  if (task.session_target.kind === "named") {
    return "固定使用一条专用长期会话执行，便于持续积累上下文。";
  }
  if (task.session_target.kind === "main") {
    return "交给主会话处理，适合把任务继续接在主线对话里。";
  }
  return "每次执行都会新开一条临时会话，不会复用旧上下文。";
}

export function get_primary_status(task: ScheduledTaskItem) {
  if (task.running) {
    return { label: "运行中", tone: "running" as const };
  }
  if (task.failure_streak > 0) {
    return { label: "待恢复", tone: "default" as const };
  }
  if (task.enabled) {
    return { label: "已启用", tone: "active" as const };
  }
  return { label: "已暂停", tone: "idle" as const };
}

export function get_run_status_label(status: string | null | undefined): string {
  if (status === "succeeded") {
    return "成功";
  }
  if (status === "running") {
    return "运行中";
  }
  if (status === "pending") {
    return "等待中";
  }
  if (status === "cancelled") {
    return "已取消";
  }
  if (status === "queued_to_main_session") {
    return "已入主会话";
  }
  if (status === "skipped") {
    return "已跳过";
  }
  if (status === "failed") {
    return "失败";
  }
  return status || "暂无记录";
}

export function get_toggle_action(task: ScheduledTaskItem): {
  label: string;
  pending_label: string;
  tone: "danger" | "primary";
} {
  if (task.enabled) {
    return {
      label: "暂停",
      pending_label: "暂停中",
      tone: "danger",
    };
  }
  return {
    label: "恢复",
    pending_label: "恢复中",
    tone: "primary",
  };
}

export function sort_tasks(items: ScheduledTaskItem[]): ScheduledTaskItem[] {
  return [...items].sort((left, right) => {
    const left_rank = left.running ? 0 : left.enabled ? 1 : 2;
    const right_rank = right.running ? 0 : right.enabled ? 1 : 2;
    if (left_rank !== right_rank) {
      return left_rank - right_rank;
    }
    const left_next_run = left.next_run_at ?? Number.MAX_SAFE_INTEGER;
    const right_next_run = right.next_run_at ?? Number.MAX_SAFE_INTEGER;
    if (left_next_run !== right_next_run) {
      return left_next_run - right_next_run;
    }
    return left.name.localeCompare(right.name, "zh-CN");
  });
}

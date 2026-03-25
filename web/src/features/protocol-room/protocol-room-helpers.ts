import { MessageSquareText, RadioTower, Shield, EyeOff, Sparkles, Zap, WandSparkles } from "lucide-react";

import { ProtocolSnapshotRecord } from "@/types";

export const PROTOCOL_EVENT_STYLE_MAP: Record<string, string> = {
  phase_started: "border-sky-400/30 bg-sky-500/8",
  turn_opened: "border-violet-400/30 bg-violet-500/8",
  action_requested: "border-amber-400/30 bg-amber-500/8",
  action_submitted: "border-orange-400/30 bg-orange-500/8",
  channel_message: "border-emerald-400/30 bg-emerald-500/8",
  phase_resolved: "border-slate-400/30 bg-slate-500/8",
  verdict: "border-rose-400/30 bg-rose-500/8",
  run_completed: "border-fuchsia-400/30 bg-fuchsia-500/8",
  run_paused: "border-zinc-400/30 bg-zinc-500/8",
  run_resumed: "border-lime-400/30 bg-lime-500/8",
};

export function toTimestamp(value?: string | null): number {
  return value ? Date.parse(value) : 0;
}

export function renderPhaseLabel(phase_name: string) {
  const labels: Record<string, string> = {
    setup: "准备阶段",
    night: "夜晚阶段",
    day_announcement: "白天播报",
    day_speeches: "白天发言",
    voting: "投票阶段",
    game_over: "结算阶段",
  };
  return labels[phase_name] || phase_name.replaceAll("_", " ");
}

export function renderEventTypeLabel(event_type: string) {
  const labels: Record<string, string> = {
    phase_started: "阶段开始",
    turn_opened: "轮次开启",
    action_requested: "动作请求",
    action_submitted: "动作已提交",
    channel_message: "频道消息",
    phase_resolved: "阶段结算",
    verdict: "裁决",
    run_completed: "协作完成",
    run_paused: "协作暂停",
    run_resumed: "协作恢复",
    room_state: "房间状态",
  };
  return labels[event_type] || event_type.replaceAll("_", " ");
}

export function renderStatusLabel(status: string) {
  if (status === "running") return "运行中";
  if (status === "paused") return "已暂停";
  if (status === "completed") return "已完成";
  return "已终止";
}

export function renderChannelVisibilityLabel(visibility: string) {
  const labels: Record<string, string> = {
    public: "公开",
    scoped: "受限",
    direct: "私密",
    system: "系统",
  };
  return labels[visibility] || visibility;
}

export function renderRoleLabel(role: string) {
  const labels: Record<string, string> = {
    wolf: "狼人",
    seer: "预言家",
    healer: "守护者",
    villager: "平民",
    member: "成员",
  };
  return labels[role] || role;
}

export function renderSeatStatusLabel(
  status: "pending" | "submitted" | "idle",
  is_alive: boolean,
  is_eliminated: boolean,
) {
  if (!is_alive && is_eliminated) return "已出局";
  if (!is_alive) return "待命";
  if (status === "pending") return "待提交";
  if (status === "submitted") return "已提交";
  return "空闲";
}

export function renderDefinitionLabel(slug: string) {
  if (slug === "werewolf_demo") return "狼人杀演示协议";
  return slug;
}

export function renderDefinitionDescription(slug: string, fallback: string) {
  if (slug === "werewolf_demo") {
    return "这是一个用于验证多人协作房间内核的演示协议。它会生成公开舞台、系统广播、私密频道、阶段推进和结构化动作。";
  }
  return fallback;
}

export function renderActionLabel(action_type: string) {
  const labels: Record<string, string> = {
    kill_target: "击杀目标",
    inspect_target: "查验目标",
    save_target: "保护目标",
    speak: "公开发言",
    vote_target: "投票目标",
    signal_ready: "准备完成",
  };
  return labels[action_type] || action_type.replaceAll("_", " ");
}

export function renderWinnerLabel(winner: string) {
  const labels: Record<string, string> = {
    wolves: "狼人阵营",
    villagers: "村民阵营",
  };
  return labels[winner] || winner;
}

export function renderChannelName(slug: string, fallback: string) {
  const labels: Record<string, string> = {
    "public-main": "公共舞台",
    "system-broadcast": "系统广播",
    "wolves-den": "狼人密聊",
  };
  if (slug.startsWith("direct-")) {
    const owner = slug.replace("direct-", "");
    return `私密频道 · ${owner}`;
  }
  return labels[slug] || fallback;
}

export function renderChannelTopic(slug: string, fallback: string) {
  const labels: Record<string, string> = {
    "public-main": "所有成员都能看到的公开协作舞台",
    "system-broadcast": "主持人与系统裁决广播区",
    "wolves-den": "仅狼人阵营可见的秘密协作频道",
  };
  if (slug.startsWith("direct-")) {
    return "仅当前成员可见的私密行动频道";
  }
  return labels[slug] || fallback;
}

export function renderChannelIcon(channel_type: string) {
  if (channel_type === "system") return RadioTower;
  if (channel_type === "direct") return Shield;
  if (channel_type === "scoped") return EyeOff;
  return MessageSquareText;
}

export function renderEventIcon(event_type: string) {
  if (event_type === "run_completed" || event_type === "verdict") return Sparkles;
  if (event_type === "action_requested" || event_type === "action_submitted") return Zap;
  if (event_type === "turn_opened") return WandSparkles;
  return RadioTower;
}

export function renderFeedHeadline(snapshot: ProtocolSnapshotRecord) {
  if (snapshot.metadata?.redacted) return "受限协作事件";
  return snapshot.headline || renderEventTypeLabel(snapshot.event_type);
}

export function renderFeedBody(snapshot: ProtocolSnapshotRecord) {
  if (snapshot.metadata?.redacted) {
    return "当前视角之外的私密或受限频道发生了一条协作事件。";
  }
  return snapshot.body || "暂无更多说明。";
}

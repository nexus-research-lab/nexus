/**
 * INPUT: Echo authoritative revision、未决目标和领域失败事实。
 * OUTPUT: 不跨资源解锁的恢复状态与完整 Problem/Impact/Recovery 反馈类型。
 * POS: 主动跟进设置的纯交互合同；不访问 React、HTTP 或全局状态。
 */
import type { EchoSettings } from "@/lib/api/settings/echo-api";

export interface EchoSettingsFeedback {
  impact: string;
  message: string;
  nextStep: string;
  title: string;
  tone: "error" | "success" | "warning";
}

export interface EchoSettingsRecoveryControls {
  canCheckLatest: boolean;
  canCompare: boolean;
  canFinishDisabling: boolean;
  checking: boolean;
  checkLatest: () => void;
  finishDisabling: () => void;
  reapplyChange: () => void;
  repairing: boolean;
  useLatest: () => void;
}

export interface PendingEchoChange {
  base: EchoSettings;
  desired: boolean;
  latest: EchoSettings | null;
  cleanupRepairRequired: boolean;
}

export function validEchoSettings(value: EchoSettings): boolean {
  return Number.isSafeInteger(value.version) && value.version > 0;
}

/**
 * INPUT: Echo authoritative revision、未决目标和领域失败事实。
 * OUTPUT: 不跨资源解锁的恢复状态、精简反馈与单一安全动作类型。
 * POS: 主动跟进设置的纯交互合同；不访问 React、HTTP 或全局状态。
 */
import type { EchoSettings } from "@/lib/api/settings/echo-api";

export type EchoSettingsFeedback =
  | {
    impact: string;
    message?: never;
    title: string;
    tone: "error" | "warning";
  }
  | {
    impact?: never;
    message: string;
    title: string;
    tone: "success";
  };

export interface EchoSettingsRecoveryControls {
  canCheckLatest: boolean;
  canCompare: boolean;
  canFinishDisabling: boolean;
  checking: boolean;
  checkLatest: () => void;
  finishDisabling: () => void;
  reapplyChange: () => void;
  repairing: boolean;
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

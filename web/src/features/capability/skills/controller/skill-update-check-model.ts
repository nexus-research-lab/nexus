import type { SkillActionFailure } from "@/types/capability/skill";

export type SkillUpdateCheckNoticeStatus = "current" | "failure" | "updates";

export interface SkillUpdateCheckNotice {
  message: string;
  status: SkillUpdateCheckNoticeStatus;
}

export function buildSkillUpdateCheckNotice(
  availableCount: number,
  failures: SkillActionFailure[],
  manual: boolean,
): SkillUpdateCheckNotice | null {
  const failure = buildFailureMessage(failures);
  if (availableCount > 0) {
    return {
      message: failure
        ? `发现 ${availableCount} 个可更新；${failure}`
        : `发现 ${availableCount} 个可更新`,
      status: "updates",
    };
  }
  if (failure) {
    return {
      message: failure,
      status: "failure",
    };
  }
  return manual
    ? { message: "已是最新版本", status: "current" }
    : null;
}

function buildFailureMessage(failures: SkillActionFailure[]): string | null {
  if (failures.length === 0) {
    return null;
  }
  const failure = failures[0];
  const skillName = failure?.skill_name.trim() || "Skill";
  const reason = failure?.error.trim();
  const message = reason
    ? `${skillName} 检查失败：${reason}`
    : `${skillName} 检查失败`;
  return failures.length > 1
    ? `${message}；另有 ${failures.length - 1} 个 Skill 检查失败`
    : message;
}

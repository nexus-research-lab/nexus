// INPUT: 命令响应与此前是否已有未知提交。
// OUTPUT: 是否有足够证据释放原命令 ID。
// POS: Team 写入恢复规则；鉴权失败只证明本次没有执行，不能否定先前的未知提交。
import { ApiRequestError } from "@/lib/api/core/http-error";

export function isTeamCommandUnapplied(cause: unknown, retrying: boolean): boolean {
  if (!(cause instanceof ApiRequestError) || cause.failure?.effect !== "not_applied") return false;
  if (!retrying) return true;
  // 这些领域拒绝发生在 Relay 精确回执查找之后；网关和身份拒绝不具备这一证据。
  return ["team.membership_version_conflict", "team.configuration_version_conflict", "team.member_state_conflict",
    "team.room_operation_forbidden", "team.resource_not_found"].includes(cause.failure.code);
}

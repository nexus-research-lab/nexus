import type { AgentNameValidationResult } from "@/types/agent/agent";

const AGENT_NAME_MIN_LENGTH = 2;
const AGENT_NAME_MAX_LENGTH = 40;
const AGENT_NAME_ALLOWED_PATTERN = /^[\p{Script=Han}A-Za-z0-9 _-]+$/u;

/** 名称只是展示信息；前端只做与服务端一致的格式预检，不查询重名。 */
export async function validateAgentNameDraft(
  name: string,
): Promise<AgentNameValidationResult> {
  const normalizedName = normalizeAgentName(name);
  const reason = getAgentNameValidationReason(normalizedName);
  const isValid = reason === "";
  return {
    is_available: isValid,
    is_valid: isValid,
    name,
    normalized_name: normalizedName,
    reason,
    workspace_path: null,
  };
}

function normalizeAgentName(name: string): string {
  return name.trim().split(/\s+/u).filter(Boolean).join(" ");
}

function getAgentNameValidationReason(name: string): string {
  const length = Array.from(name).length;
  switch (true) {
    case length === 0:
      return "名称不能为空";
    case length < AGENT_NAME_MIN_LENGTH:
      return `名称至少 ${AGENT_NAME_MIN_LENGTH} 个字符`;
    case length > AGENT_NAME_MAX_LENGTH:
      return `名称不能超过 ${AGENT_NAME_MAX_LENGTH} 个字符`;
    case !AGENT_NAME_ALLOWED_PATTERN.test(name):
      return "仅支持中文、英文、数字、空格、下划线和连字符";
    default:
      return "";
  }
}

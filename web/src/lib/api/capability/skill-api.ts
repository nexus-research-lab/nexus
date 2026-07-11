/**
 * Skill API 服务模块
 *
 * [INPUT]: 依赖 @/types/capability/skill, @/types/system/api
 * [OUTPUT]: 对外提供 Agent 技能接口与全局 Skill Marketplace 接口
 * [POS]: lib 模块的 Skill API 层，被技能市场、Agent 配置与联系人页消费
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi, type RequestApiOptions } from "@/lib/api/core/http";
import type {
  AgentSkillEntry,
  CheckSkillUpdatesResponse,
  ExternalSkillSourceInfo,
  ExternalSkillSourceRequest,
  ExternalSkillSearchItem,
  ExternalSkillPreviewResponse,
  SearchExternalSkillsResponse,
  SkillDetail,
  SkillInfo,
} from "@/types/capability/skill";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();
const SKILL_GIT_OPERATION_TIMEOUT_MS = 360_000;

interface SkillQueryParams {
  agent_id?: string;
  category_key?: string;
  source_type?: string;
  scope?: string;
  q?: string;
}

interface ApiErrorPayload {
  detail?: string;
  message?: string;
}

function buildQuery(params?: Record<string, string | undefined>): string {
  const searchParams = new URLSearchParams();

  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value) {
      searchParams.set(key, value);
    }
  });

  const queryString = searchParams.toString();
  return queryString ? `?${queryString}` : "";
}

function normalizeSkillQuery(
  params?: SkillQueryParams,
): Record<string, string | undefined> | undefined {
  if (!params) {
    return undefined;
  }
  return {
    agent_id: params.agent_id,
    category_key: params.category_key,
    source_type: params.source_type,
    scope: params.scope,
    q: params.q,
  };
}

async function requestSkillApi<T>(
  path: string,
  init?: RequestApiOptions,
): Promise<T> {
  try {
    return await requestApi<T>(`${AGENT_API_BASE_URL}${path}`, init);
  } catch (error) {
    const errorPayload = error as ApiErrorPayload | null;
    throw new Error(
      errorPayload?.detail ||
        errorPayload?.message ||
        (error instanceof Error ? error.message : "请求失败"),
    );
  }
}

/** 获取所有可用 Skill 清单 */
export const getAvailableSkillsApi = async (
  params?: SkillQueryParams,
): Promise<SkillInfo[]> => {
  const query = buildQuery(normalizeSkillQuery(params));
  return requestSkillApi<SkillInfo[]>(`/skills${query}`, {
    method: "GET",
  });
};

/** 获取单个 Skill 详情 */
export const getSkillDetailApi = async (
  skillName: string,
  params?: { agent_id?: string },
): Promise<SkillDetail> => {
  const query = buildQuery(params);
  return requestSkillApi<SkillDetail>(
    `/skills/${encodeURIComponent(skillName)}${query}`,
    {
      method: "GET",
    },
  );
};

/** 导入本地 Skill，支持文件上传或本地路径 */
export const importLocalSkillApi = async (
  fileOrPath: File | string,
): Promise<SkillDetail> => {
  const formData = new FormData();

  if (typeof fileOrPath === "string") {
    formData.append("local_path", fileOrPath);
  } else {
    formData.append("file", fileOrPath);
  }

  return requestSkillApi<SkillDetail>("/skills/import/local", {
    method: "POST",
    body: formData,
  });
};

/** 通过 Git 仓库导入 Skill */
export const importGitSkillApi = async (
  url: string,
  branch?: string,
  path?: string,
): Promise<SkillDetail> => {
  return requestSkillApi<SkillDetail>("/skills/import/git", {
    method: "POST",
    timeout_ms: SKILL_GIT_OPERATION_TIMEOUT_MS,
    body: JSON.stringify({ url, branch, path }),
  });
};

/** 从社区来源搜索外部 Skill */
export const searchExternalSkillsApi = async (
  q: string,
  includeReadme: boolean = false,
  signal?: AbortSignal,
): Promise<SearchExternalSkillsResponse> => {
  const query = buildQuery({
    q,
    include_readme: includeReadme ? "true" : undefined,
  });
  return requestSkillApi<SearchExternalSkillsResponse>(
    `/skills/search/external${query}`,
    {
      method: "GET",
      signal,
    },
  );
};

/** 获取社区 Skill 预览内容 */
export const getExternalSkillPreviewApi = async (
  detailUrl: string,
): Promise<ExternalSkillPreviewResponse> => {
  const query = buildQuery({ detail_url: detailUrl });
  return requestSkillApi<ExternalSkillPreviewResponse>(
    `/skills/external/preview${query}`,
    {
      method: "GET",
    },
  );
};

/** 从社区来源导入指定 Skill */
export const importExternalSkillApi = async (
  item: ExternalSkillSearchItem,
): Promise<SkillDetail> => {
  return requestSkillApi<SkillDetail>("/skills/import/skills-sh", {
    method: "POST",
    timeout_ms: SKILL_GIT_OPERATION_TIMEOUT_MS,
    body: JSON.stringify(item),
  });
};

/** 获取社区 Skill 来源配置 */
export const listExternalSkillSourcesApi =
  async (): Promise<ExternalSkillSourceInfo[]> => {
    return requestSkillApi<ExternalSkillSourceInfo[]>("/skills/sources", {
      method: "GET",
    });
  };

/** 更新社区 Skill 来源配置 */
export const updateExternalSkillSourceApi = async (
  sourceId: string,
  payload: Partial<ExternalSkillSourceRequest>,
): Promise<ExternalSkillSourceInfo> => {
  return requestSkillApi<ExternalSkillSourceInfo>(
    `/skills/sources/${encodeURIComponent(sourceId)}`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
  );
};

/** 检查全局已导入 Skill 是否有更新 */
export const checkSkillUpdatesApi =
  async (): Promise<CheckSkillUpdatesResponse> => {
    return requestSkillApi<CheckSkillUpdatesResponse>(
      "/skills/check-updates",
      {
        method: "POST",
        timeout_ms: SKILL_GIT_OPERATION_TIMEOUT_MS,
      },
    );
  };

/** 更新单个全局 Skill */
export const updateSingleSkillApi = async (
  skillName: string,
): Promise<SkillDetail> => {
  return requestSkillApi<SkillDetail>(
    `/skills/${encodeURIComponent(skillName)}/update`,
    {
      method: "POST",
      timeout_ms: SKILL_GIT_OPERATION_TIMEOUT_MS,
    },
  );
};

/** 从技能库删除外部 Skill */
export const deleteSkillApi = async (skillName: string): Promise<void> => {
  await requestSkillApi<{ success: boolean }>(
    `/skills/${encodeURIComponent(skillName)}`,
    {
      method: "DELETE",
    },
  );
};

/** 获取 Agent 的 Skill 列表（含安装状态） */
export const getAgentSkillsApi = async (
  agentId: string,
  signal?: AbortSignal,
): Promise<AgentSkillEntry[]> => {
  return requestSkillApi<AgentSkillEntry[]>(
    `/agents/${encodeURIComponent(agentId)}/skills`,
    {
      method: "GET",
      signal,
    },
  );
};

/** 为 Agent 安装 Skill */
export const installSkillApi = async (
  agentId: string,
  skillName: string,
): Promise<AgentSkillEntry> => {
  return requestSkillApi<AgentSkillEntry>(
    `/agents/${encodeURIComponent(agentId)}/skills`,
    {
      method: "POST",
      body: JSON.stringify({ skill_name: skillName }),
    },
  );
};

/** 从 Agent 卸载 Skill */
export const uninstallSkillApi = async (
  agentId: string,
  skillName: string,
): Promise<void> => {
  await requestSkillApi<{ success: boolean }>(
    `/agents/${encodeURIComponent(agentId)}/skills/${encodeURIComponent(skillName)}`,
    {
      method: "DELETE",
    },
  );
};

/**
 * Skill API 服务模块
 *
 * [INPUT]: 依赖 @/types/capability/skill, @/types/system/api
 * [OUTPUT]: 提供 Agent 技能读取/显式启停与 Skill Marketplace 接口，并原样保留传输层 FailureCore；不保留无人使用的 install/uninstall 兼容写入口
 * [POS]: lib 模块的 Skill API 层，被技能市场、Agent 配置与联系人页消费
 */

import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { RequestApiOptions } from "@/lib/api/core/http-request";
import type {
  AgentSkillEntry,
  CheckSkillUpdatesResponse,
  CreateExternalSkillSourceRequest,
  ExternalSkillSourceInfo,
  ExternalSkillSourceRequest,
  ExternalSkillSearchItem,
  ExternalSkillPreviewResponse,
  SearchExternalSkillsResponse,
  SkillDetail,
  SkillAgentBinding,
  SkillBindingTargetScope,
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
  return requestApi<T>(`${AGENT_API_BASE_URL}${path}`, init);
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

/** 获取技能在各 Agent 上的启用状态 */
export const getSkillAgentsApi = async (
  skillName: string,
): Promise<SkillAgentBinding[]> => {
  return requestSkillApi<SkillAgentBinding[]>(
    `/skills/${encodeURIComponent(skillName)}/agents`,
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

/** 从社区或私有来源搜索外部 Skill */
export const searchExternalSkillsApi = async (
  q: string,
  includeReadme: boolean = false,
  sourceId?: string,
  signal?: AbortSignal,
): Promise<SearchExternalSkillsResponse> => {
  const query = buildQuery({
    q,
    include_readme: includeReadme ? "true" : undefined,
    source_id: sourceId,
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

/** 从社区或私有来源导入指定 Skill */
export const importExternalSkillApi = async (
  item: ExternalSkillSearchItem,
): Promise<SkillDetail> => {
  if (item.source_kind === "private_registry") {
    return requestSkillApi<SkillDetail>("/skills/import/source", {
      method: "POST",
      timeout_ms: SKILL_GIT_OPERATION_TIMEOUT_MS,
      body: JSON.stringify({
        source_id: item.source_key,
        skill_id: item.package_spec,
      }),
    });
  }
  return requestSkillApi<SkillDetail>("/skills/import/skills-sh", {
    method: "POST",
    timeout_ms: SKILL_GIT_OPERATION_TIMEOUT_MS,
    body: JSON.stringify(item),
  });
};

/** 获取社区与私有 Skill 来源配置 */
export const listExternalSkillSourcesApi =
  async (): Promise<ExternalSkillSourceInfo[]> => {
    return requestSkillApi<ExternalSkillSourceInfo[]>("/skills/sources", {
      method: "GET",
    });
  };

/** 新增一个私有 Skill 来源 */
export const createExternalSkillSourceApi = async (
  payload: CreateExternalSkillSourceRequest,
): Promise<ExternalSkillSourceInfo> => {
  return requestSkillApi<ExternalSkillSourceInfo>("/skills/sources", {
    method: "POST",
    body: JSON.stringify(payload),
  });
};

/** 更新 Skill 来源配置 */
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

/** 删除一个私有 Skill 来源 */
export const deleteExternalSkillSourceApi = async (
  sourceId: string,
): Promise<void> => {
  await requestSkillApi<{ success: boolean }>(
    `/skills/sources/${encodeURIComponent(sourceId)}`,
    { method: "DELETE" },
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

/** 获取 Agent 的 Skill 列表（含启用状态） */
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

/** 切换 Agent 技能状态；停用只改变绑定，不删除工作区文件。 */
export const setAgentSkillEnabledApi = async (
  agentId: string,
  skillName: string,
  enabled: boolean,
  targetScope: SkillBindingTargetScope,
): Promise<AgentSkillEntry> => {
  return requestSkillApi<AgentSkillEntry>(
    `/agents/${encodeURIComponent(agentId)}/skills/${encodeURIComponent(skillName)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ enabled, target_scope: targetScope }),
    },
  );
};

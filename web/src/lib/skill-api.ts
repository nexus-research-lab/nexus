/**
 * Skill API 服务模块
 *
 * [INPUT]: 依赖 @/types/skill, @/types/api
 * [OUTPUT]: 对外提供 Agent 技能接口与全局 Skill Marketplace 接口
 * [POS]: lib 模块的 Skill API 层，被技能市场、Agent 配置与联系人页消费
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/http";
import type {
  AgentSkillEntry,
  ExternalSkillSearchItem,
  ExternalSkillPreviewResponse,
  SearchExternalSkillsResponse,
  SkillDetail,
  SkillInfo,
  UpdateInstalledSkillsResponse,
} from "@/types/skill";

const AGENT_API_BASE_URL = get_agent_api_base_url();

interface SkillQueryParams {
  agent_id?: string;
  category_key?: string;
  source_type?: string;
  q?: string;
}

interface ApiErrorPayload {
  detail?: string;
  message?: string;
}

function build_query(params?: Record<string, string | undefined>): string {
  const search_params = new URLSearchParams();

  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value) {
      search_params.set(key, value);
    }
  });

  const query_string = search_params.toString();
  return query_string ? `?${query_string}` : "";
}

function normalize_skill_query(
  params?: SkillQueryParams,
): Record<string, string | undefined> | undefined {
  if (!params) {
    return undefined;
  }
  return {
    agent_id: params.agent_id,
    category_key: params.category_key,
    source_type: params.source_type,
    q: params.q,
  };
}

async function request_skill_api<T>(path: string, init?: RequestInit): Promise<T> {
  try {
    return await request_api<T>(`${AGENT_API_BASE_URL}${path}`, init);
  } catch (error) {
    const error_payload = error as ApiErrorPayload | null;
    throw new Error(
      error_payload?.detail ||
      error_payload?.message ||
      (error instanceof Error ? error.message : "请求失败"),
    );
  }
}

/** 获取所有可用 Skill 清单 */
export const get_available_skills_api = async (params?: SkillQueryParams): Promise<SkillInfo[]> => {
  const query = build_query(normalize_skill_query(params));
  return request_skill_api<SkillInfo[]>(`/skills${query}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
};

/** 获取单个 Skill 详情 */
export const get_skill_detail_api = async (
  skill_name: string,
  params?: { agent_id?: string },
): Promise<SkillDetail> => {
  const query = build_query(params);
  return request_skill_api<SkillDetail>(`/skills/${encodeURIComponent(skill_name)}${query}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
};

/** 导入本地 Skill，支持文件上传或本地路径 */
export const import_local_skill_api = async (file_or_path: File | string): Promise<SkillDetail> => {
  const form_data = new FormData();

  if (typeof file_or_path === "string") {
    form_data.append("local_path", file_or_path);
  } else {
    form_data.append("file", file_or_path);
  }

  return request_skill_api<SkillDetail>("/skills/import/local", {
    method: "POST",
    body: form_data,
  });
};

/** 通过 Git 仓库导入 Skill */
export const import_git_skill_api = async (url: string, branch?: string): Promise<SkillDetail> => {
  return request_skill_api<SkillDetail>("/skills/import/git", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, branch }),
  });
};

/** 从 skills.sh 搜索外部 Skill */
export const search_external_skills_api = async (
  q: string,
  include_readme: boolean = false,
): Promise<ExternalSkillSearchItem[]> => {
  const query = build_query({
    q,
    include_readme: include_readme ? "true" : undefined,
  });
  const result = await request_skill_api<SearchExternalSkillsResponse>(`/skills/search/external${query}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  return result.results;
};

/** 获取 skills.sh 技能预览内容 */
export const get_external_skill_preview_api = async (
  detail_url: string,
): Promise<ExternalSkillPreviewResponse> => {
  const query = build_query({ detail_url });
  return request_skill_api<ExternalSkillPreviewResponse>(`/skills/external/preview${query}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
};

/** 从 skills.sh 导入指定 Skill */
export const import_skills_sh_skill_api = async (
  package_spec: string,
  skill_slug: string,
): Promise<SkillDetail> => {
  return request_skill_api<SkillDetail>("/skills/import/skills-sh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ package_spec, skill_slug }),
  });
};

/** 更新全局已导入 Skill */
export const update_imported_skills_api = async (): Promise<UpdateInstalledSkillsResponse> => {
  return request_skill_api<UpdateInstalledSkillsResponse>("/skills/update-imported", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
};

/** 更新单个全局 Skill */
export const update_single_skill_api = async (skill_name: string): Promise<SkillDetail> => {
  return request_skill_api<SkillDetail>(`/skills/${encodeURIComponent(skill_name)}/update`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
};

/** 从技能库删除外部 Skill */
export const delete_skill_api = async (skill_name: string): Promise<void> => {
  await request_skill_api<{ success: boolean }>(`/skills/${encodeURIComponent(skill_name)}`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
  });
};

/** 获取 Agent 的 Skill 列表（含安装状态） */
export const get_agent_skills_api = async (agent_id: string): Promise<AgentSkillEntry[]> => {
  return request_skill_api<AgentSkillEntry[]>(`/agents/${encodeURIComponent(agent_id)}/skills`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
};

/** 为 Agent 安装 Skill */
export const install_skill_api = async (
  agent_id: string,
  skill_name: string,
): Promise<AgentSkillEntry> => {
  return request_skill_api<AgentSkillEntry>(`/agents/${encodeURIComponent(agent_id)}/skills`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ skill_name }),
  });
};

/** 从 Agent 卸载 Skill */
export const uninstall_skill_api = async (agent_id: string, skill_name: string): Promise<void> => {
  await request_skill_api<{ success: boolean }>(
    `/agents/${encodeURIComponent(agent_id)}/skills/${encodeURIComponent(skill_name)}`,
    {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
    },
  );
};

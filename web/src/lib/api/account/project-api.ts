import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  ProjectAccess,
  ProjectGrantResult,
  SharedProject,
} from "@/types/settings/project";

const PROJECTS_BASE_URL = `${getAgentApiBaseUrl()}/projects`;

export async function getProjectsApi(): Promise<SharedProject[]> {
  return requestApi<SharedProject[]>(PROJECTS_BASE_URL, {
    method: "GET",
  });
}

export async function createProjectApi(
  projectId: string,
): Promise<SharedProject> {
  return requestApi<SharedProject>(PROJECTS_BASE_URL, {
    method: "POST",
    body: JSON.stringify({ project_id: projectId }),
  });
}

export async function updateProjectMemberApi(
  projectId: string,
  ownerUserId: string,
  access: ProjectAccess,
): Promise<ProjectGrantResult> {
  return requestApi<ProjectGrantResult>(
    `${PROJECTS_BASE_URL}/${encodeURIComponent(projectId)}/members/${encodeURIComponent(ownerUserId)}`,
    {
      method: "PUT",
      body: JSON.stringify({ access }),
    },
  );
}

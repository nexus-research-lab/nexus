import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  createProjectApi,
  getProjectsApi,
  updateProjectMemberApi,
} from "@/lib/api/account/project-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ProjectAccess, SharedProject } from "@/types/settings/project";

import {
  buildProjectFeedback,
  projectMemberDraftKey,
  type ProjectAdminViewModel,
  type ProjectFeedback,
} from "./project-admin-model";

interface UseProjectAdminOptions {
  canManageMembers: boolean;
}

export function useProjectAdmin({
  canManageMembers,
}: UseProjectAdminOptions) {
  const { t } = useI18n();
  const [projects, setProjects] = useState<SharedProject[]>([]);
  const [loading, setLoading] = useState(true);
  const [pendingKey, setPendingKey] = useState<string | null>(null);
  const [newProjectId, setNewProjectId] = useState("");
  const [memberDrafts, setMemberDrafts] = useState<Record<string, string>>({});
  const [feedback, setFeedback] = useState<ProjectFeedback | null>(null);
  const transactionRunning = useRef(false);

  const runTransaction = useCallback(async (
    key: string,
    request: () => Promise<void>,
  ) => {
    if (transactionRunning.current) {
      return false;
    }
    transactionRunning.current = true;
    setPendingKey(key);
    try {
      await request();
      return true;
    } finally {
      transactionRunning.current = false;
      setPendingKey(null);
    }
  }, []);

  const loadProjects = useCallback(async () => {
    setLoading(true);
    try {
      setProjects(await getProjectsApi());
      setFeedback((current) => current?.tone === "error" ? null : current);
    } catch (error) {
      setFeedback(buildProjectFeedback(t, "load-failed", error));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadProjects();
  }, [loadProjects]);

  const createProject = useCallback(async () => {
    const projectId = newProjectId.trim();
    if (!projectId) {
      setFeedback(buildProjectFeedback(t, "create-failed"));
      return;
    }
    await runTransaction("create-project", async () => {
      try {
        const project = await createProjectApi(projectId);
        setProjects((current) => {
          const next = current.filter((item) => item.project_id !== project.project_id);
          return [...next, project].sort((left, right) =>
            left.project_id.localeCompare(right.project_id),
          );
        });
        setNewProjectId("");
        setFeedback(buildProjectFeedback(t, "create-succeeded"));
      } catch (error) {
        setFeedback(buildProjectFeedback(t, "create-failed", error));
      }
    });
  }, [newProjectId, runTransaction, t]);

  const updateMember = useCallback(async (
    projectId: string,
    ownerUserId: string,
    access: ProjectAccess,
  ): Promise<boolean> => {
    if (!canManageMembers || !ownerUserId.trim()) {
      return false;
    }
    let succeeded = false;
    await runTransaction(`member:${projectId}:${ownerUserId}`, async () => {
      try {
        await updateProjectMemberApi(projectId, ownerUserId.trim(), access);
        setProjects(await getProjectsApi());
        setFeedback(buildProjectFeedback(t, "grant-succeeded"));
        succeeded = true;
      } catch (error) {
        setFeedback(buildProjectFeedback(t, "grant-failed", error));
      }
    });
    return succeeded;
  }, [canManageMembers, runTransaction, t]);

  const addMember = useCallback(async (projectId: string) => {
    const draftKey = projectMemberDraftKey(projectId);
    const ownerUserId = memberDrafts[draftKey]?.trim() ?? "";
    if (!ownerUserId) {
      setFeedback(buildProjectFeedback(t, "grant-failed"));
      return;
    }
    if (await updateMember(projectId, ownerUserId, "write")) {
      setMemberDrafts((current) => ({ ...current, [draftKey]: "" }));
    }
  }, [memberDrafts, t, updateMember]);

  const changeMemberDraft = useCallback((projectId: string, value: string) => {
    const draftKey = projectMemberDraftKey(projectId);
    setMemberDrafts((current) => ({ ...current, [draftKey]: value }));
  }, []);

  const viewModel: ProjectAdminViewModel = useMemo(() => ({
    canManageMembers,
    feedback,
    loading,
    memberDrafts,
    newProjectId,
    pendingKey,
    projects,
  }), [
    canManageMembers,
    feedback,
    loading,
    memberDrafts,
    newProjectId,
    pendingKey,
    projects,
  ]);

  return {
    viewModel,
    addMember,
    changeMemberDraft,
    createProject,
    dismissFeedback: () => setFeedback(null),
    refreshProjects: loadProjects,
    setNewProjectId,
    updateMember,
  };
}

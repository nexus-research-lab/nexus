export type ProjectAccess = "read" | "write" | "none";

export interface SharedProject {
  project_id: string;
  group_name: string;
  gid: number;
  root: string;
  members: Record<string, Exclude<ProjectAccess, "none">>;
  generation: number;
}

export interface ProjectGrantResult {
  changed: boolean;
}

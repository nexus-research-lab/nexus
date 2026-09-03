// INPUT: Workspace 相对路径、用户可见 Agent 名称与路径变更前后缀。
// OUTPUT: 不泄漏物理目录的显示层级，以及稳定的路径拼接、聚焦和替换结果。
// POS: Workspace 路径纯模型；不读取文件系统，也不渲染 Breadcrumb。

export function getParentWorkspacePath(path: string): string | null {
  const separatorIndex = path.lastIndexOf("/");
  return separatorIndex < 0 ? null : path.slice(0, separatorIndex);
}

export function getWorkspaceRootLabel(
  displayName: string | null | undefined,
  name: string | null | undefined,
  fallbackLabel: string,
): string {
  return displayName?.trim() || name?.trim() || fallbackLabel;
}

export function getWorkspaceFileLocationSegments(
  filePath: string,
  workspaceRootLabel: string,
): string[] {
  const parentPath = getParentWorkspacePath(filePath);
  return [
    workspaceRootLabel,
    ...(parentPath ? parentPath.split(/[\\/]+/).filter(Boolean) : []),
  ];
}

export function getWorkspaceFocusPath(path?: string | null): string | null {
  return path ? getParentWorkspacePath(path) : null;
}

export function joinWorkspacePath(parentPath: string | null, name: string): string {
  return parentPath ? `${parentPath}/${name}` : name;
}

export function joinLocalWorkspacePath(
  workspaceRoot: string,
  relativePath: string,
): string {
  const normalizedRoot = workspaceRoot.trim().replace(/[\\/]+$/, "");
  if (!normalizedRoot) {
    return relativePath;
  }
  const separator = normalizedRoot.includes("\\") ? "\\" : "/";
  return `${normalizedRoot}${separator}${relativePath.replace(/[\\/]+/g, separator)}`;
}

export function isWorkspacePathWithin(path: string | null, parentPath: string): boolean {
  return Boolean(path && (path === parentPath || path.startsWith(`${parentPath}/`)));
}

/**
 * 只替换同一文件或目录子树的前缀，返回 null 表示当前路径不受影响。
 */
export function replaceWorkspacePathPrefix(
  path: string | null,
  previousPrefix: string,
  nextPrefix: string,
): string | null {
  if (!isWorkspacePathWithin(path, previousPrefix)) {
    return null;
  }
  return `${nextPrefix}${path?.slice(previousPrefix.length) ?? ""}`;
}

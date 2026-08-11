export function getParentWorkspacePath(path: string): string | null {
  const separatorIndex = path.lastIndexOf("/");
  return separatorIndex < 0 ? null : path.slice(0, separatorIndex);
}

export function getWorkspaceRootLabel(
  workspacePath: string,
  fallbackLabel: string,
): string {
  const pathSegments = workspacePath.split(/[\\/]+/).filter(Boolean);
  return pathSegments.at(-1) ?? fallbackLabel;
}

export function getWorkspaceFileLocationLabel(
  filePath: string,
  workspaceRootLabel: string,
): string {
  return getParentWorkspacePath(filePath) || workspaceRootLabel;
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

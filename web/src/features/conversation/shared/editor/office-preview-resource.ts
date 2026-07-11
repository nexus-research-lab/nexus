import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent-manage-api";

export const DEFAULT_OFFICE_PREVIEW_MAX_BYTES = 15 * 1024 * 1024;

interface FetchOfficePreviewBufferOptions {
  agentId: string;
  fileLabel: string;
  maxBytes?: number;
  path: string;
  signal: AbortSignal;
}

function assertPreviewSize(
  byteLength: number,
  fileLabel: string,
  maxBytes: number,
): void {
  if (byteLength <= maxBytes) {
    return;
  }

  const maxMegabytes = Math.floor(maxBytes / (1024 * 1024));
  throw new Error(
    `${fileLabel} 文件超过 ${maxMegabytes}MB，当前无法内置预览，请使用上方按钮处理`,
  );
}

/** Office 预览共用同一下载边界，避免不同格式遗漏响应头或实际载荷校验。 */
export async function fetchOfficePreviewBuffer({
  agentId,
  fileLabel,
  maxBytes = DEFAULT_OFFICE_PREVIEW_MAX_BYTES,
  path,
  signal,
}: FetchOfficePreviewBufferOptions): Promise<ArrayBuffer> {
  const response = await fetch(getWorkspaceFilePreviewUrl(agentId, path), {
    credentials: "include",
    signal,
  });
  if (!response.ok) {
    throw new Error(`读取 ${fileLabel} 文件失败：HTTP ${response.status}`);
  }

  assertPreviewSize(
    Number(response.headers.get("content-length") || 0),
    fileLabel,
    maxBytes,
  );
  const buffer = await response.arrayBuffer();
  assertPreviewSize(buffer.byteLength, fileLabel, maxBytes);
  return buffer;
}

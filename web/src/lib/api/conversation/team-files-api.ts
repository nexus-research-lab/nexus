// INPUT: 在线群文件身份、用户选择的文件与取消信号。
// OUTPUT: 同源共享目录、幂等上传和有界下载；不传递本机路径。
// POS: Relay 共享文件的浏览器传输边界。
import { requestApi } from "@/lib/api/core/http";
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { applyDesktopRequestHeaders } from "@/config/desktop-runtime";

export interface TeamFile {id: string; name: string; size: number; created_at: string}
const filesURL = (roomId: string) => `${getAgentApiBaseUrl()}/team/rooms/${encodeURIComponent(roomId)}/files`;
const hexDigest = async (data: BufferSource) => Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", data)), (value) => value.toString(16).padStart(2,"0")).join("");

export function listTeamFiles(roomId: string, signal: AbortSignal) {
  return requestApi<TeamFile[]>(filesURL(roomId), {signal});
}

export async function uploadTeamFile(roomId: string, file: File, signal: AbortSignal) {
  if (file.size > 32 * 1024 * 1024) throw new Error("File exceeds 32 MiB");
  const digest = await hexDigest(await file.arrayBuffer());
  const command = await hexDigest(new TextEncoder().encode(`${file.name}\0${digest}`));
  signal.throwIfAborted();
  return requestApi<TeamFile>(filesURL(roomId), {method: "POST", body: file, timeout_ms: 60_000, signal, headers: {
    "Content-Type": "application/octet-stream", "X-File-Name": encodeURIComponent(file.name), "X-File-SHA256": digest, "Idempotency-Key": `file-${command}`,
  }});
}

export async function downloadTeamFile(roomId: string, file: TeamFile, signal: AbortSignal) {
  const url = `${filesURL(roomId)}/${encodeURIComponent(file.id)}`;
  const headers = new Headers(); applyDesktopRequestHeaders(url, headers);
  const response = await fetch(url, {credentials: "include", headers, signal: AbortSignal.any([signal, AbortSignal.timeout(60_000)])});
  if (!response.ok) throw new Error("Download failed");
  const blob = await response.blob(); signal.throwIfAborted();
  if (blob.size !== file.size || blob.size > 32 * 1024 * 1024) throw new Error("Invalid file size");
  return blob;
}

/** 桌面协议共享的 URL 归一化，避免鉴权与诊断各自解释请求地址。 */

export function currentDesktopLocationPath(): string {
  return `${window.location.pathname || "/"}${window.location.search}${window.location.hash}`;
}

export function desktopRequestPath(input: string): string {
  try {
    const requestUrl = new URL(input, window.location.href);
    return `${requestUrl.pathname}${requestUrl.search}${requestUrl.hash}`;
  } catch {
    return input.trim() || "unknown";
  }
}

export function isDesktopApiRequest(input: string, apiBaseUrl: string): boolean {
  try {
    const requestUrl = new URL(input, window.location.href);
    const apiUrl = new URL(apiBaseUrl, window.location.href);
    const apiPath = apiUrl.pathname.replace(/\/+$/, "");
    return requestUrl.origin === apiUrl.origin &&
      (requestUrl.pathname === apiPath || requestUrl.pathname.startsWith(`${apiPath}/`));
  } catch {
    return false;
  }
}

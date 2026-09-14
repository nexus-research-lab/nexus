// INPUT: 本机可选主题/排版偏好的存储键和值。
// OUTPUT: 可用时持久化；浏览器拒绝存储时保持当前页面可用。
// POS: 仅主题偏好容错边界，不用于业务事务或权限存储。
export function readThemePreference(key: string): string | null {
  try { return typeof window === "undefined" ? null : window.localStorage.getItem(key); }
  catch { return null; }
}
export function writeThemePreference(key: string, value: string): void {
  try { if (typeof window !== "undefined") window.localStorage.setItem(key, value); }
  catch { /* Optional persistence must not prevent applying the live preference. */ }
}
export function removeThemePreference(key: string): void {
  try { if (typeof window !== "undefined") window.localStorage.removeItem(key); }
  catch { /* The current document remains usable without persistent preferences. */ }
}

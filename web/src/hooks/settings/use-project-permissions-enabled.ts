// INPUT: 后端运行配置中的 ACL 可用性及 owner 变更事件。
// OUTPUT: 默认关闭、随权威配置更新的项目权限能力。
// POS: 设置能力订阅；不从用户角色推断宿主 ACL 支持。
import { useSyncExternalStore } from "react";
import { getProjectPermissionsEnabled, USER_PREFERENCES_CHANGED_EVENT } from "@/config/runtime-options";

function subscribe(listener: () => void) {
  window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, listener);
  return () => window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, listener);
}

export function useProjectPermissionsEnabled() {
  return useSyncExternalStore(subscribe, getProjectPermissionsEnabled, () => false);
}

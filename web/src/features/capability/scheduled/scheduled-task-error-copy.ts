interface ScheduledTaskErrorCopy {
  detail: string;
  summary: string;
}

export function getScheduledTaskErrorCopy(
  value: string | null | undefined,
): ScheduledTaskErrorCopy | null {
  const message = value?.trim();
  if (!message) {
    return null;
  }
  if (message === "previous run is still running; overlap_policy=skip") {
    return {
      detail: `上一次运行仍未结束，任务按“跳过重叠执行”策略略过了本次调度。\n\n技术信息：${message}`,
      summary: "上一次运行未结束，本次调度已跳过",
    };
  }
  if (message === "Permission request timeout") {
    return {
      detail: `任务等待权限响应超时。\n\n技术信息：${message}`,
      summary: "等待权限响应超时",
    };
  }
  return { detail: message, summary: message.split("\n", 1)[0] ?? message };
}

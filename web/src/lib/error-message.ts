// INPUT: 任意失败值与当前操作的用户可执行兜底文案。
// OUTPUT: 保留具体业务错误；把无行动价值的内部服务占位错误收敛为当前操作文案。
// POS: Web 错误展示的最小净化边界，不改写日志或后端 failure code。
const INTERNAL_ERROR_PLACEHOLDERS = new Set([
  "服务内部错误",
  "内部服务错误",
  "Internal server error",
  "Internal Server Error",
]);

export function getErrorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof Error)) {
    return fallback;
  }
  const message = error.message.trim();
  return message && !INTERNAL_ERROR_PLACEHOLDERS.has(message)
    ? message
    : fallback;
}

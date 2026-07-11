export function formatMessageTime(timestamp?: number | null): string {
  if (!timestamp) {
    return "-- --:--";
  }

  const messageDate = new Date(timestamp);
  const currentYear = new Date().getFullYear();
  return messageDate.toLocaleString("zh-CN", {
    ...(messageDate.getFullYear() === currentYear ? {} : { year: "numeric" }),
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

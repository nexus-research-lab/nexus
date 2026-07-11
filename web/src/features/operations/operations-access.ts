export function canUseOperations(role?: string | null): boolean {
  return role === "owner" || role === "admin";
}

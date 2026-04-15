export interface TodoItem {
  content: string;
  status: "pending" | "completed" | "in_progress";
  active_form?: string;
}

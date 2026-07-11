export type MessageSegment<T extends Record<string, string>> = {
  [K in keyof T]: string;
};

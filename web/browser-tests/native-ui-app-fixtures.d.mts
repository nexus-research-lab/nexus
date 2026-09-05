// INPUT: Node HTTP server and requests from isolated App-shell verification.
// OUTPUT: Type boundary shared by TypeScript browser tests and the Node fixture server.
// POS: Declarations for the adjacent test-only fixture module.
import type { Server } from "node:http";

export function appShellRead(method: string, pathname: string): { data: unknown } | null;
export const APP_SHELL_INIT_SCRIPT: string;
export function attachAppShellSocket(server: Server, observations: {
  rejected: string[]; socketConnections: number; socketMessages: string[];
}): () => void;

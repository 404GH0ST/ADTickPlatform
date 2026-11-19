/** Human label for a game-core match state (server- and client-safe). */
export function formatMatchStateLabel(state?: string): string {
  if (!state) {
    return "Inactive";
  }
  const labels: Record<string, string> = {
    not_started: "Not started",
    running: "Running",
    paused: "Paused",
    stopped: "Stopped",
    finished: "Finished",
  };
  const key = state.toLowerCase();
  return labels[key] ?? state.replaceAll("_", " ");
}

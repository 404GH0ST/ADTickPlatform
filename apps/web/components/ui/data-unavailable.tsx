/**
 * Stands in for a list whose request failed. An empty list says "nothing has
 * happened yet"; this says the answer is unknown.
 */
export function DataUnavailable({ title, description }: { title: string; description: string }) {
  return (
    <div
      data-slot="well"
      data-testid="data-unavailable"
      role="status"
      className="grid justify-items-start gap-1 rounded-sm border border-border/70 bg-muted/10 p-4"
    >
      <p className="text-sm font-medium text-foreground">{title}</p>
      <p className="max-w-[65ch] text-sm text-muted-foreground">{description}</p>
    </div>
  );
}

/** Shown above data that is still on screen after a refresh failed. */
export function StaleDataNotice() {
  return (
    <p
      data-slot="well"
      data-testid="stale-data-notice"
      role="status"
      className="rounded-sm border border-border/70 bg-muted/10 px-4 py-3 text-sm font-medium text-foreground"
    >
      Showing the last data received. It may be out of date until the platform API responds.
    </p>
  );
}

export const SCORES_UNAVAILABLE = {
  title: "Scores can't be loaded",
  description: "The platform API didn't return the standings. This doesn't mean no team has scored.",
};

export const ATTACKS_UNAVAILABLE = {
  title: "Attacks can't be loaded",
  description: "The platform API didn't return the attack feed. This doesn't mean no attacks have landed.",
};

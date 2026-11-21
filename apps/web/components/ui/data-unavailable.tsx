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

export const SCORES_UNAVAILABLE = {
  title: "Scores can't be loaded",
  description: "The platform API didn't return the standings. This doesn't mean no team has scored.",
};

export const ATTACKS_UNAVAILABLE = {
  title: "Attacks can't be loaded",
  description: "The platform API didn't return the attack feed. This doesn't mean no attacks have landed.",
};

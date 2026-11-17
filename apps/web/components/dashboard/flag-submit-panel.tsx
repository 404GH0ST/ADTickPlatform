"use client";

import type { ReactElement } from "react";
import { CircleCheck, CircleMinus, CircleX, Flag, LoaderCircle } from "lucide-react";

import { ThemeSlot } from "@/components/theme/theme-slot";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatusBanner } from "@/components/ui/status-banner";
import {
  type FlagSubmitState,
  type FlagVerdictOutcome,
  useFlagSubmit,
} from "@/components/dashboard/use-flag-submit";
import { cn } from "@/lib/utils";

const verdictPresentation: Record<
  FlagVerdictOutcome,
  { label: string; tone: string; Icon: typeof CircleCheck }
> = {
  accepted: { label: "Accepted", tone: "tone-success", Icon: CircleCheck },
  duplicate: { label: "Duplicate", tone: "tone-warning", Icon: CircleMinus },
  rejected: { label: "Rejected", tone: "tone-danger", Icon: CircleX },
};

/**
 * Global attack-flag entry: sits under the participant nav so submit is always
 * reachable (not buried under service controls). Logic lives in useFlagSubmit;
 * themes can replace the view through the FlagSubmit slot.
 */
export function FlagSubmitPanel({
  acceptingSubmissions,
}: {
  acceptingSubmissions?: boolean;
}): ReactElement {
  const state = useFlagSubmit({ acceptingSubmissions });
  return <ThemeSlot name="FlagSubmit" fallback={DefaultFlagSubmitView} props={state} />;
}

export function DefaultFlagSubmitView({
  rawFlags,
  setRawFlags,
  pending,
  closed,
  error,
  note,
  verdicts,
  hasFeedback,
  hint,
  onSubmit,
  onKeyDown,
  inputRef,
}: FlagSubmitState): ReactElement {
  const feedbackId = "flag-submit-feedback";
  const hintId = "flag-submit-hint";

  return (
    <section
      data-testid="flag-submit-panel"
      className="surface-workroom rounded-sm border border-border bg-card p-3"
      aria-label="Submit flags"
    >
      <form
        className="grid gap-3"
        onSubmit={onSubmit}
        aria-busy={pending || undefined}
      >
        <div className="grid gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <Flag className="h-4 w-4 shrink-0 text-primary" aria-hidden />
            <h2 className="text-sm font-semibold">Submit flags</h2>
            {closed ? (
              <Badge className="tone-warning" variant="outline">
                Submissions closed
              </Badge>
            ) : null}
          </div>

          <div
            className={cn(
              "overflow-hidden rounded-sm border border-input bg-background transition-[border-color,box-shadow]",
              "focus-within:border-ring focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background",
              error &&
                !pending &&
                "border-destructive focus-within:border-destructive focus-within:ring-destructive/40",
            )}
          >
            <label className="sr-only" htmlFor="flag-batch">
              Flags
            </label>
            <textarea
              ref={inputRef}
              id="flag-batch"
              className="min-h-11 w-full resize-y border-0 bg-transparent px-3 py-2 font-mono text-sm text-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-60"
              rows={2}
              placeholder="PLAYIT{...}"
              value={rawFlags}
              onChange={(event) => setRawFlags(event.target.value)}
              onKeyDown={onKeyDown}
              disabled={pending || closed}
              spellCheck={false}
              autoComplete="off"
              aria-invalid={error ? true : undefined}
              aria-describedby={
                hasFeedback ? `${hintId} ${feedbackId}` : hintId
              }
            />
            <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border bg-muted/20 px-2 py-1.5">
              <p
                id={hintId}
                className="min-w-0 text-xs leading-5 text-muted-foreground"
              >
                {hint}
              </p>
              <Button
                type="submit"
                size="sm"
                className="min-h-8 min-w-[5.5rem] shrink-0"
                disabled={pending || closed}
              >
                {pending ? (
                  <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden />
                ) : null}
                <span>{pending ? "Submitting…" : "Submit"}</span>
              </Button>
            </div>
          </div>
        </div>

        {hasFeedback ? (
          <div id={feedbackId} className="grid gap-2" aria-live="polite">
            {error ? <StatusBanner message={error} variant="error" /> : null}
            {note ? (
              <div data-testid="flag-submit-summary" data-variant={note.variant}>
                <StatusBanner message={note.message} variant={note.variant} />
              </div>
            ) : null}
            {verdicts.length > 0 ? (
              <ul
                className="grid max-h-40 gap-1 overflow-y-auto text-xs"
                aria-label="Flag verdicts"
              >
                {verdicts.map((verdict, index) => {
                  const outcome = verdict.outcome;
                  const { label, tone, Icon } = verdictPresentation[outcome];
                  const detail = verdict.detail || verdict.message;
                  return (
                    <li
                      key={`${verdict.flag}-${index}`}
                      data-testid="flag-verdict"
                      data-verdict={outcome}
                      className={cn(
                        "flex items-start gap-2 rounded-sm border px-2 py-1 leading-5",
                        tone,
                      )}
                    >
                      <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
                      <span className="min-w-0 break-all">
                        <span className="font-semibold">{label}</span>
                        {": "}
                        <span className="font-mono">{verdict.flag}</span>
                        {detail ? ` — ${detail}` : ""}
                      </span>
                    </li>
                  );
                })}
              </ul>
            ) : null}
          </div>
        ) : null}
      </form>
    </section>
  );
}

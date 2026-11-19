"use client";

import { Check, Flag, LoaderCircle } from "lucide-react";
import { useId } from "react";

import { Button, cn, ScrollableRegion, StatusBanner, type FlagSubmitState } from "@/components/theme/kit";

const VERDICT_FILL: Record<string, string> = {
  accepted: "nb-fill-aqua",
  duplicate: "nb-fill-yellow",
};

/** Flag form as a bold drop zone. Same submit logic and verdicts as the stock form. */
export function NbFlagSubmit({
  rawFlags,
  setRawFlags,
  pending,
  closed,
  error,
  note,
  verdicts,
  hint,
  onSubmit,
  onKeyDown,
  inputRef,
}: FlagSubmitState) {
  const hintId = useId();
  const errorId = useId();
  return (
    <section data-testid="flag-submit-panel" aria-label="Submit flags" className="nb-panel overflow-hidden">
      <form onSubmit={onSubmit} aria-busy={pending || undefined} className="grid grid-cols-1 gap-0 md:grid-cols-[13rem_minmax(0,1fr)]">
        <div className="flex flex-wrap items-center gap-3 border-b-[3px] border-border bg-[var(--nb-zebra)] px-4 py-4 sm:px-5 md:flex-col md:flex-nowrap md:items-start md:justify-between md:border-b-0 md:border-r-[3px]">
          <span className="nb-icon-key grid size-11 shrink-0 place-items-center rounded-[var(--nb-radius-sm)]">
            <Flag className="size-5" strokeWidth={2.4} aria-hidden />
          </span>
          <div className="grid min-w-0 grow basis-28 grid-cols-1 gap-0.5 wrap-anywhere md:grow-0 md:basis-auto">
            <h2 className="text-2xl font-extrabold leading-none">Submit flags</h2>
            {closed ? <span className="text-sm font-bold">Submissions closed</span> : null}
          </div>
        </div>
        <div className="grid grid-cols-1 gap-3 p-4 sm:p-5">
          <label htmlFor="flag-batch" className="sr-only">
            Flags
          </label>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-stretch">
            <textarea
              ref={inputRef}
              id="flag-batch"
              rows={2}
              placeholder="PLAYIT{...}"
              value={rawFlags}
              onChange={(event) => setRawFlags(event.target.value)}
              onKeyDown={onKeyDown}
              readOnly={pending}
              disabled={closed}
              spellCheck={false}
              autoComplete="off"
              aria-invalid={error ? true : undefined}
              aria-describedby={error ? `${hintId} ${errorId}` : hintId}
              className="min-h-14 min-w-0 flex-1 resize-y px-3 py-2.5 font-mono text-sm disabled:opacity-60"
            />
            <Button type="submit" size="lg" disabled={pending || closed} className="h-auto min-h-12 px-3 text-base sm:px-6">
              {pending ? <LoaderCircle className="size-4 shrink-0 motion-safe:animate-spin" aria-hidden /> : null}
              {pending ? "Submitting…" : "Submit"}
            </Button>
          </div>
          <p id={hintId} className="text-xs font-medium text-muted-foreground">{hint}</p>
          <div aria-live="polite" className="grid gap-2">
            {error ? <div id={errorId}><StatusBanner message={error} variant="error" /></div> : null}
            {note ? (
              <div data-testid="flag-submit-summary" data-variant={note.variant}>
                <StatusBanner message={note.message} variant={note.variant} />
              </div>
            ) : null}
            {verdicts.length > 0 ? (
              <ScrollableRegion axis="vertical" label="Flag results" className="max-h-44 overflow-y-auto">
                <ul aria-label="Flag verdicts" className="grid grid-cols-1 gap-2">
                  {verdicts.map((verdict, index) => (
                    <li
                      key={`${verdict.flag}-${index}`}
                      data-testid="flag-verdict"
                      data-verdict={verdict.outcome}
                      className="grid min-w-0 grid-cols-1 gap-1 rounded-[var(--nb-radius-sm)] border-2 border-border bg-muted p-2 text-xs wrap-anywhere"
                    >
                      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                        <span className={cn("nb-sticker max-w-full uppercase", VERDICT_FILL[verdict.outcome] ?? "nb-fill-pink")}>
                          {verdict.outcome === "accepted" ? (
                            <Check className="size-3.5 shrink-0" strokeWidth={3} aria-hidden />
                          ) : null}
                          {verdict.outcome}
                        </span>
                        {verdict.detail || verdict.message ? <span>{verdict.detail || verdict.message}</span> : null}
                      </div>
                      <span className="font-mono font-medium">{verdict.flag}</span>
                    </li>
                  ))}
                </ul>
              </ScrollableRegion>
            ) : null}
          </div>
        </div>
      </form>
    </section>
  );
}

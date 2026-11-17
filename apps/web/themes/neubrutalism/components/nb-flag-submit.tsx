"use client";

import { Flag, LoaderCircle } from "lucide-react";

import { Button, cn, StatusBanner, type FlagSubmitState } from "@/components/theme/kit";

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
  return (
    <section data-testid="flag-submit-panel" aria-label="Submit flags" className="nb-panel overflow-hidden">
      <form onSubmit={onSubmit} aria-busy={pending || undefined} className="grid gap-0 md:grid-cols-[13rem_minmax(0,1fr)]">
        <div className="nb-fill-yellow flex items-center gap-3 border-b-[3px] border-[var(--nb-ink)] px-5 py-4 md:flex-col md:items-start md:justify-between md:border-b-0 md:border-r-[3px]">
          <span className="grid size-11 place-items-center rounded-[var(--nb-radius)] nb-tile border-2 border-current">
            <Flag className="size-5" strokeWidth={2.4} aria-hidden />
          </span>
          <div className="grid gap-0.5">
            <h2 className="text-2xl font-extrabold leading-none">Submit flags</h2>
            {closed ? <span className="text-sm font-bold">Submissions closed</span> : null}
          </div>
        </div>
        <div className="grid gap-3 p-4 sm:p-5">
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
              disabled={pending || closed}
              spellCheck={false}
              autoComplete="off"
              aria-invalid={error ? true : undefined}
              className="min-h-14 flex-1 resize-y px-3 py-2.5 font-mono text-sm disabled:opacity-60"
            />
            <Button type="submit" size="lg" disabled={pending || closed} className="h-auto min-h-12 px-6 text-base">
              {pending ? <LoaderCircle className="size-4 animate-spin" aria-hidden /> : null}
              {pending ? "Submitting…" : "Submit"}
            </Button>
          </div>
          <p className="text-xs font-medium text-muted-foreground">{hint}</p>
          <div aria-live="polite" className="grid gap-2">
            {error ? <StatusBanner message={error} variant="error" /> : null}
            {note ? (
              <div data-testid="flag-submit-summary" data-variant={note.variant}>
                <StatusBanner message={note.message} variant={note.variant} />
              </div>
            ) : null}
            {verdicts.length > 0 ? (
              <ul aria-label="Flag verdicts" className="flex max-h-44 flex-wrap gap-2 overflow-y-auto">
                {verdicts.map((verdict, index) => (
                  <li
                    key={`${verdict.flag}-${index}`}
                    data-testid="flag-verdict"
                    data-verdict={verdict.outcome}
                    className={cn("nb-sticker max-w-full", VERDICT_FILL[verdict.outcome] ?? "nb-fill-pink")}
                  >
                    <span className="uppercase">{verdict.outcome}</span>
                    <span className="truncate font-mono font-medium">{verdict.flag}</span>
                    {verdict.detail || verdict.message ? (
                      <span className="truncate font-medium opacity-80">{verdict.detail || verdict.message}</span>
                    ) : null}
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        </div>
      </form>
    </section>
  );
}

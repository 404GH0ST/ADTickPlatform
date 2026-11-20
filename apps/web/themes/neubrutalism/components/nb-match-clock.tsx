"use client";

import type { ReactNode } from "react";

import { cn, useMatchClock, type PlatformOverview } from "@/components/theme/kit";

// Idle (not started, finished) is no status at all, so it takes the neutral
// sticker: lilac means held or "you" everywhere else in this theme.
const STATE_FILL = {
  running: "nb-fill-aqua",
  paused: "nb-fill-orange",
  idle: "nb-stat",
} as const;

const CELL_PADDING = "px-5 py-4 @max-[10.5rem]:px-2 @max-[9rem]:px-1";

function Cell({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={cn("grid min-w-0 content-start gap-1.5", CELL_PADDING)}>
      <dt className="nb-eyebrow text-[0.6875rem] text-muted-foreground">{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

/** Match progress as one bold strip: the tick number leads, then state, next tick and countdown. */
export function NbMatchClock({ overview }: { overview: PlatformOverview }) {
  const clock = useMatchClock(overview);
  // Running, but the scheduler has not published the next tick yet: say so
  // instead of showing two bare dashes.
  const waiting = clock.stateKind === "running" && !clock.counting;
  return (
    <section
      data-testid="match-progress"
      aria-label="Match progress"
      className="nb-panel @container grid grid-cols-1 overflow-hidden wrap-anywhere md:grid-cols-[minmax(10rem,auto)_minmax(0,1fr)]"
    >
      <div className={cn("grid content-center gap-1 border-b-[3px] border-border md:border-b-0 md:border-r-[3px]", CELL_PADDING)}>
        <p className="nb-eyebrow text-[0.6875rem] text-muted-foreground">Current tick</p>
        <p className="nb-display text-5xl font-extrabold leading-none tabular-nums">{clock.currentTickLabel}</p>
      </div>
      <dl className="grid grid-cols-1 divide-y-2 divide-border sm:grid-cols-3 sm:divide-x-2 sm:divide-y-0">
        <Cell label="Match state">
          <span className={cn("nb-sticker inline-flex max-w-full items-center gap-2 whitespace-normal", STATE_FILL[clock.stateKind])}>
            <span aria-hidden className="size-2 shrink-0 rounded-full bg-current" />
            {clock.stateLabel}
          </span>
        </Cell>
        <Cell label="Next tick at">
          {waiting ? (
            <span className="nb-display text-xl font-extrabold">Scheduling…</span>
          ) : (
            <span className="nb-display text-2xl font-extrabold tabular-nums">{clock.nextTickLabel}</span>
          )}
        </Cell>
        <Cell label="Countdown">
          {clock.counting ? (
            <span className="nb-countdown">{clock.countdown.primary}</span>
          ) : (
            <span className={cn("nb-display text-xl font-extrabold", !waiting && "text-muted-foreground")}>
              {waiting ? "Waiting" : clock.countdown.primary}
            </span>
          )}
          {waiting || clock.countdown.secondary ? (
            <span className="mt-1 block text-xs font-semibold text-muted-foreground">
              {waiting ? "Next tick time not published yet" : clock.countdown.secondary}
            </span>
          ) : null}
        </Cell>
      </dl>
    </section>
  );
}

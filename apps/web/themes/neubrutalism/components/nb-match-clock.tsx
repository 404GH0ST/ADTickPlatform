"use client";

import type { ReactNode } from "react";

import { cn, useMatchClock, type PlatformOverview } from "@/components/theme/kit";

const STATE_FILL = {
  running: "nb-fill-aqua",
  paused: "nb-fill-orange",
  idle: "nb-fill-lilac",
} as const;

function Cell({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid content-start gap-1.5 px-5 py-4">
      <dt className="nb-eyebrow text-[0.6875rem] text-muted-foreground">{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

/** Match progress as one bold strip: the tick number leads, then state, next tick and countdown. */
export function NbMatchClock({ overview }: { overview: PlatformOverview }) {
  const clock = useMatchClock(overview);
  return (
    <section
      data-testid="match-progress"
      aria-label="Match progress"
      className="nb-panel grid overflow-hidden sm:grid-cols-[minmax(10rem,auto)_minmax(0,1fr)]"
    >
      <div className="nb-fill-yellow grid content-center gap-1 border-b-[3px] border-[var(--nb-ink)] px-5 py-4 sm:border-b-0 sm:border-r-[3px]">
        <p className="nb-eyebrow text-[0.6875rem]">Current tick</p>
        <p className="nb-display text-5xl font-extrabold leading-none tabular-nums">{clock.currentTickLabel}</p>
      </div>
      <dl className="grid grid-cols-1 divide-y-2 divide-border sm:grid-cols-3 sm:divide-x-2 sm:divide-y-0">
        <Cell label="Match state">
          <span className={cn("nb-sticker inline-flex items-center gap-2", STATE_FILL[clock.stateKind])}>
            <span
              aria-hidden
              className={cn(
                "inline-block size-2 rounded-full bg-[var(--nb-ink)]",
                clock.stateKind === "running" && "animate-pulse",
              )}
            />
            {clock.stateLabel}
          </span>
        </Cell>
        <Cell label="Next tick at">
          <span className="nb-display text-2xl font-extrabold tabular-nums">{clock.nextTickLabel}</span>
        </Cell>
        <Cell label="Countdown">
          {clock.counting ? (
            <span className="nb-countdown">{clock.countdown.primary}</span>
          ) : (
            <span className="nb-display text-xl font-extrabold text-muted-foreground">{clock.countdown.primary}</span>
          )}
          {clock.countdown.secondary ? (
            <span className="mt-1 block text-xs font-semibold text-muted-foreground">{clock.countdown.secondary}</span>
          ) : null}
        </Cell>
      </dl>
    </section>
  );
}

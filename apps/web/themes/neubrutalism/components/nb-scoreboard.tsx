"use client";

import { Crown, Snowflake } from "lucide-react";

import { NbStandings } from "./nb-standings";

import {
  cn,
  DataUnavailable,
  SCORES_UNAVAILABLE,
  type ScoreRow,
  type ScoreboardViewProps,
} from "@/components/theme/kit";
import { formatIndonesianDate } from "@/lib/date-format";

function format(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

// DOM order is rank order (screen readers read 1, 2, 3); order-* lays out the
// classic 2-1-3 podium. Phones get three short columns so the standings start
// in the first screen. The breakpoint is in rem, so enlarged browser text
// (or a 320px screen) falls back to a stack instead of overflowing.
// In the shared grid row, top margins create the steps while every card can
// still grow to fit its content. Fills follow the medal ramp.
const PODIUM = [
  { place: 1, fill: "nb-fill-yellow", layout: "min-[22rem]:order-2 min-[22rem]:min-h-[8.5rem] sm:min-h-[14rem]" },
  { place: 2, fill: "nb-fill-silver", layout: "min-[22rem]:order-1 min-[22rem]:mt-4 min-[22rem]:min-h-[7.5rem] sm:mt-12 sm:min-h-[11rem]" },
  { place: 3, fill: "nb-fill-orange", layout: "min-[22rem]:order-3 min-[22rem]:mt-8 min-[22rem]:min-h-[6.5rem] sm:mt-[4.5rem] sm:min-h-[9.5rem]" },
] as const;

function PodiumBlock({ row, fill, layout, mine }: { row: ScoreRow; fill: string; layout: string; mine: boolean }) {
  const first = row.rank === 1;
  return (
    <li
      className={cn("nb-pop flex min-w-0 flex-col justify-between gap-2 rounded-[var(--nb-radius)] p-2.5 sm:gap-4 sm:p-5", fill, layout)}
      data-podium-current={mine ? "true" : undefined}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="nb-display text-3xl font-extrabold leading-none sm:text-5xl">#{row.rank}</span>
        {first ? <Crown className="size-5 sm:size-7" strokeWidth={2.4} aria-label="Leader" /> : null}
      </div>
      <div className="grid grid-cols-1 gap-1">
        <p className="nb-display wrap-anywhere text-sm font-bold leading-tight sm:text-xl">
          {row.team}
          {mine ? <span className="nb-sticker nb-status-chip align-middle max-sm:mt-1 max-sm:flex max-sm:w-fit sm:ml-2">You</span> : null}
        </p>
        {/* Phones leave institution and the A/D/SLA split to the standings below. */}
        {row.institution ? (
          <p data-testid="nb-podium-institution" className="nb-institution nb-institution-chip justify-self-start max-sm:hidden">
            {row.institution}
          </p>
        ) : null}
        <p className="nb-display wrap-anywhere text-xl font-extrabold tabular-nums leading-none sm:text-3xl">{format(row.total)}</p>
        <p className="text-xs font-semibold tabular-nums max-sm:hidden">
          A {format(row.attack)} · D {format(row.defense)} · SLA {format(row.sla)}
        </p>
      </div>
    </li>
  );
}

/** Podium for the top three, then every team as a row with a score bar. */
export function NbScoreboard({ scoreRows, currentTeamName, frozen, freezeAt, unavailable }: ScoreboardViewProps) {
  const podium = PODIUM.map((slot) => ({
    slot,
    row: scoreRows.find((row) => row.rank === slot.place && !row.disqualified),
  })).filter(
    (entry): entry is { slot: (typeof PODIUM)[number]; row: ScoreRow } => entry.row !== undefined,
  );

  return (
    <section data-testid="nb-scoreboard" className="grid gap-6" aria-label="Scoreboard">
      {frozen ? (
        <p
          role="status"
          data-testid="scoreboard-frozen-banner"
          className="nb-frozen flex items-center gap-3 rounded-[var(--nb-radius)] px-4 py-3 text-sm"
        >
          <Snowflake className="size-5 shrink-0" strokeWidth={2.4} aria-hidden />
          <span>
            <span className="font-bold">Scoreboard frozen.</span>
            {freezeAt ? ` Snapshot as of ${formatIndonesianDate(freezeAt)}.` : ""}
          </span>
        </p>
      ) : null}

      {unavailable ? (
        <DataUnavailable {...SCORES_UNAVAILABLE} />
      ) : scoreRows.length === 0 ? (
        <p className="nb-panel p-6 text-sm text-muted-foreground">
          No scores yet. The board fills in after the first scored tick.
        </p>
      ) : (
        <>
          {podium.length > 0 ? (
            <ol aria-label="Top teams" className="grid grid-cols-1 gap-3 min-[22rem]:grid-cols-3 sm:gap-5">
              {podium.map(({ slot, row }) => (
                <PodiumBlock
                  key={row.team}
                  row={row}
                  fill={slot.fill}
                  layout={slot.layout}
                  mine={row.team === currentTeamName}
                />
              ))}
            </ol>
          ) : null}

          <NbStandings rows={scoreRows} currentTeamName={currentTeamName} />
        </>
      )}
    </section>
  );
}

"use client";

import { Crown, Snowflake } from "lucide-react";

import { NbStandings } from "./nb-standings";

import { cn, type ScoreRow, type ScoreboardViewProps } from "@/components/theme/kit";

function format(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

// DOM order is rank order (mobile stack and screen readers read 1, 2, 3);
// sm:order-* lays out the classic 2-1-3 podium. Fills follow the medal ramp.
const PODIUM = [
  { place: 1, fill: "nb-fill-yellow", height: "sm:order-2 sm:min-h-[14rem]" },
  { place: 2, fill: "nb-fill-lilac", height: "sm:order-1 sm:min-h-[11rem]" },
  { place: 3, fill: "nb-fill-orange", height: "sm:order-3 sm:min-h-[9.5rem]" },
] as const;

function PodiumBlock({ row, fill, height, mine }: { row: ScoreRow; fill: string; height: string; mine: boolean }) {
  const first = row.rank === 1;
  return (
    <li
      className={cn("nb-pop flex min-w-0 flex-col justify-between gap-4 rounded-[var(--nb-radius)] p-4 sm:p-5", fill, height, first && "sm:-translate-y-1")}
      data-podium-current={mine ? "true" : undefined}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="nb-display text-5xl font-extrabold leading-none">#{row.rank}</span>
        {first ? <Crown className="size-7" strokeWidth={2.4} aria-label="Leader" /> : null}
      </div>
      <div className="grid grid-cols-1 gap-1">
        <p className="nb-display wrap-anywhere text-xl font-bold">
          {row.team}
          {mine ? <span className="nb-sticker nb-status-chip ml-2 align-middle">You</span> : null}
        </p>
        <p className="nb-display wrap-anywhere text-3xl font-extrabold tabular-nums leading-none">{format(row.total)}</p>
        <p className="text-xs font-semibold tabular-nums">
          A {format(row.attack)} · D {format(row.defense)} · SLA {format(row.sla)}
        </p>
      </div>
    </li>
  );
}

/** Podium for the top three, then every team as a row with a score bar. */
export function NbScoreboard({ scoreRows, currentTeamName, frozen, freezeAt }: ScoreboardViewProps) {
  const podium = PODIUM.map((slot) => ({ slot, row: scoreRows.find((row) => row.rank === slot.place) })).filter(
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
            {freezeAt ? ` Snapshot as of ${new Date(freezeAt).toLocaleString()}.` : ""}
          </span>
        </p>
      ) : null}

      {scoreRows.length === 0 ? (
        <p className="nb-panel p-6 text-sm text-muted-foreground">
          No scores yet. The board fills in after the first scored tick.
        </p>
      ) : (
        <>
          {podium.length > 0 ? (
            <ol aria-label="Top teams" className="grid items-end gap-5 sm:grid-cols-3">
              {podium.map(({ slot, row }) => (
                <PodiumBlock
                  key={row.team}
                  row={row}
                  fill={slot.fill}
                  height={slot.height}
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

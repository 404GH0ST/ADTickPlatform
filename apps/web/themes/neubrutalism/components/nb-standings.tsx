"use client";

import type { ReactNode } from "react";
import { ArrowDown, ArrowUp } from "lucide-react";

import { cn, type ScoreRow } from "@/components/theme/kit";

function format(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

function RankDelta({ delta }: { delta: string }) {
  if (delta === "new") {
    return <span className="nb-sticker nb-fill-blue py-0 text-[0.625rem]">new</span>;
  }
  const value = Number.parseInt(delta, 10);
  if (Number.isNaN(value) || value === 0) {
    return <span className="text-xs font-bold text-muted-foreground" aria-label="No rank change">–</span>;
  }
  const up = value > 0;
  return (
    <span
      className={cn("inline-flex items-center text-xs font-extrabold tabular-nums", up ? "text-positive" : "text-negative")}
      aria-label={`${up ? "Up" : "Down"} ${Math.abs(value)}`}
    >
      {up ? <ArrowUp className="size-3.5" strokeWidth={3} aria-hidden /> : <ArrowDown className="size-3.5" strokeWidth={3} aria-hidden />}
      {Math.abs(value)}
    </span>
  );
}

/**
 * Standings list shared by the participant and organizer boards: one row per
 * team with totals, then a card per service (signed total plus A/D/SLA).
 */
export function NbStandings({
  rows,
  currentTeamName,
  showDelta = false,
  title = "Standings",
  meta,
  toolbar,
  emptyMessage = "No scores yet.",
  testId,
}: {
  rows: ScoreRow[];
  currentTeamName?: string;
  showDelta?: boolean;
  title?: string;
  meta?: ReactNode;
  toolbar?: ReactNode;
  emptyMessage?: string;
  testId?: string;
}) {
  return (
    <div className="nb-panel overflow-hidden" data-testid={testId}>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b-[3px] border-border px-5 py-3">
        <div className="flex items-baseline gap-3">
          <h2 className="text-lg font-extrabold">{title}</h2>
          {meta ?? <span className="text-xs font-semibold text-muted-foreground">{rows.length} teams</span>}
        </div>
        {toolbar ? <div className="flex flex-wrap items-center gap-2">{toolbar}</div> : null}
      </div>
      {rows.length === 0 ? (
        <p className="px-5 py-8 text-sm text-muted-foreground">{emptyMessage}</p>
      ) : (
        <>
          <div
            aria-hidden
            className="hidden grid-cols-[3.5rem_minmax(0,1fr)_repeat(3,4.5rem)_5.5rem] gap-x-4 px-5 pt-3 nb-eyebrow text-[0.6875rem] text-muted-foreground md:grid"
          >
            <span>Rank</span>
            <span>Team</span>
            <span className="text-right">Atk</span>
            <span className="text-right">Def</span>
            <span className="text-right">SLA</span>
            <span className="text-right">Total</span>
          </div>
          <ol className="grid gap-1 p-2 sm:p-3">
            {rows.map((row) => {
              const mine = currentTeamName !== undefined && row.team === currentTeamName;
              return (
                <li
                  key={row.team}
                  data-testid="nb-score-row"
                  data-current-team={mine ? "true" : undefined}
                  className={cn(
                    "grid items-center gap-x-4 gap-y-1 rounded-[var(--nb-radius-sm)] border-2 border-transparent px-3 py-2.5 text-sm",
                    "grid-cols-[3.5rem_minmax(0,1fr)_auto] md:grid-cols-[3.5rem_minmax(0,1fr)_repeat(3,4.5rem)_5.5rem]",
                    mine && "nb-fill-lilac border-[var(--nb-ink)]",
                  )}
                >
                  <span className="flex items-center gap-1.5">
                    <span className="nb-display text-lg font-extrabold tabular-nums">#{row.rank}</span>
                    {showDelta ? <RankDelta delta={row.delta} /> : null}
                  </span>
                  <span className="truncate font-semibold">
                    {row.team}
                    {mine ? <span className="ml-2 text-xs font-bold">(you)</span> : null}
                  </span>
                  <span className="hidden text-right tabular-nums md:block" title="Attack">{format(row.attack)}</span>
                  <span className="hidden text-right tabular-nums md:block" title="Defense">{format(row.defense)}</span>
                  <span className="hidden text-right tabular-nums md:block" title="SLA">{format(row.sla)}</span>
                  <span className="nb-display row-start-1 text-right text-lg font-extrabold tabular-nums md:row-start-auto">
                    {format(row.total)}
                  </span>
                  <p className="col-span-full flex flex-wrap gap-x-4 gap-y-0.5 text-xs font-semibold tabular-nums md:hidden">
                    <span>Atk {format(row.attack)}</span>
                    <span>Def {format(row.defense)}</span>
                    <span>SLA {format(row.sla)}</span>
                  </p>
                  {row.services && row.services.length > 0 ? (
                    <ul
                      aria-label={`${row.team} per-service scores`}
                      className="col-span-full mt-1.5 grid gap-2 sm:grid-cols-2 md:ml-[4.5rem] lg:grid-cols-3 2xl:grid-cols-4"
                    >
                      {row.services.map((service) => (
                        <li
                          key={service.challenge_id}
                          data-testid="nb-service-score"
                          className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 rounded-[var(--nb-radius-sm)] border-2 border-border bg-card px-3 py-2 text-xs text-card-foreground"
                        >
                          <span className="truncate text-sm font-bold">{service.service}</span>
                          <span
                            className={cn("nb-display text-base font-extrabold tabular-nums", service.total < 0 && "text-negative")}
                          >
                            {service.total > 0 ? "+" : ""}
                            {format(service.total)}
                          </span>
                          <span className="col-span-2 flex gap-1.5 tabular-nums">
                            <span className="nb-sticker nb-fill-pink py-0 text-[0.6875rem]" title="Attack">
                              A {format(service.attack)}
                            </span>
                            <span className="nb-sticker nb-fill-blue py-0 text-[0.6875rem]" title="Defense">
                              D {format(service.defense)}
                            </span>
                            <span className="nb-sticker nb-fill-aqua py-0 text-[0.6875rem]" title="SLA">
                              SLA {format(service.sla)}
                            </span>
                          </span>
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </li>
              );
            })}
          </ol>
        </>
      )}
    </div>
  );
}

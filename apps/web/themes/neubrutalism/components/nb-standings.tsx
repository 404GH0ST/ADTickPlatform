"use client";

import { useState, type ReactNode } from "react";
import { ArrowDown, ArrowUp, ListCollapse, ListTree } from "lucide-react";

import { cn, type ScoreRow } from "@/components/theme/kit";

function format(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

function RankDelta({ delta }: { delta: string }) {
  if (delta === "new") {
    return <span className="nb-sticker nb-stat py-0 text-[0.625rem]">new</span>;
  }
  const value = Number.parseInt(delta, 10);
  if (Number.isNaN(value) || value === 0) {
    return (
      <span className="text-xs font-bold text-muted-foreground">
        <span aria-hidden>–</span><span className="sr-only">No rank change</span>
      </span>
    );
  }
  const up = value > 0;
  return (
    <span
      className={cn("inline-flex items-center text-xs font-extrabold tabular-nums", up ? "text-positive" : "text-negative")}
    >
      <span className="sr-only">{up ? "Up" : "Down"} </span>
      {up ? <ArrowUp className="size-3.5" strokeWidth={3} aria-hidden /> : <ArrowDown className="size-3.5" strokeWidth={3} aria-hidden />}
      {Math.abs(value)}
    </span>
  );
}

/** Above this many teams the per-service breakdown starts collapsed. */
const DETAIL_OPEN_MAX_TEAMS = 8;

/**
 * Standings list shared by the participant and organizer boards: one row per
 * team with totals, then (toggleable) a card per service with its signed
 * total plus A/D/SLA.
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
  const hasDetail = rows.some((row) => row.services && row.services.length > 0);
  const [showDetail, setShowDetail] = useState(rows.length <= DETAIL_OPEN_MAX_TEAMS);
  return (
    <div className="nb-panel @container overflow-hidden" data-testid={testId}>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b-[3px] border-border px-5 py-3">
        <div className="flex min-w-0 max-w-full flex-wrap items-baseline gap-x-3 gap-y-1">
          <h2 className="wrap-anywhere text-lg font-extrabold">{title}</h2>
          {meta ?? <span className="text-xs font-semibold text-muted-foreground">{rows.length} teams</span>}
        </div>
        {toolbar || hasDetail ? (
          <div className="flex min-w-0 max-w-full flex-wrap items-center gap-2">
            {hasDetail ? (
              <button
                type="button"
                aria-pressed={showDetail}
                onClick={() => setShowDetail((open) => !open)}
                className="nb-sticker nb-stat min-h-9 max-w-full cursor-pointer gap-1.5 px-3 py-1 text-xs wrap-anywhere whitespace-normal"
              >
                {showDetail ? (
                  <ListCollapse className="size-4 shrink-0" strokeWidth={2.4} aria-hidden />
                ) : (
                  <ListTree className="size-4 shrink-0" strokeWidth={2.4} aria-hidden />
                )}
                Per-service scores
              </button>
            ) : null}
            {toolbar}
          </div>
        ) : null}
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
          <ol className="grid grid-cols-1 gap-1 p-2 sm:p-3">
            {rows.map((row) => {
              const mine = currentTeamName !== undefined && row.team === currentTeamName;
              return (
                <li
                  key={row.team}
                  data-testid="nb-score-row"
                  data-current-team={mine ? "true" : undefined}
                  className={cn(
                    "nb-score-row grid items-center gap-x-4 gap-y-1 rounded-[var(--nb-radius-sm)] border-2 border-transparent px-3 py-2.5 text-sm",
                    "grid-cols-[3.5rem_minmax(0,1fr)_auto] md:grid-cols-[3.5rem_minmax(0,1fr)_repeat(3,4.5rem)_5.5rem]",
                    mine && "nb-fill-lilac border-border",
                  )}
                >
                  <span className="flex items-center gap-1.5">
                    <span className="nb-display text-lg font-extrabold tabular-nums"><span className="sr-only">Rank </span>#{row.rank}</span>
                    {showDelta ? <RankDelta delta={row.delta} /> : null}
                  </span>
                  <span className="min-w-0 wrap-anywhere font-semibold">
                    {row.team}
                    {mine ? <span className="ml-2 text-xs font-bold">(you)</span> : null}
                  </span>
                  <span className="hidden text-right tabular-nums md:block" title="Attack"><span className="sr-only">Attack </span>{format(row.attack)}</span>
                  <span className="hidden text-right tabular-nums md:block" title="Defense"><span className="sr-only">Defense </span>{format(row.defense)}</span>
                  <span className="hidden text-right tabular-nums md:block" title="SLA"><span className="sr-only">SLA </span>{format(row.sla)}</span>
                  <span className="nb-score-total nb-display wrap-anywhere text-right text-lg font-extrabold tabular-nums">
                    <span className="sr-only @max-[16rem]:not-sr-only @max-[16rem]:mr-1 @max-[16rem]:text-xs">Total</span>
                    {format(row.total)}
                  </span>
                  <p className="col-span-full flex flex-wrap gap-x-4 gap-y-0.5 text-xs font-semibold tabular-nums md:hidden">
                    <span>Atk {format(row.attack)}</span>
                    <span>Def {format(row.defense)}</span>
                    <span>SLA {format(row.sla)}</span>
                  </p>
                  {showDetail && row.services && row.services.length > 0 ? (
                    <ul
                      aria-label={`${row.team} per-service scores`}
                      className="col-span-full mt-1.5 grid grid-cols-[repeat(auto-fill,minmax(min(100%,12rem),1fr))] gap-2 md:ml-[4.5rem]"
                    >
                      {row.services.map((service) => (
                        <li
                          key={service.challenge_id}
                          data-testid="nb-service-score"
                          className="@container grid grid-cols-1 gap-1 rounded-[var(--nb-radius-sm)] border-2 border-border bg-card px-1.5 py-2 text-xs text-card-foreground @[12rem]:px-3"
                        >
                          <span className="flex flex-wrap items-baseline gap-x-3 gap-y-1 wrap-anywhere">
                            <span className="text-sm font-bold">{service.service}</span>
                            <span
                              className={cn("nb-display ms-auto text-end text-sm font-extrabold tabular-nums @[9rem]:text-base", service.total < 0 && "text-negative")}
                            >
                              {service.total > 0 ? "+" : ""}
                              {format(service.total)}
                            </span>
                          </span>
                          <span className="grid min-w-0 grid-cols-1 gap-1.5 tabular-nums @[9rem]:grid-cols-2 @[16rem]:grid-cols-[repeat(2,minmax(0,1fr))_minmax(0,1.5fr)]">
                            <span className="nb-sticker nb-stat nb-stat-a min-w-0 justify-center px-1 py-0 text-center text-[0.6875rem] wrap-anywhere whitespace-normal" title="Attack">
                              A {format(service.attack)}
                            </span>
                            <span className="nb-sticker nb-stat nb-stat-d min-w-0 justify-center px-1 py-0 text-center text-[0.6875rem] wrap-anywhere whitespace-normal" title="Defense">
                              D {format(service.defense)}
                            </span>
                            <span className="nb-sticker nb-stat nb-stat-sla col-span-full min-w-0 justify-center px-1 py-0 text-center text-[0.6875rem] wrap-anywhere whitespace-normal @[16rem]:col-span-1" title="SLA">
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

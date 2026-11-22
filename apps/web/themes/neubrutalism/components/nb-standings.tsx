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
      className={cn("inline-flex min-w-0 items-center text-xs font-extrabold tabular-nums", up ? "text-positive" : "text-negative")}
    >
      <span className="sr-only">{up ? "Up" : "Down"} </span>
      {up ? <ArrowUp className="size-3.5 shrink-0" strokeWidth={3} aria-hidden /> : <ArrowDown className="size-3.5 shrink-0" strokeWidth={3} aria-hidden />}
      <span className="min-w-0 wrap-anywhere">{Math.abs(value)}</span>
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
  teamCount = rows.length,
  currentTeamName,
  showDelta = false,
  title = "Standings",
  meta,
  toolbar,
  emptyMessage = "No scores yet.",
  testId,
}: {
  rows: ScoreRow[];
  teamCount?: number;
  currentTeamName?: string;
  showDelta?: boolean;
  title?: string;
  meta?: ReactNode;
  toolbar?: ReactNode;
  emptyMessage?: string;
  testId?: string;
}) {
  const hasDetail = rows.some((row) => row.services && row.services.length > 0);
  const [detailPreference, setDetailPreference] = useState<boolean | null>(null);
  const showDetail = detailPreference ?? (teamCount <= DETAIL_OPEN_MAX_TEAMS);
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
                onClick={() => setDetailPreference(!showDetail)}
                className={cn(
                  "nb-sticker nb-stat min-h-9 max-w-full cursor-pointer gap-1.5 px-3 py-1 text-xs wrap-anywhere whitespace-normal",
                  showDetail && "nb-selected",
                )}
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
        <div className="grid grid-cols-1 p-2 sm:p-3 @max-[16rem]:px-1 @[42rem]:grid-cols-[minmax(4.5rem,max-content)_minmax(0,1fr)_repeat(3,minmax(4.5rem,max-content))_minmax(5.5rem,max-content)] @[42rem]:gap-x-4">
          <div
            aria-hidden
            className="nb-eyebrow col-span-full hidden grid-cols-subgrid border-x-2 border-transparent px-3 pb-1 text-[0.6875rem] text-muted-foreground @[42rem]:grid"
          >
            <span>Rank</span>
            <span>Team</span>
            <span className="text-right">Atk</span>
            <span className="text-right">Def</span>
            <span className="text-right">SLA</span>
            <span className="text-right">Total</span>
          </div>
          <ol className="col-span-full grid grid-cols-subgrid gap-y-1">
            {rows.map((row) => {
              const mine = currentTeamName !== undefined && row.team === currentTeamName;
              return (
                <li
                  key={row.team}
                  data-testid="nb-score-row"
                  data-current-team={mine ? "true" : undefined}
                  className={cn(
                    "col-span-full grid items-center gap-x-4 gap-y-1 rounded-[var(--nb-radius-sm)] border-2 border-transparent px-3 py-2.5 text-sm",
                    "grid-cols-[4rem_minmax(0,1fr)] @max-[16rem]:grid-cols-1 @max-[16rem]:px-1 @[22rem]:grid-cols-[4rem_minmax(0,1fr)_fit-content(45%)] @[42rem]:grid-cols-subgrid",
                    mine && "nb-fill-lilac border-border",
                  )}
                >
                  <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0">
                    {row.disqualified ? (
                      <span data-testid="nb-score-dq" title="Disqualified" className="nb-sticker nb-stat py-0 text-xs text-negative">
                        <span aria-hidden>DQ</span>
                        <span className="sr-only">Disqualified</span>
                      </span>
                    ) : (
                      <span className="nb-display min-w-0 wrap-anywhere text-lg font-extrabold tabular-nums"><span className="sr-only">Rank </span>#{row.rank}</span>
                    )}
                    {showDelta && !row.disqualified ? <RankDelta delta={row.delta} /> : null}
                  </span>
                  <span className="min-w-0 wrap-anywhere font-semibold">
                    {row.team}
                    {mine ? <span className="ml-2 text-xs font-bold">(you)</span> : null}
                    {row.institution ? (
                      <span data-testid="nb-score-institution" className="nb-institution mt-0.5 block">
                        {row.institution}
                      </span>
                    ) : null}
                  </span>
                  <span className="hidden min-w-0 wrap-anywhere text-right tabular-nums @[42rem]:block" title="Attack"><span className="sr-only">Attack </span>{format(row.attack)}</span>
                  <span className="hidden min-w-0 wrap-anywhere text-right tabular-nums @[42rem]:block" title="Defense"><span className="sr-only">Defense </span>{format(row.defense)}</span>
                  <span className="hidden min-w-0 wrap-anywhere text-right tabular-nums @[42rem]:block" title="SLA"><span className="sr-only">SLA </span>{format(row.sla)}</span>
                  <span className="col-span-full min-w-0 @[22rem]:col-span-1 @[22rem]:text-right">
                    <span className="block text-xs font-semibold @[42rem]:sr-only">Total </span>
                    <span className="nb-display block wrap-anywhere text-lg font-extrabold tabular-nums">{format(row.total)}</span>
                  </span>
                  <p className="col-span-full flex flex-wrap gap-x-4 gap-y-0.5 text-xs font-semibold tabular-nums wrap-anywhere @[42rem]:hidden">
                    <span>Atk {format(row.attack)}</span>
                    <span>Def {format(row.defense)}</span>
                    <span>SLA {format(row.sla)}</span>
                  </p>
                  {showDetail && row.services && row.services.length > 0 ? (
                    <ul
                      aria-label={`${row.team} per-service scores`}
                      className="col-span-full mt-1.5 grid grid-cols-[repeat(auto-fill,minmax(min(100%,12rem),1fr))] gap-2 @[42rem]:col-[2/-1]"
                    >
                      {row.services.map((service) => (
                        <li
                          key={service.challenge_id}
                          data-testid="nb-service-score"
                          className="@container grid grid-cols-1 gap-1 rounded-[var(--nb-radius-sm)] border-2 border-transparent bg-[var(--nb-zebra)] px-1.5 py-2 text-xs text-card-foreground @[12rem]:px-3"
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
                              <span className="min-w-0"><span aria-hidden>A </span><span className="sr-only">Attack </span>{format(service.attack)}</span>
                            </span>
                            <span className="nb-sticker nb-stat nb-stat-d min-w-0 justify-center px-1 py-0 text-center text-[0.6875rem] wrap-anywhere whitespace-normal" title="Defense">
                              <span className="min-w-0"><span aria-hidden>D </span><span className="sr-only">Defense </span>{format(service.defense)}</span>
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
          {hasDetail && showDetail ? (
            <p
              data-testid="nb-score-legend"
              className="col-span-full mt-2 flex flex-wrap gap-x-4 gap-y-1 border-t-2 border-dashed border-border px-3 pt-2.5 text-xs font-semibold text-muted-foreground"
            >
              <span><span className="font-extrabold text-foreground">A</span> Attack</span>
              <span><span className="font-extrabold text-foreground">D</span> Defense</span>
              <span><span className="font-extrabold text-foreground">SLA</span> Availability</span>
            </p>
          ) : null}
        </div>
      )}
    </div>
  );
}
